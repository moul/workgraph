package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/moul/workgraph/internal/model"
	"github.com/moul/workgraph/internal/store"
)

func newEngine(t *testing.T) *Engine {
	t.Helper()
	root := t.TempDir()
	cfg := store.DefaultConfig()
	cfg.WorkspaceID = "WKG-T"
	cfg.IndexPolicy = "committed"
	if err := cfg.Save(root); err != nil {
		t.Fatal(err)
	}
	ws, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	// NoCommit so tests need no git repo; index still rebuilds to files.
	return New(ws, Options{Actor: "human:moul", NoCommit: true})
}

func TestFullLoop(t *testing.T) {
	e := newEngine(t)

	proj, err := e.CreateProject("Hermes", "git@github.com:moul/hermes.git", "main")
	if err != nil {
		t.Fatal(err)
	}
	it, err := e.CreateItem("Add run summary", proj.ID, "task", true)
	if err != nil {
		t.Fatal(err)
	}
	if it.Status != model.StatusReady {
		t.Errorf("ready item status = %q", it.Status)
	}
	// Item inherits no target repo yet; set one via update.
	if _, err := e.UpdateItem(it.ID, func(i *model.Item) { i.TargetRepo = proj.TargetRepo }, "", "set target"); err != nil {
		t.Fatal(err)
	}

	// Create a run with a target repo dir -> capsule.
	target := t.TempDir()
	res, err := e.CreateRun(it.ID, "agent:claude", "claude", target, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if res.Round != 1 {
		t.Errorf("round = %d", res.Round)
	}
	capFile := filepath.Join(res.CapsuleDir, "PROMPT.md")
	if _, err := os.Stat(capFile); err != nil {
		t.Errorf("capsule PROMPT.md missing: %v", err)
	}
	runJSON := filepath.Join(res.CapsuleDir, "RUN.json")
	if b, err := os.ReadFile(runJSON); err != nil || len(b) == 0 {
		t.Errorf("RUN.json missing")
	}

	// Item should now be in_progress and claimed.
	g, _ := e.WS.Load()
	cur := g.ByID(it.ID).(*model.Item)
	if cur.Status != model.StatusInProgress || cur.RunID != res.Run.RunID {
		t.Errorf("after run: status=%q runID=%q", cur.Status, cur.RunID)
	}

	// Finish the run to review.
	after, err := e.Finish(res.Run.RunID, "review", "Opened PR", "github:moul/hermes#123")
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != model.StatusReview || after.RunID != "" {
		t.Errorf("after finish: status=%q runID=%q", after.Status, after.RunID)
	}
}

func TestVersionConflict(t *testing.T) {
	e := newEngine(t)
	proj, _ := e.CreateProject("P", "", "")
	it, _ := e.CreateItem("T", proj.ID, "task", true)
	if _, err := e.UpdateItem(it.ID, func(i *model.Item) { i.Priority = "high" }, "blob:deadbeef", "bad"); err == nil {
		t.Error("expected version conflict error")
	}
	// Correct version succeeds.
	g, _ := e.WS.Load()
	cur := g.ByID(it.ID).(*model.Item)
	ver := e.ItemVersion(cur)
	if _, err := e.UpdateItem(it.ID, func(i *model.Item) { i.Priority = "low" }, ver, "ok"); err != nil {
		t.Errorf("expected success with correct version: %v", err)
	}
}

func TestSingleParallelPolicyBlocksSecondRun(t *testing.T) {
	e := newEngine(t)
	proj, _ := e.CreateProject("P", "", "")
	it, _ := e.CreateItem("T", proj.ID, "task", true)
	if _, err := e.CreateRun(it.ID, "agent:a", "generic", "", "auto"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.CreateRun(it.ID, "agent:b", "generic", "", "auto"); err == nil {
		t.Error("expected second run to be refused for single parallel policy")
	}
}
