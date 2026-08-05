package index

import (
	"testing"
	"time"

	"github.com/moul/workgraph/internal/model"
	"github.com/moul/workgraph/internal/store"
)

func healthWS(t *testing.T) *store.Workspace {
	t.Helper()
	root := t.TempDir()
	cfg := store.DefaultConfig()
	cfg.WorkspaceID = "WKG-H"
	if err := cfg.Save(root); err != nil {
		t.Fatal(err)
	}
	ws, _ := store.Open(root)
	return ws
}

func project(t *testing.T, ws *store.Workspace, id, health string) {
	t.Helper()
	p := &model.Project{Common: model.Common{ID: id, Type: "project", Title: id, Status: "active"}, Health: health}
	if _, err := ws.Save(p); err != nil {
		t.Fatal(err)
	}
}

func item(t *testing.T, ws *store.Workspace, id, project, status string) *model.Item {
	t.Helper()
	it := &model.Item{Common: model.Common{ID: id, Type: "item", Title: id, Status: status}, Project: project}
	if _, err := ws.Save(it); err != nil {
		t.Fatal(err)
	}
	return it
}

func suggestedFor(rows []HealthLine, project string) HealthLine {
	for _, r := range rows {
		if r.Project == project {
			return r
		}
	}
	return HealthLine{}
}

func TestHealthUnknownWhenEmpty(t *testing.T) {
	ws := healthWS(t)
	project(t, ws, "PRJ-01K4A2D9Q9N7H2EA2A0P5X0T00", "")
	g, _ := ws.Load()
	got := suggestedFor(Health(g, time.Now()), "PRJ-01K4A2D9Q9N7H2EA2A0P5X0T00")
	if got.Suggested != "unknown" {
		t.Errorf("empty project = %q, want unknown", got.Suggested)
	}
}

func TestHealthOnTrack(t *testing.T) {
	ws := healthWS(t)
	pid := "PRJ-01K4A2D9Q9N7H2EA2A0P5X0T00"
	project(t, ws, pid, "")
	item(t, ws, "ITM-01K4A2D9Q9N7H2EA2A0P5X0T01", pid, "in_progress")
	item(t, ws, "ITM-01K4A2D9Q9N7H2EA2A0P5X0T02", pid, "ready")
	g, _ := ws.Load()
	got := suggestedFor(Health(g, time.Now()), pid)
	if got.Suggested != "on_track" {
		t.Errorf("suggested = %q, want on_track (reasons=%v)", got.Suggested, got.Reasons)
	}
}

func TestHealthAtRiskOnBlocked(t *testing.T) {
	ws := healthWS(t)
	pid := "PRJ-01K4A2D9Q9N7H2EA2A0P5X0T00"
	project(t, ws, pid, "")
	item(t, ws, "ITM-01K4A2D9Q9N7H2EA2A0P5X0T01", pid, "in_progress")
	item(t, ws, "ITM-01K4A2D9Q9N7H2EA2A0P5X0T03", pid, "blocked")
	g, _ := ws.Load()
	got := suggestedFor(Health(g, time.Now()), pid)
	if got.Suggested != "at_risk" {
		t.Errorf("suggested = %q, want at_risk", got.Suggested)
	}
	if len(got.Reasons) == 0 {
		t.Errorf("expected a reason for at_risk")
	}
}

func TestHealthBlockedWhenNothingActionable(t *testing.T) {
	ws := healthWS(t)
	pid := "PRJ-01K4A2D9Q9N7H2EA2A0P5X0T00"
	project(t, ws, pid, "")
	item(t, ws, "ITM-01K4A2D9Q9N7H2EA2A0P5X0T03", pid, "blocked")
	g, _ := ws.Load()
	got := suggestedFor(Health(g, time.Now()), pid)
	if got.Suggested != "blocked" {
		t.Errorf("suggested = %q, want blocked (nothing actionable)", got.Suggested)
	}
}

func TestHealthStaleReview(t *testing.T) {
	ws := healthWS(t)
	pid := "PRJ-01K4A2D9Q9N7H2EA2A0P5X0T00"
	project(t, ws, pid, "")
	it := item(t, ws, "ITM-01K4A2D9Q9N7H2EA2A0P5X0T04", pid, "review")
	old := time.Now().Add(-10 * 24 * time.Hour).Format(time.RFC3339)
	it.UpdatedAt = old
	if _, err := ws.Save(it); err != nil {
		t.Fatal(err)
	}
	g, _ := ws.Load()
	got := suggestedFor(Health(g, time.Now()), pid)
	if got.Suggested != "at_risk" {
		t.Errorf("suggested = %q, want at_risk for stale review", got.Suggested)
	}
}
