package webui

import (
	"strings"
	"testing"

	"github.com/moul/workgraph/internal/index"
)

// TestRenderEmptyWorkspaceEmbedsArrays guards the fix from #21: a workspace with
// no attention/runs must embed empty JS arrays, never `null` (which would make
// `ATT.length` throw and blank the dashboard).
func TestRenderEmptyWorkspaceEmbedsArrays(t *testing.T) {
	html := Render(&index.Result{}) // all slices nil

	for _, want := range []string{"OBJS=[]", "ATT=[]", "RUNS=[]", "HEALTH=[]"} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered HTML missing %q", want)
		}
	}
	if strings.Contains(html, "=null") {
		t.Errorf("rendered HTML embeds a null slice; nil slices must coerce to []")
	}
}

func TestRenderPopulated(t *testing.T) {
	res := &index.Result{
		Objects:   []index.ObjectLine{{ID: "ITM-1", Type: "item", Status: "ready", Title: "T"}},
		Attention: []index.AttentionLine{{ID: "ITM-1", Reason: "review_assigned_to_human", Severity: "high", Summary: "s"}},
		Runs:      []index.RunLine{{Run: "RUN-1", Item: "ITM-1", Round: 1, Status: "review"}},
	}
	html := Render(res)
	if !strings.Contains(html, "ITM-1") || !strings.Contains(html, "review_assigned_to_human") {
		t.Errorf("populated render missing data")
	}
}
