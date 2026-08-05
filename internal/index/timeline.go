package index

import "github.com/moul/workgraph/internal/eventlog"

// TimelineLine is one compact activity-feed row derived from the event log.
type TimelineLine struct {
	At      string `json:"at"`
	Actor   string `json:"actor,omitempty"`
	Action  string `json:"action"`
	Object  string `json:"object,omitempty"`
	Run     string `json:"run,omitempty"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Status  string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
}

// TimelineMax bounds how many recent events the in-memory timeline carries; the
// full history always remains in events/*.jsonl.
const TimelineMax = 200

// buildTimeline returns the most recent events newest-first (events arrive
// sorted oldest-first), capped at TimelineMax.
func buildTimeline(evs []eventlog.Event) []TimelineLine {
	start := 0
	if len(evs) > TimelineMax {
		start = len(evs) - TimelineMax
	}
	recent := evs[start:]
	out := make([]TimelineLine, 0, len(recent))
	for i := len(recent) - 1; i >= 0; i-- {
		e := recent[i]
		out = append(out, TimelineLine{
			At: e.At, Actor: e.Actor, Action: e.Action, Object: e.Object,
			Run: e.Run, From: e.From, To: e.To, Status: e.Status, Message: e.Message,
		})
	}
	return out
}
