// Package validate implements deterministic, stricter-than-search validation of
// a Workgraph workspace. Validation must be stricter than the indexer so people
// cannot come to rely on forgiving cached behavior the files do not encode.
package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/moul/workgraph/internal/eventlog"
	"github.com/moul/workgraph/internal/frontmatter"
	"github.com/moul/workgraph/internal/id"
	"github.com/moul/workgraph/internal/index"
	"github.com/moul/workgraph/internal/model"
	"github.com/moul/workgraph/internal/store"
)

// Severity of a finding.
type Severity string

const (
	Error   Severity = "ERROR"
	Warning Severity = "WARN"
)

// Finding is one validation result.
type Finding struct {
	Severity Severity
	Object   string // id or path
	Message  string
}

func (f Finding) String() string {
	who := f.Object
	if who != "" {
		who = " " + who
	}
	return fmt.Sprintf("%-5s%s %s", f.Severity, who, f.Message)
}

// Report is the ordered set of findings plus a quick error count.
type Report struct {
	Findings []Finding
}

func (r *Report) add(sev Severity, obj, msg string) {
	r.Findings = append(r.Findings, Finding{sev, obj, msg})
}
func (r *Report) errf(obj, f string, a ...any)  { r.add(Error, obj, fmt.Sprintf(f, a...)) }
func (r *Report) warnf(obj, f string, a ...any) { r.add(Warning, obj, fmt.Sprintf(f, a...)) }

// Errors returns the number of Error-severity findings.
func (r *Report) Errors() int {
	n := 0
	for _, f := range r.Findings {
		if f.Severity == Error {
			n++
		}
	}
	return n
}

// record is a tolerant, partially-decoded object used for cross-checks even
// when some files are malformed.
type record struct {
	id     string
	typ    string
	kind   string
	status string
	path   string
	item   *model.Item
	dec    *model.Decision
	proj   *model.Project
}

// Run validates the workspace and returns a report. It never aborts on a single
// malformed file; each problem becomes a finding.
func Run(w *store.Workspace) (*Report, error) {
	r := &Report{}
	ont := w.Ontology

	recs := scan(w, r, ont)

	byID := map[string]*record{}
	for i := range recs {
		rec := &recs[i]
		if rec.id == "" {
			continue
		}
		if prev, ok := byID[rec.id]; ok {
			r.errf(rec.id, "duplicate id also defined at %s", prev.path)
			continue
		}
		byID[rec.id] = rec
	}

	checkReferences(r, recs, byID)
	checkCycles(r, recs, byID)
	checkLifecycle(r, recs, byID, time.Now())
	checkEvents(w, r, ont)
	checkStaleIndex(w, r)
	secretScan(w, r)

	sort.SliceStable(r.Findings, func(i, j int) bool {
		if r.Findings[i].Severity != r.Findings[j].Severity {
			return r.Findings[i].Severity == Error // errors first
		}
		return r.Findings[i].Object < r.Findings[j].Object
	})
	return r, nil
}

