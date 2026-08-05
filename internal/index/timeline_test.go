package index

import (
	"testing"

	"github.com/moul/workgraph/internal/eventlog"
)

func TestBuildTimelineNewestFirst(t *testing.T) {
	evs := []eventlog.Event{ // arrive oldest-first, as ReadAll sorts them
		{At: "2026-08-04T21:00:00Z", Action: "item.created", Object: "ITM-1"},
		{At: "2026-08-04T21:05:00Z", Action: "item.status_changed", Object: "ITM-1", From: "ready", To: "in_progress"},
		{At: "2026-08-04T21:10:00Z", Action: "run.finished", Object: "ITM-1", Status: "review"},
	}
	tl := buildTimeline(evs)
	if len(tl) != 3 {
		t.Fatalf("len = %d, want 3", len(tl))
	}
	if tl[0].Action != "run.finished" {
		t.Errorf("newest first expected run.finished, got %q", tl[0].Action)
	}
	if tl[2].Action != "item.created" {
		t.Errorf("oldest last expected item.created, got %q", tl[2].Action)
	}
}

func TestBuildTimelineCap(t *testing.T) {
	var evs []eventlog.Event
	for i := 0; i < TimelineMax+50; i++ {
		evs = append(evs, eventlog.Event{At: "2026-08-04T21:00:00Z", Action: "item.created"})
	}
	if got := len(buildTimeline(evs)); got != TimelineMax {
		t.Errorf("timeline len = %d, want cap %d", got, TimelineMax)
	}
}
