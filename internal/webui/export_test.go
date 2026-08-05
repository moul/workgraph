package webui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moul/workgraph/internal/index"
)

func TestExportStatic(t *testing.T) {
	dir := t.TempDir()
	res := &index.Result{
		Objects: []index.ObjectLine{{ID: "PRJ-1", Type: "project", Status: "active", Title: "P"}},
		Health:  []index.HealthLine{{Project: "PRJ-1", Title: "P", Suggested: "at_risk"}},
	}
	n, err := ExportStatic(res, dir)
	if err != nil {
		t.Fatal(err)
	}
	if n < 3 {
		t.Fatalf("wrote %d files, want >=3", n)
	}
	for _, f := range []string{"index.html", "data/objects.json", "data/health.json", "assets/badge-PRJ-1.svg"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("missing %s", f)
		}
	}
	svg, _ := os.ReadFile(filepath.Join(dir, "assets/badge-PRJ-1.svg"))
	if !strings.Contains(string(svg), "<svg") || !strings.Contains(string(svg), "at_risk") {
		t.Errorf("badge SVG malformed")
	}
}