// scan walks candidate object files tolerantly.
func scan(w *store.Workspace, r *Report, ont *model.Ontology) []record {
	var recs []record
	walk := func(rel string, want string) {
		abs := filepath.Join(w.Root, rel)
		raw, err := os.ReadFile(abs)
		if err != nil {
			return
		}
		doc, err := frontmatter.Parse(raw)
		if err != nil {
			r.errf(rel, "malformed frontmatter: %v", err)
			return
		}
		rec := record{path: rel}
		if v, ok := doc.Meta["id"].(string); ok {
			rec.id = v
		}
		if v, ok := doc.Meta["type"].(string); ok {
			rec.typ = v
		}
		if v, ok := doc.Meta["status"].(string); ok {
			rec.status = v
		}
		// Required fields on every object.
		for _, f := range []string{"id", "type", "title", "status", "created_at", "updated_at"} {
			if _, ok := doc.Meta[f]; !ok {
				r.errf(rel, "missing required field %q", f)
			}
		}
		if rec.id != "" && !id.Valid(rec.id) {
			r.errf(rel, "invalid id syntax %q", rec.id)
		}
		if rec.typ != "" && !ont.Has("object_type", rec.typ) {
			r.errf(rec.id, "unknown object type %q (path %s)", rec.typ, rel)
		}
		// filename/id invariant for prefixed-ULID objects.
		if want != "project" && want != "worker" && rec.id != "" {
			if got := id.IDFromFilename(filepath.Base(rel)); got != rec.id {
				r.errf(rec.id, "filename %s does not start with id", filepath.Base(rel))
			}
		}
		// Typed decode + per-type checks.
		switch rec.typ {
		case model.TypeItem:
			var it model.Item
			if err := model.Decode(doc.Meta, &it); err != nil {
				r.errf(rec.id, "decode item: %v", err)
			} else {
				rec.item = &it
				rec.kind = it.Kind
				if it.Kind != "" && !ont.Has("item_kind", it.Kind) {
					r.errf(rec.id, "unknown item kind %q", it.Kind)
				}
				if !ont.Has("item_status", it.Status) {
					r.errf(rec.id, "unknown item status %q", it.Status)
				}
				checkDates(r, rec.id, it.CreatedAt, it.UpdatedAt, it.DueAt)
			}
		case model.TypeDecision:
			var d model.Decision
			if err := model.Decode(doc.Meta, &d); err == nil {
				rec.dec = &d
				if !ont.Has("decision_status", d.Status) {
					r.errf(rec.id, "unknown decision status %q", d.Status)
				}
			}
		case model.TypeProject:
			var p model.Project
			if err := model.Decode(doc.Meta, &p); err == nil {
				rec.proj = &p
				if p.Health != "" && !ont.Has("health", p.Health) {
					r.errf(rec.id, "unknown health value %q", p.Health)
				}
				if !ont.Has("project_status", p.Status) {
					r.warnf(rec.id, "unusual project status %q", p.Status)
				}
			}
		case model.TypeWorker:
			var wk model.Worker
			if err := model.Decode(doc.Meta, &wk); err == nil {
				if wk.Kind != "" && !ont.Has("worker_kind", wk.Kind) {
					r.errf(rec.id, "unknown worker kind %q", wk.Kind)
				}
				for _, c := range wk.Capabilities {
					if !ont.Has("capability", c) {
						r.errf(rec.id, "unknown capability %q", c)
					}
				}
			}
		}
		recs = append(recs, rec)
	}

	// Projects tree.
	projectsDir := filepath.Join(w.Root, "projects")
	if pds, err := os.ReadDir(projectsDir); err == nil {
		for _, pd := range pds {
			if !pd.IsDir() {
				continue
			}
			base := filepath.Join("projects", pd.Name())
			walk(filepath.ToSlash(filepath.Join(base, "PROJECT.md")), "project")
			walkDir(w.Root, filepath.Join(base, "items"), "item", walk)
			walkDir(w.Root, filepath.Join(base, "decisions"), "decision", walk)
		}
	}
	walkDir(w.Root, "inbox", "item", walk)
	walkDir(w.Root, "workers", "worker", walk)
	return recs
}

func walkDir(root, rel, want string, walk func(string, string)) {
	entries, err := os.ReadDir(filepath.Join(root, rel))
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		walk(filepath.ToSlash(filepath.Join(rel, e.Name())), want)
	}
}

func checkReferences(r *Report, recs []record, byID map[string]*record) {
	ref := func(from, rel, to string) {
		if to == "" {
			return
		}
		if _, ok := byID[to]; !ok {
			r.errf(from, "%s references unknown object %s", rel, to)
		}
	}
	for _, rec := range recs {
		if rec.item != nil {
			it := rec.item
			if it.Project != "" {
				if p, ok := byID[it.Project]; !ok {
					r.errf(rec.id, "references unknown project %s", it.Project)
				} else if p.typ != model.TypeProject {
					r.errf(rec.id, "project field points to non-project %s", it.Project)
				}
			}
			ref(rec.id, "parent", it.Parent)
			ref(rec.id, "goal", it.Goal)
			ref(rec.id, "duplicate_of", it.DuplicateOf)
			for _, d := range it.DependsOn {
				ref(rec.id, "depends_on", d)
			}
			for _, b := range it.BlockedBy {
				ref(rec.id, "blocked_by", b)
			}
			for _, b := range it.Blocks {
				ref(rec.id, "blocks", b)
			}
			for _, d := range it.DerivedFrom {
				ref(rec.id, "derived_from", d)
			}
		}
		if rec.dec != nil {
			for _, s := range rec.dec.Supersedes {
				ref(rec.id, "supersedes", s)
			}
			ref(rec.id, "superseded_by", rec.dec.SupersededBy)
			ref(rec.id, "project", rec.dec.Project)
		}
	}
}

