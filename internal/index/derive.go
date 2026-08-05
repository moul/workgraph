package index

import (
	"sort"
	"time"

	"github.com/moul/workgraph/internal/eventlog"
	"github.com/moul/workgraph/internal/model"
	"github.com/moul/workgraph/internal/store"
)

// buildRuns aggregates run.* events into one compact line per run, so item
// history renders without reading every event file.
func buildRuns(g *store.Graph, evs []eventlog.Event) []RunLine {
	runs := map[string]*RunLine{}
	order := []string{}
	repoOf := map[string]string{}
	for _, it := range g.Items {
		repoOf[it.ID] = it.TargetRepo
	}
	for _, e := range evs {
		if e.Run == "" {
			continue
		}
		r, ok := runs[e.Run]
		if !ok {
			r = &RunLine{Run: e.Run, Item: e.Object}
			runs[e.Run] = r
			order = append(order, e.Run)
		}
		if e.Object != "" {
			r.Item = e.Object
			if repo := repoOf[e.Object]; repo != "" {
				r.TargetRepo = repo
			}
		}
		if e.Round != 0 {
			r.Round = e.Round
		}
		if e.Worker != "" {
			r.Worker = e.Worker
		}
		switch e.Action {
		case "run.created":
			r.StartedAt = e.At
		case "run.started":
			if r.StartedAt == "" {
				r.StartedAt = e.At
			}
		case "run.blocked":
			r.Status = "blocked"
			if e.Reason != "" {
				r.Summary = e.Reason
			}
			r.EndedAt = e.At
		case "run.finished":
			if e.Status != "" {
				r.Status = e.Status
			} else {
				r.Status = "done"
			}
			if e.Summary != "" {
				r.Summary = e.Summary
			}
			if e.PR != "" {
				r.PR = e.PR
			}
			r.EndedAt = e.At
		case "run.review_requested":
			r.Status = "review"
		case "run.reviewed":
			r.Status = "reviewed"
		}
		if e.Status != "" && r.Status == "" {
			r.Status = e.Status
		}
	}
	out := make([]RunLine, 0, len(order))
	for _, id := range order {
		out = append(out, *runs[id])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Run < out[j].Run })
	return out
}

// HeartbeatStale is how long an in_progress run may go without a run.* event
// before it becomes a no_heartbeat attention item. Orchestrators are expected
// to emit run.heartbeat periodically; manual work simply won't trip this until
// the lease (if any) also lapses.
const HeartbeatStale = time.Hour

// buildAttention derives the human attention queue. Attention is mostly
// derived so stale manual flags cannot rot; a manual override is honored only
// until its expiry.
func buildAttention(g *store.Graph, evs []eventlog.Event, now time.Time) []AttentionLine {
	var out []AttentionLine
	add := func(id, reason, sev, summary string) {
		out = append(out, AttentionLine{ID: id, Reason: reason, Severity: sev, Summary: summary})
	}
	known := map[string]model.Object{}
	for _, o := range g.All() {
		known[o.ObjectID()] = o
	}

	// Latest activity timestamp per run, for heartbeat staleness detection.
	lastActivity := map[string]time.Time{}
	for _, e := range evs {
		if e.Run == "" || e.At == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, e.At); err == nil {
			if cur, ok := lastActivity[e.Run]; !ok || t.After(cur) {
				lastActivity[e.Run] = t
			}
		}
	}

	for _, it := range g.Items {
		switch it.Status {
		case model.StatusBlocked:
			if len(it.BlockedBy) == 0 {
				add(it.ID, "blocked_without_blocked_by", "medium", "Blocked but no blocked_by recorded.")
			} else {
				add(it.ID, "blocked_by_human", "high", "Blocked; needs a decision or external change.")
			}
		case model.StatusReview:
			add(it.ID, "review_assigned_to_human", "high", "Output exists and needs review.")
		case model.StatusTriage:
			add(it.ID, "new_triage_item", "low", "New item awaiting triage.")
		case model.StatusReady:
			for _, dep := range it.DependsOn {
				d, ok := known[dep]
				if !ok {
					add(it.ID, "missing_dependency", "high", "Depends on unknown item "+dep+".")
					break
				}
				if d.ObjectStatus() != model.StatusDone {
					add(it.ID, "missing_dependency", "medium", "Ready but dependency "+dep+" is not done.")
					break
				}
			}
		}
		// Lease expiry for active work.
		leaseExpired := false
		if it.Status == model.StatusInProgress && it.LeaseUntil != "" {
			if t, err := time.Parse(time.RFC3339, it.LeaseUntil); err == nil && now.After(t) {
				add(it.ID, "lease_expired", "high", "Active lease expired; run may be abandoned.")
				leaseExpired = true
			}
		}
		// Stale heartbeat: an active run with no recent activity. Skipped when the
		// lease already expired (that is the louder, sufficient signal).
		if it.Status == model.StatusInProgress && it.RunID != "" && !leaseExpired {
			if last, ok := lastActivity[it.RunID]; ok && now.Sub(last) > HeartbeatStale {
				add(it.ID, "no_heartbeat", "medium", "Active run has not reported progress since "+last.Format(time.RFC3339)+".")
			}
		}
		// Manual override, honored until expiry.
		if it.Attention {
			if it.AttentionUntil != "" {
				if t, err := time.Parse(time.RFC3339, it.AttentionUntil); err == nil && now.After(t) {
					continue
				}
			}
			reason := it.AttentionReason
			if reason == "" {
				reason = "Manual attention flag set."
			}
			add(it.ID, "manual_override", "medium", reason)
		}
	}

	// Proposed decisions surface as attention, not hard constraints.
	for _, d := range g.Decisions {
		if d.Status == "proposed" {
			add(d.ID, "proposed_decision", "low", "Proposed decision awaiting acceptance.")
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Reason < out[j].Reason
	})
	return out
}
