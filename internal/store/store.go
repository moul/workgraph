// Package store loads and saves canonical Workgraph objects from a workspace
// directory and resolves human references (id, id-prefix, slug, or path) to a
// single object. The filesystem answers "where is the primary context?"; object
// metadata answers "how is this connected?".
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moul/workgraph/internal/frontmatter"
	"github.com/moul/workgraph/internal/id"
	"github.com/moul/workgraph/internal/model"
)

// Workspace is a loaded control repo.
type Workspace struct {
	Root     string
	Config   *Config
	Ontology *model.Ontology
}

// Open loads the workspace configuration and ontology from root. It does not
// load every object; call Load for that.
func Open(root string) (*Workspace, error) {
	root = filepath.Clean(root)
	cfg, err := LoadConfig(root)
	if err != nil {
		return nil, err
	}
	ont, err := model.LoadOntology(root)
	if err != nil {
		return nil, err
	}
	return &Workspace{Root: root, Config: cfg, Ontology: ont}, nil
}

// FindRoot walks up from dir looking for a workgraph.yaml, returning the
// workspace root or an error.
func FindRoot(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ConfigFile)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("store: no %s found in %s or any parent", ConfigFile, dir)
		}
		dir = parent
	}
}

// Graph is a fully loaded set of objects with lookup maps.
type Graph struct {
	Projects  []*model.Project
	Items     []*model.Item
	Decisions []*model.Decision
	Workers   []*model.Worker

	byID   map[string]model.Object
	byPath map[string]model.Object
}

// Load walks the workspace and loads every canonical object.
func (w *Workspace) Load() (*Graph, error) {
	g := &Graph{byID: map[string]model.Object{}, byPath: map[string]model.Object{}}

	// Projects, items, decisions live under projects/*.
	projectsDir := filepath.Join(w.Root, "projects")
	projDirs, _ := os.ReadDir(projectsDir)
	for _, pd := range projDirs {
		if !pd.IsDir() {
			continue
		}
		base := filepath.Join(projectsDir, pd.Name())
		// PROJECT.md
		if _, err := os.Stat(filepath.Join(base, "PROJECT.md")); err == nil {
			var p model.Project
			if err := w.loadInto(filepath.Join(base, "PROJECT.md"), &p); err != nil {
				return nil, err
			}
			g.add(&p)
			g.Projects = append(g.Projects, &p)
		}
		if err := loadDir(w, filepath.Join(base, "items"), func() model.Object { return &model.Item{} }, func(o model.Object) {
			g.add(o)
			g.Items = append(g.Items, o.(*model.Item))
		}); err != nil {
			return nil, err
		}
		if err := loadDir(w, filepath.Join(base, "decisions"), func() model.Object { return &model.Decision{} }, func(o model.Object) {
			g.add(o)
			g.Decisions = append(g.Decisions, o.(*model.Decision))
		}); err != nil {
			return nil, err
		}
	}

	// Top-level inbox items (project-less).
	if err := loadDir(w, filepath.Join(w.Root, "inbox"), func() model.Object { return &model.Item{} }, func(o model.Object) {
		g.add(o)
		g.Items = append(g.Items, o.(*model.Item))
	}); err != nil {
		return nil, err
	}

	// Workers.
	if err := loadDir(w, filepath.Join(w.Root, "workers"), func() model.Object { return &model.Worker{} }, func(o model.Object) {
		g.add(o)
		g.Workers = append(g.Workers, o.(*model.Worker))
	}); err != nil {
		return nil, err
	}

	g.sort()
	return g, nil
}

func loadDir(w *Workspace, dir string, mk func() model.Object, add func(model.Object)) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("store: readdir %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		obj := mk()
		if err := w.loadInto(filepath.Join(dir, e.Name()), obj); err != nil {
			return err
		}
		add(obj)
	}
	return nil
}