func checkCycles(r *Report, recs []record, byID map[string]*record) {
	// parent chain cycles.
	parent := map[string]string{}
	depends := map[string][]string{}
	for _, rec := range recs {
		if rec.item == nil {
			continue
		}
		parent[rec.id] = rec.item.Parent
		depends[rec.id] = rec.item.DependsOn
	}
	for start := range parent {
		seen := map[string]bool{}
		cur := start
		for cur != "" {
			if seen[cur] {
				r.errf(start, "parent cycle detected")
				break
			}
			seen[cur] = true
			cur = parent[cur]
		}
	}
	// dependency cycles (DFS).
	color := map[string]int{} // 0 white,1 gray,2 black
	var dfs func(string) bool
	dfs = func(n string) bool {
		color[n] = 1
		for _, m := range depends[n] {
			if _, ok := byID[m]; !ok {
				continue
			}
			if color[m] == 1 {
				return true
			}
			if color[m] == 0 && dfs(m) {
				return true
			}
		}
		color[n] = 2
		return false
	}
	for n := range depends {
		if color[n] == 0 && dfs(n) {
			r.errf(n, "dependency cycle detected")
		}
	}
}

func checkLifecycle(r *Report, recs []record, byID map[string]*record, now time.Time) {
	for _, rec := range recs {
		if rec.item == nil {
			continue
		}
		it := rec.item
		if it.Status == model.StatusDone {
			for _, d := range it.DependsOn {
				if dep, ok := byID[d]; ok && dep.status != model.StatusDone && dep.status != model.StatusCancelled {
					r.warnf(rec.id, "done but dependency %s is %s", d, dep.status)
				}
			}
		}
		if it.Status == model.StatusBlocked && len(it.BlockedBy) == 0 {
			r.warnf(rec.id, "blocked without blocked_by")
		}
		if it.Status == model.StatusInProgress && it.LeaseUntil != "" {
			if t, err := time.Parse(time.RFC3339, it.LeaseUntil); err == nil && now.After(t) {
				r.warnf(rec.id, "active lease expired at %s", it.LeaseUntil)
			}
		}
	}
}

func checkEvents(w *store.Workspace, r *Report, ont *model.Ontology) {
	evs, err := eventlog.ReadAll(w.Root)
	if err != nil {
		r.errf("", "cannot read events: %v", err)
		return
	}
	for _, e := range evs {
		if !ont.Has("event_action", e.Action) {
			r.errf(e.ID, "unknown event action %q", e.Action)
		}
	}
}

// checkStaleIndex compares committed indexes against a fresh rebuild.
func checkStaleIndex(w *store.Workspace, r *Report) {
	if w.Config.IndexPolicy != "committed" {
		return
	}
	res, err := index.Build(w, false)
	if err != nil {
		r.warnf("", "cannot rebuild index for staleness check: %v", err)
		return
	}
	compare := func(file string, want []byte) {
		p := filepath.Join(w.Root, index.Dir, file)
		got, err := os.ReadFile(p)
		if err != nil {
			r.warnf("indexes/"+file, "index missing; run `workgraph index`")
			return
		}
		if strings.TrimRight(string(got), "\n") != strings.TrimRight(string(want), "\n") {
			r.warnf("indexes/"+file, "index is stale; run `workgraph index`")
		}
	}
	compare(index.FileObjects, index.RenderJSONL(res.Objects))
	compare(index.FileLinks, index.RenderJSONL(res.Links))
	compare(index.FileRuns, index.RenderJSONL(res.Runs))
	compare(index.FileAttention, index.RenderJSONL(res.Attention))
}

func checkDates(r *Report, oid string, dates ...string) {
	for _, d := range dates {
		if d == "" {
			continue
		}
		if _, err := time.Parse(time.RFC3339, d); err == nil {
			continue
		}
		if _, err := time.Parse("2006-01-02", d); err == nil {
			continue
		}
		r.errf(oid, "invalid date %q (want RFC3339 or YYYY-MM-DD)", d)
	}
}
