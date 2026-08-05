package index

import (
	"testing"
	"time"

	"github.com/moul/workgraph/internal/eventlog"
	"github.com/moul/workgraph/internal/model"
	"github.com/moul/workgraph/internal/store"
)

func inProgressGraph(t *testing.T, lease string) (*store.Graph, string) {
	t.Helper()
	root := t.TempDir()
	cfg := store.DefaultConfig()
	cfg.WorkspaceID = "WKG-HB"
	if err := cfg.Save(root); err != nil {
		t.Fatal(err)
	}
	ws, _ := store.Open(root)
	it := &model.Item{
		Common: model.Common{ID: "ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F", Type: "item", Title: "T", Status: model.StatusInProgress},
		RunID:  "RUN-01K4A2D9Q9N7H2EA2A0P5X0R01",
	}
	it.LeaseUntil = lease
	if _, err := ws.Save(it); err != nil {
		t.Fatal(err)
	}
	g, _ := ws.Load()
	return g, it.ID
}

func TestNoHeartbeatAttention(t *testing.T) {
	g, itemID := inProgressGraph(t, "")
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	// Last activity 3h ago -> stale.
	evs := []eventlog.Event{
		{Action: "run.started", Run: "RUN-01K4A2D9Q9N7H2EA2A0P5X0R01", Object: itemID, At: now.Add(-3 * time.Hour).Format(time.RFC3339)},
	}
	att := buildAttention(g, evs, now)
	if !hasReason(att, "no_heartbeat") {
		t.Fatalf("expected no_heartbeat, got %+v", att)
	}
}

func TestFreshHeartbeatNoAttention(t *testing.T) {
	g, itemID := inProgressGraph(t, "")
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	evs := []eventlog.Event{
		{Action: "run.heartbeat", Run: "RUN-01K4A2D9Q9N7H2EA2A0P5X0R01", Object: itemID, At: now.Add(-5 * time.Minute).Format(time.RFC3339)},
	}
	att := buildAttention(g, evs, now)
	if hasReason(att, "no_heartbeat") {
		t.Fatalf("did not expect no_heartbeat for a fresh heartbeat, got %+v", att)
	}
}

func TestExpiredLeaseSuppressesHeartbeat(t *testing.T) {
	// When the lease is already expired, the louder lease_expired wins and
	// no_heartbeat is suppressed to avoid double-flagging.
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	g, itemID := inProgressGraph(t, now.Add(-1*time.Hour).Format(time.RFC3339))
	evs := []eventlog.Event{
		{Action: "run.started", Run: "RUN-01K4A2D9Q9N7H2EA2A0P5X0R01", Object: itemID, At: now.Add(-3 * time.Hour).Format(time.RFC3339)},
	}
	att := buildAttention(g, evs, now)
	if !hasReason(att, "lease_expired") {
		t.Errorf("expected lease_expired")
	}
	if hasReason(att, "no_heartbeat") {
		t.Errorf("no_heartbeat should be suppressed when lease already expired")
	}
}

func hasReason(att []AttentionLine, reason string) bool {
	for _, a := range att {
		if a.Reason == reason {
			return true
		}
	}
	return false
}
