package webui

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"

	"github.com/moul/workgraph/internal/index"
)

// ExportStatic writes a self-contained static site from a built index into dir:
// the dashboard, the index data as JSON (for other tools), and one SVG health
// badge per project. Everything consumes local files only — no runtime API — so
// CI can publish it to GitHub Pages. Returns the number of files written.
func ExportStatic(res *index.Result, dir string) (int, error) {
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o755); err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		return 0, err
	}
	n := 0
	write := func(rel string, b []byte) error {
		if err := os.WriteFile(filepath.Join(dir, rel), b, 0o644); err != nil {
			return err
		}
		n++
		return nil
	}

	// The dashboard (read-only, no server features).
	if err := write("index.html", []byte(Render(res))); err != nil {
		return n, err
	}
	// Machine-readable data.
	for name, v := range map[string]any{
		"objects.json":   res.Objects,
		"attention.json": res.Attention,
		"runs.json":      res.Runs,
		"health.json":    res.Health,
		"timeline.json":  res.Timeline,
	} {
		b, _ := json.MarshalIndent(v, "", "  ")
		if err := write("data/"+name, b); err != nil {
			return n, err
		}
	}
	// One health badge SVG per project.
	for _, h := range res.Health {
		if err := write("assets/badge-"+h.Project+".svg", healthBadge(h.Title, h.Suggested)); err != nil {
			return n, err
		}
	}
	return n, nil
}

var badgeColor = map[string]string{
	"on_track": "#0a7d33", "at_risk": "#b7791f", "blocked": "#c22", "unknown": "#777",
}

// healthBadge renders a shields.io-style SVG badge: "<label> | <status>".
func healthBadge(label, status string) []byte {
	color := badgeColor[status]
	if color == "" {
		color = "#777"
	}
	label = html.EscapeString(label)
	status = html.EscapeString(status)
	lw := 8*len(label) + 16
	sw := 8*len(status) + 16
	total := lw + sw
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="20" role="img" aria-label="%s: %s">
<rect width="%d" height="20" fill="#444" rx="3"/>
<rect x="%d" width="%d" height="20" fill="%s" rx="3"/>
<rect x="%d" width="4" height="20" fill="%s"/>
<g fill="#fff" font-family="Verdana,DejaVu Sans,sans-serif" font-size="11" text-anchor="middle">
<text x="%d" y="14">%s</text>
<text x="%d" y="14">%s</text>
</g></svg>`,
		total, label, status,
		lw, lw, sw, color, lw, color,
		lw/2, label, lw+sw/2, status)
	return []byte(svg)
}
