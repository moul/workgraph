// Package index builds the deterministic, diff-friendly JSONL indexes that make
// Workgraph cheap to read for agents on weak machines and outdated clones.
//
// The indexes are committed but generated: always rebuildable from source, and
// the validator warns when they are stale. Every line is one compact JSON
// object so a tired human can query with jq:
//
//	jq -r 'select(.status=="ready") | [.id,.title,.target_repo] | @tsv' indexes/objects.jsonl
package index

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/moul/workgraph/internal/eventlog"
	"github.com/moul/workgraph/internal/model"
	"github.com/moul/workgraph/internal/store"
)

// Dir is the indexes directory within a workspace.
const Dir = "indexes"

// Files produced by Build.
const (
	FileObjects   = "objects.jsonl"
	FileLinks     = "links.jsonl"
	FileRuns      = "runs.jsonl"
	FileAttention = "attention.jsonl"
)

// ObjectLine is one row of indexes/objects.jsonl.
type ObjectLine struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Kind       string `json:"kind,omitempty"`
	Status     string `json:"status"`
	Title      string `json:"title"`
	Project    string `json:"project,omitempty"`
	Owner      string `json:"owner,omitempty"`
	Priority   string `json:"priority,omitempty"`
	TargetRepo string `json:"target_repo,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
	Path       string `json:"path"`
	Version    string `json:"version"`
	Summary    string `json:"summary,omitempty"`
}

// LinkLine is one row of indexes/links.jsonl.
type LinkLine struct {
	From string `json:"from"`
	Rel  string `json:"rel"`
	To   string `json:"to"`
}

// RunLine is one row of indexes/runs.jsonl.
type RunLine struct {
	Run        string `json:"run"`
	Item       string `json:"item"`
	Round      int    `json:"round,omitempty"`
	Worker     string `json:"worker,omitempty"`
	Status     string `json:"status,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	EndedAt    string `json:"ended_at,omitempty"`
	TargetRepo string `json:"target_repo,omitempty"`
	PR         string `json:"pr,omitempty"`
	Summary    string `json:"summary,omitempty"`
}

// AttentionLine is one row of indexes/attention.jsonl.
type AttentionLine struct {
	ID       string `json:"id"`
	Reason   string `json:"reason"`
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
}

// Result is the in-memory index, returned by Build for callers that want the
// data without re-reading files.
type Result struct {
	Objects   []ObjectLine
	Links     []LinkLine
	Runs      []RunLine
	Attention []AttentionLine
}

