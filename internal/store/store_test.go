package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/moul/workgraph/internal/model"
)

func newWS(t *testing.T) *Workspace {
	t.Helper()
	root := t.TempDir()
	cfg := DefaultConfig()
	cfg.WorkspaceID = "WKG-TEST"
	if err := cfg.Save(root); err != nil {
		t.Fatal(err)
	}
	ws, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return ws
}

func TestSaveLoadRoundTrip(t *testing.T) {
	ws := newWS(t)

	proj := &model.Project{Common: model.Common{ID: "PRJ-01K4A2D9Q9N7H2EA2A0P5X0T6A", Type: "project", Title: "Hermes", Status: "active"}}
	proj.SetBody("# Hermes\n\n## Purpose\nDo things.\n")
	if _, err := ws.Save(proj); err != nil {
		t.Fatal(err)
	}

	item := &model.Item{Common: model.Common{ID: "ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F", Type: "item", Title: "Add run summary", Status: "ready"}, Kind: "task", Project: proj.ID, Priority: "high"}
	item.SetBody("# Add run summary\n\n## Goal\nSummarize runs.\n")
	relPath, err := ws.Save(item)
	if err != nil {
		t.Fatal(err)
	}
	if relPath != "projects/hermes/items/ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F-add-run-summary.md" {
		t.Errorf("unexpected item path %q", relPath)
	}

	g, err := ws.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Projects) != 1 || len(g.Items) != 1 {
		t.Fatalf("loaded %d projects, %d items", len(g.Projects), len(g.Items))
	}
	got := g.Items[0]
	if got.Title != "Add run summary" || got.Kind != "task" || got.Priority != "high" {
		t.Errorf("item round-trip lost data: %+v", got)
	}
	if got.Body() == "" {
		t.Errorf("body not preserved")
	}
}

func TestResolve(t *testing.T) {
	ws := newWS(t)
	proj := &model.Project{Common: model.Common{ID: "PRJ-01K4A2D9Q9N7H2EA2A0P5X0T6A", Type: "project", Title: "Hermes", Status: "active"}}
	if _, err := ws.Save(proj); err != nil {
		t.Fatal(err)
	}
	item := &model.Item{Common: model.Common{ID: "ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F", Type: "item", Title: "Add run summary", Status: "ready"}, Project: proj.ID}
	if _, err := ws.Save(item); err != nil {
		t.Fatal(err)
	}
	g, err := ws.Load()
	if err != nil {
		t.Fatal(err)
	}

	// Full id.
	if o, err := g.Resolve(item.ID); err != nil || o.ObjectID() != item.ID {
		t.Errorf("resolve by id: %v %v", o, err)
	}
	// Slug.
	if o, err := g.Resolve("add-run-summary"); err != nil || o.ObjectID() != item.ID {
		t.Errorf("resolve by slug: %v %v", o, err)
	}
	// Case-insensitive id fragment.
	if o, err := g.Resolve("01k4a2d9q9n7h2ea2a0p5x0t6f"); err != nil || o.ObjectID() != item.ID {
		t.Errorf("resolve by fragment: %v %v", o, err)
	}
	// Unknown.
	if _, err := g.Resolve("nope-nothing"); err == nil {
		t.Errorf("expected error for unknown ref")
	}
}

func TestFindRoot(t *testing.T) {
	ws := newWS(t)
	sub := filepath.Join(ws.Root, "projects", "x", "items")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := FindRoot(sub)
	if err != nil {
		t.Fatal(err)
	}
	if got != ws.Root {
		t.Errorf("FindRoot = %q, want %q", got, ws.Root)
	}
}
