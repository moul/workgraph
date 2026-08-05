package index

import (
	"testing"

	"github.com/moul/workgraph/internal/model"
)

// TestIsPlaceholder guards the fix from #22: scaffold filler must not leak into
// summaries.
func TestIsPlaceholder(t *testing.T) {
	cases := map[string]bool{
		"<!-- What outcome does this produce? -->": true,
		"_Describe the project._":                  true,
		"*also emphasis*":                          true,
		"-":                                        true,
		"*":                                        true,
		"Real content here.":                       false,
		"## Goal":                                  false,
		"- a real bullet":                          false,
	}
	for in, want := range cases {
		if got := isPlaceholder(in); got != want {
			t.Errorf("isPlaceholder(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestSummaryIgnoresScaffold verifies a freshly-scaffolded object (whose only
// body content is a placeholder comment) gets an empty summary rather than the
// scaffold text.
func TestSummaryIgnoresScaffold(t *testing.T) {
	it := &model.Item{Common: model.Common{ID: "ITM-1", Type: "item", Title: "T", Status: "ready"}}
	it.SetBody("# T\n\n## Goal\n\n<!-- What outcome does this produce? -->\n\n## Context\n")
	if s := summarize(it); s != "" {
		t.Errorf("summary = %q, want empty (scaffold placeholder must not leak)", s)
	}

	// Real content under Goal is still summarized.
	it.SetBody("# T\n\n## Goal\n\nShip the parser.\n")
	if s := summarize(it); s != "Ship the parser." {
		t.Errorf("summary = %q, want %q", s, "Ship the parser.")
	}
}
