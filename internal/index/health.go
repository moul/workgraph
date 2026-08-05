package index

import (
	"fmt"
	"sort"
	"time"

	"github.com/moul/workgraph/internal/model"
	"github.com/moul/workgraph/internal/store"
)

// HealthLine is a qualitative, explainable health suggestion for a project.
// Health is never a percentage — it is one of unknown/on_track/at_risk/blocked
// with the reasons that produced it, so a human can judge rather than trust a
// fabricated number.
type HealthLine struct {
	Project   string   `json:"project"`
	Title     string   `json:"title"`
	Explicit  string   `json:"explicit,omitempty"` // health set on PROJECT.md, if any
	Suggested string   `json:"suggested_health"`
	Reasons   []string `json:"reasons,omitempty"`
}

// staleReviewDays is how old the oldest review item may be before it counts
// against a project's health.
const staleReviewDays = 7

// Health derives a suggested health per project from its items. now is passed
// in for deterministic testing; because the result depends on time it is
// computed on demand and never written to a committed index.
func Health(g *store.Graph, now time.Time) []HealthLine {
	// Bucket items by project.
	byProject := map[string][]*model.Item{}
	known := map[string]model.Object{}
	for _, o := range g.All() {
		known[o.ObjectID()] = o
	}
	for _, it := range g.Items {
		if it.Project == "" {
			continue
		}
		byProject[it.Project] = append(byProject[it.Project], it)
	}

	var out []HealthLine
	for _, p := range g.Projects {
		hl := HealthLine{Project: p.ID, Title: p.Title, Explicit: p.Health}
		hl.Suggested, hl.Reasons = suggestHealth(byProject[p.ID], known, now)
		out = append(out, hl)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Project < out[j].Project })
	return out
}

func suggestHealth(items []*model.Item, known map[string]model.Object, now time.Time) (string, []string) {
	var active []*model.Item
	for _, it := range items {
		switch it.Status {
		case model.StatusArchived, model.StatusCancelled, model.StatusDone:
			// closed; ignored for forward-looking health
		default:
			active = append(active, it)
		}
	}
	if len(active) == 0 {
		if len(items) == 0 {
			return "unknown", []string{"no items yet"}
		}
		return "on_track", []string{"all items are done, cancelled, or archived"}
	}

	var reasons []string
	blocked, ready, inProgress := 0, 0, 0
	missingDep := 0
	oldestReviewDays := -1
	for _, it := range active {
		switch it.Status {
		case model.StatusBlocked:
			blocked++
		case model.StatusReady:
			ready++
			for _, dep := range it.DependsOn {
				d, ok := known[dep]
				if !ok || d.ObjectStatus() != model.StatusDone {
					missingDep++
					break
				}
			}
		case model.StatusInProgress:
			inProgress++
		case model.StatusReview:
			if d := ageDays(it.UpdatedAt, now); d > oldestReviewDays {
				oldestReviewDays = d
			}
		}
	}

	if blocked > 0 {
		reasons = append(reasons, plural(blocked, "blocked item", "blocked items"))
	}
	if missingDep > 0 {
		reasons = append(reasons, plural(missingDep, "ready item with an unmet dependency", "ready items with unmet dependencies"))
	}
	if oldestReviewDays >= staleReviewDays {
		reasons = append(reasons, fmt.Sprintf("oldest review item is %d days old", oldestReviewDays))
	}

	actionable := ready + inProgress
	switch {
	case actionable == 0 && blocked > 0:
		// nothing can move forward and work is blocked
		return "blocked", reasons
	case blocked > 0 || missingDep > 0 || oldestReviewDays >= staleReviewDays:
		return "at_risk", reasons
	case actionable > 0:
		if len(reasons) == 0 {
			reasons = append(reasons, plural(actionable, "actionable item", "actionable items"))
		}
		return "on_track", reasons
	default:
		return "unknown", reasons
	}
}

func ageDays(ts string, now time.Time) int {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		if t, err = time.Parse("2006-01-02", ts); err != nil {
			return -1
		}
	}
	d := int(now.Sub(t).Hours() / 24)
	if d < 0 {
		return 0
	}
	return d
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}