// loadInto reads a Markdown object file into obj (a *model.X) and records its
// repo-relative path and body.
func (w *Workspace) loadInto(path string, obj model.Object) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("store: read %s: %w", path, err)
	}
	doc, err := frontmatter.Parse(raw)
	if err != nil {
		return fmt.Errorf("store: %s: %w", relTo(w.Root, path), err)
	}
	if err := model.Decode(doc.Meta, obj); err != nil {
		return fmt.Errorf("store: %s: %w", relTo(w.Root, path), err)
	}
	rel := relTo(w.Root, path)
	obj.SetSourcePath(rel)
	if bs, ok := obj.(interface{ SetBody(string) }); ok {
		bs.SetBody(doc.Body)
	}
	return nil
}

func relTo(root, path string) string {
	if r, err := filepath.Rel(root, path); err == nil {
		return filepath.ToSlash(r)
	}
	return filepath.ToSlash(path)
}

func (g *Graph) add(o model.Object) {
	g.byID[o.ObjectID()] = o
	g.byPath[o.SourcePath()] = o
}

func (g *Graph) sort() {
	sort.SliceStable(g.Items, func(i, j int) bool { return g.Items[i].ID < g.Items[j].ID })
	sort.SliceStable(g.Projects, func(i, j int) bool { return g.Projects[i].ID < g.Projects[j].ID })
	sort.SliceStable(g.Decisions, func(i, j int) bool { return g.Decisions[i].ID < g.Decisions[j].ID })
	sort.SliceStable(g.Workers, func(i, j int) bool { return g.Workers[i].ID < g.Workers[j].ID })
}

// ByID returns the object with the given id, or nil.
func (g *Graph) ByID(oid string) model.Object { return g.byID[oid] }

// All returns every loaded object.
func (g *Graph) All() []model.Object {
	out := make([]model.Object, 0, len(g.byID))
	for _, p := range g.Projects {
		out = append(out, p)
	}
	for _, it := range g.Items {
		out = append(out, it)
	}
	for _, d := range g.Decisions {
		out = append(out, d)
	}
	for _, wk := range g.Workers {
		out = append(out, wk)
	}
	return out
}

// Resolve maps a human reference to exactly one object. Accepted forms:
//   - a full id ("ITM-01K...")
//   - a case-insensitive id prefix ("01K4A2", "itm-01k4a2")
//   - a slug appearing in a filename ("add-run-summary")
//   - a repo-relative path
//
// It returns an error when the reference is ambiguous or unknown so callers
// never act on the wrong object.
func (g *Graph) Resolve(ref string) (model.Object, error) {
	if ref == "" {
		return nil, fmt.Errorf("store: empty reference")
	}
	if o, ok := g.byID[ref]; ok {
		return o, nil
	}
	if o, ok := g.byPath[ref]; ok {
		return o, nil
	}
	if id.IsWorker(ref) {
		return nil, fmt.Errorf("store: unknown worker %q", ref)
	}

	low := strings.ToLower(ref)
	var matches []model.Object
	seen := map[string]bool{}
	for oid, o := range g.byID {
		lid := strings.ToLower(oid)
		// id prefix (after or including the type prefix)
		if strings.HasPrefix(lid, low) || strings.Contains(lid, "-"+low) {
			if !seen[oid] {
				matches = append(matches, o)
				seen[oid] = true
			}
			continue
		}
		// slug match on filename
		base := strings.TrimSuffix(filepath.Base(o.SourcePath()), ".md")
		if slugPart := afterID(base); slugPart != "" && strings.ToLower(slugPart) == low {
			if !seen[oid] {
				matches = append(matches, o)
				seen[oid] = true
			}
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("store: no object matches %q", ref)
	case 1:
		return matches[0], nil
	default:
		sort.Slice(matches, func(i, j int) bool { return matches[i].ObjectID() < matches[j].ObjectID() })
		var ids []string
		for _, m := range matches {
			ids = append(ids, m.ObjectID())
		}
		return nil, fmt.Errorf("store: %q is ambiguous: %s", ref, strings.Join(ids, ", "))
	}
}

// afterID returns the slug portion of a filename base ("ITM-01K...-slug" ->
// "slug"), or "".
func afterID(base string) string {
	oid := id.IDFromFilename(base + ".md")
	if oid == "" {
		return ""
	}
	rest := strings.TrimPrefix(base, oid)
	return strings.TrimPrefix(rest, "-")
}
