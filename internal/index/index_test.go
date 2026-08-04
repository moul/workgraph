package index

import (
	"testing"
	"time"

	"github.com/moul/workgraph/internal/eventlog"
	"github.com/moul/workgraph/internal/model"
	"github.com/moul/workgraph/internal/store"
)

func TestBlobVersionMatchesGit(t *testing.T) {
	// echo -n "hello" | git hash-object --stdin -> b6fc4c620b67d95f953a5c1c1230aaab5db5a1b0
	got := BlobVersion([]byte("hello"))
	want := "blob:b6fc4c620b67d95f953a5c1c1230aaab5db5a1b0"
	if got != want {
		t.Errorf("BlobVersion = %q, want %q", got, want)
	}
}

func buildGraph(t *testing.T) (*store.Workspace, *store.Graph) {
	t.Helper()
	root := t.TempDir()
	cfg := store.DefaultConfig()
	cfg.WorkspaceID = "WKG-T"
	if err := cfg.Save(root); err != nil {
		t.Fatal(err)
	}
	ws, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	proj := &model.Project{Common: model.Common{ID: "PRJ-01K4A2D9Q9N7H2EA2A0P5X0T6A", Type: "project", Title: "Hermes", Status: "active"}}
	if _, err := ws.Save(proj); err != nil {
		t.Fatal(err)
	}
	it := &model.Item{Common: model.Common{ID: "ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F", Type: "item", Title: "Add run summary", Status: "ready"}, Kind: "task", Project: proj.ID, DependsOn: []string{"ITM-01K4A2D9Q9N7H2EA2A0P5X0T6G"}}
	it.SetBody("# x\n\n## Goal\nSummarize runs compactly.\n")
	if _, err := ws.Save(it); err != nil {
		t.Fatal(err)
	}
	g, err := ws.Load()
	if err != nil {
		t.Fatal(err)
	}
	return ws, g
}

func TestBuildObjectsAndSummary(t *testing.T) {
	ws, _ := buildGraph(t)
	res, err := Build(ws, true)
	if err != nil {
		t.Fatal(err)
	}
	var item *ObjectLine
	for i := range res.Objects {
		if res.Objects[i].Type == "item" {
			item = &res.Objects[i]
		}
	}
	if item == nil {
		t.Fatal("no item in objects index")
	}
	if item.Version == "" {
		t.Error("version not computed")
	}
	if item.Summary != "Summarize runs compactly." {
		t.Errorf("summary = %q", item.Summary)
	}
}

func TestBuildAttentionMissingDependency(t *testing.T) {
	ws, _ := buildGraph(t)
	res, err := Build(ws, false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range res.Attention {
		if a.Reason == "missing_dependency" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected missing_dependency attention, got %+v", res.Attention)
	}
}

func TestBuildRuns(t *testing.T) {
	_, g := buildGraph(t)
	evs := []eventlog.Event{
		{Action: "run.created", Run: "RUN-1", Object: "ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F", Worker: "agent:claude", Round: 1, At: "2026-08-04T21:00:00Z"},
		{Action: "run.finished", Run: "RUN-1", Object: "ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F", Status: "review", PR: "github:moul/hermes#123", Summary: "Opened PR", At: "2026-08-04T22:00:00Z"},
	}
	runs := buildRuns(g, evs)
	if len(runs) != 1 {
		t.Fatalf("got %d runs", len(runs))
	}
	r := runs[0]
	if r.Status != "review" || r.PR != "github:moul/hermes#123" || r.Round != 1 {
		t.Errorf("run = %+v", r)
	}
	_ = time.Now
}