// Build loads the workspace graph and events, computes all indexes, and (when
// write is true) writes them to indexes/*.jsonl.
func Build(w *store.Workspace, write bool) (*Result, error) {
	g, err := w.Load()
	if err != nil {
		return nil, err
	}
	evs, err := eventlog.ReadAll(w.Root)
	if err != nil {
		return nil, err
	}
	res := &Result{}
	res.Objects = buildObjects(w.Root, g)
	res.Links = buildLinks(g)
	res.Runs = buildRuns(g, evs)
	res.Attention = buildAttention(g, evs, time.Now())

	if write {
		dir := filepath.Join(w.Root, Dir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		if err := writeJSONL(filepath.Join(dir, FileObjects), res.Objects); err != nil {
			return nil, err
		}
		if err := writeJSONL(filepath.Join(dir, FileLinks), res.Links); err != nil {
			return nil, err
		}
		if err := writeJSONL(filepath.Join(dir, FileRuns), res.Runs); err != nil {
			return nil, err
		}
		if err := writeJSONL(filepath.Join(dir, FileAttention), res.Attention); err != nil {
			return nil, err
		}
	}
	return res, nil
}

func buildObjects(root string, g *store.Graph) []ObjectLine {
	var out []ObjectLine
	for _, o := range g.All() {
		line := ObjectLine{
			ID:      o.ObjectID(),
			Type:    o.ObjectType(),
			Status:  o.ObjectStatus(),
			Title:   o.ObjectTitle(),
			Path:    o.SourcePath(),
			Version: versionOf(root, o.SourcePath()),
			Summary: summarize(o),
		}
		switch t := o.(type) {
		case *model.Item:
			line.Kind = t.Kind
			line.Project = t.Project
			line.Owner = t.Owner
			line.Priority = t.Priority
			line.TargetRepo = t.TargetRepo
			line.UpdatedAt = t.UpdatedAt
		case *model.Project:
			line.UpdatedAt = t.UpdatedAt
			line.TargetRepo = t.TargetRepo
		case *model.Decision:
			line.Project = t.Project
			line.UpdatedAt = t.UpdatedAt
		case *model.Worker:
			line.Kind = t.Kind
		}
		out = append(out, line)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func versionOf(root, relPath string) string {
	raw, err := os.ReadFile(filepath.Join(root, relPath))
	if err != nil {
		return ""
	}
	return BlobVersion(raw)
}

func buildLinks(g *store.Graph) []LinkLine {
	var out []LinkLine
	add := func(from, rel, to string) {
		if to == "" {
			return
		}
		out = append(out, LinkLine{From: from, Rel: rel, To: to})
	}
	for _, it := range g.Items {
		if it.Parent != "" {
			add(it.Parent, "parent_of", it.ID)
		}
		for _, d := range it.DependsOn {
			add(it.ID, "depends_on", d)
		}
		for _, b := range it.Blocks {
			add(it.ID, "blocks", b)
		}
		for _, b := range it.BlockedBy {
			add(it.ID, "blocked_by", b)
		}
		for _, d := range it.DerivedFrom {
			add(it.ID, "derived_from", d)
		}
		add(it.ID, "duplicate_of", it.DuplicateOf)
		if it.TargetRepo != "" {
			add(it.ID, "targets_repo", it.TargetRepo)
		}
	}
	for _, d := range g.Decisions {
		for _, s := range d.Supersedes {
			add(d.ID, "supersedes", s)
		}
		add(d.ID, "superseded_by", d.SupersededBy)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		if out[i].Rel != out[j].Rel {
			return out[i].Rel < out[j].Rel
		}
		return out[i].To < out[j].To
	})
	return out
}

// summarize returns a compact one-line summary for an object: the first
// meaningful section paragraph, trimmed. Agents choose work from this without
// reading the full body.
func summarize(o model.Object) string {
	b, ok := o.(interface{ Body() string })
	if !ok {
		return ""
	}
	body := b.Body()
	// Prefer the paragraph under a Goal/Outcome/Purpose/Summary heading.
	for _, h := range []string{"## Goal", "## Outcome", "## Purpose", "## Summary", "## Context"} {
		if s := sectionParagraph(body, h); s != "" {
			return truncate(s, 200)
		}
	}
	// Otherwise the first non-heading, non-empty, non-placeholder line.
	for _, ln := range strings.Split(body, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") || isPlaceholder(ln) {
			continue
		}
		return truncate(ln, 200)
	}
	return ""
}

// isPlaceholder reports whether a line is scaffold filler rather than real
// content: an HTML comment (<!-- … -->) or a line wholly wrapped in emphasis
// markers (_…_ or *…*), such as the "_Describe the project._" scaffold text.
// Such lines must not leak into an object's summary.
func isPlaceholder(t string) bool {
	if strings.HasPrefix(t, "<!--") {
		return true
	}
	// A bare list marker with no content ("-", "*", "+") is an empty scaffold
	// bullet, not a summary.
	if t == "-" || t == "*" || t == "+" {
		return true
	}
	if len(t) >= 2 {
		if (t[0] == '_' && t[len(t)-1] == '_') || (t[0] == '*' && t[len(t)-1] == '*') {
			return true
		}
	}
	return false
}

func sectionParagraph(body, heading string) string {
	idx := strings.Index(body, heading)
	if idx < 0 {
		return ""
	}
	rest := body[idx+len(heading):]
	var para []string
	for _, ln := range strings.Split(rest, "\n") {
		t := strings.TrimSpace(ln)
		if t == "" {
			if len(para) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(t, "#") {
			if len(para) > 0 {
				break
			}
			continue
		}
		if isPlaceholder(t) {
			if len(para) > 0 {
				break
			}
			continue
		}
		para = append(para, t)
	}
	return strings.Join(para, " ")
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n]) + "…"
}

func writeJSONL[T any](path string, rows []T) error {
	return os.WriteFile(path, RenderJSONL(rows), 0o644)
}

// RenderJSONL serializes rows to newline-delimited JSON deterministically. It is
// exported so the validator can compare committed indexes against a rebuild
// without touching disk.
func RenderJSONL[T any](rows []T) []byte {
	var sb strings.Builder
	for _, r := range rows {
		b, err := json.Marshal(r)
		if err != nil {
			continue
		}
		sb.Write(b)
		sb.WriteByte('\n')
	}
	return []byte(sb.String())
}
