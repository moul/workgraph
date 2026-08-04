package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moul/workgraph/internal/model"
	"github.com/moul/workgraph/internal/store"
)

func newWS(t *testing.T) *store.Workspace {
	t.Helper()
	root := t.TempDir()
	cfg := store.DefaultConfig()
	cfg.WorkspaceID = "WKG-T"
	cfg.IndexPolicy = "ignored" // skip staleness in unit tests unless asserted
	if err := cfg.Save(root); err != nil {
		t.Fatal(err)
	}
	ws, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return ws
}

func write(t *testing.T, ws *store.Workspace, rel, content string) {
	t.Helper()
	p := filepath.Join(ws.Root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasFinding(r *Report, sev Severity, substr string) bool {
	for _, f := range r.Findings {
		if f.Severity == sev && strings.Contains(f.Message, substr) {
			return true
		}
	}
	return false
}

func TestValidateClean(t *testing.T) {
	ws := newWS(t)
	proj := &model.Project{Common: model.Common{ID: "PRJ-01K4A2D9Q9N7H2EA2A0P5X0T6A", Type: "project", Title: "Hermes", Status: "active", CreatedAt: "2026-08-04T21:00:00Z", UpdatedAt: "2026-08-04T21:00:00Z"}}
	if _, err := ws.Save(proj); err != nil {
		t.Fatal(err)
	}
	it := &model.Item{Common: model.Common{ID: "ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F", Type: "item", Title: "T", Status: "ready", CreatedAt: "2026-08-04T21:00:00Z", UpdatedAt: "2026-08-04T21:00:00Z"}, Kind: "task", Project: proj.ID}
	if _, err := ws.Save(it); err != nil {
		t.Fatal(err)
	}
	r, err := Run(ws)
	if err != nil {
		t.Fatal(err)
	}
	if r.Errors() != 0 {
		t.Fatalf("expected clean, got: %v", r.Findings)
	}
}

func TestValidateBrokenReference(t *testing.T) {
	ws := newWS(t)
	write(t, ws, "projects/hermes/PROJECT.md", "---\nid: PRJ-01K4A2D9Q9N7H2EA2A0P5X0T6A\ntype: project\ntitle: Hermes\nstatus: active\ncreated_at: 2026-08-04T21:00:00Z\nupdated_at: 2026-08-04T21:00:00Z\n---\n")
	write(t, ws, "projects/hermes/items/ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F-x.md", "---\nid: ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F\ntype: item\ntitle: X\nstatus: ready\nproject: PRJ-01K4A2D9Q9N7H2EA2A0P5X0T6A\ndepends_on:\n  - ITM-01K4A2D9Q9N7H2EA2A0P5XBAD0\ncreated_at: 2026-08-04T21:00:00Z\nupdated_at: 2026-08-04T21:00:00Z\n---\n")
	r, _ := Run(ws)
	if !hasFinding(r, Error, "unknown object") {
		t.Errorf("expected broken reference error, got %v", r.Findings)
	}
}

func TestValidateDuplicateID(t *testing.T) {
	ws := newWS(t)
	body := "---\nid: ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F\ntype: item\ntitle: X\nstatus: ready\ncreated_at: 2026-08-04T21:00:00Z\nupdated_at: 2026-08-04T21:00:00Z\n---\n"
	write(t, ws, "inbox/ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F-a.md", body)
	write(t, ws, "inbox/ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F-b.md", body)
	r, _ := Run(ws)
	if !hasFinding(r, Error, "duplicate id") {
		t.Errorf("expected duplicate id error, got %v", r.Findings)
	}
}

func TestValidateUnknownStatus(t *testing.T) {
	ws := newWS(t)
	write(t, ws, "inbox/ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F-a.md", "---\nid: ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F\ntype: item\ntitle: X\nstatus: planned\ncreated_at: 2026-08-04T21:00:00Z\nupdated_at: 2026-08-04T21:00:00Z\n---\n")
	r, _ := Run(ws)
	if !hasFinding(r, Error, "unknown item status") {
		t.Errorf("expected unknown status error, got %v", r.Findings)
	}
}

func TestValidateMalformedYAML(t *testing.T) {
	ws := newWS(t)
	write(t, ws, "inbox/ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F-a.md", "---\nid: ITM-1\n : : bad\n\t- weird\n---\nbody\n")
	r, _ := Run(ws)
	if !hasFinding(r, Error, "malformed frontmatter") {
		t.Errorf("expected malformed frontmatter, got %v", r.Findings)
	}
}

func TestValidateFilenameMismatch(t *testing.T) {
	ws := newWS(t)
	write(t, ws, "inbox/wrong-name.md", "---\nid: ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F\ntype: item\ntitle: X\nstatus: ready\ncreated_at: 2026-08-04T21:00:00Z\nupdated_at: 2026-08-04T21:00:00Z\n---\n")
	r, _ := Run(ws)
	if !hasFinding(r, Error, "does not start with id") {
		t.Errorf("expected filename mismatch, got %v", r.Findings)
	}
}

func TestValidateDependencyCycle(t *testing.T) {
	ws := newWS(t)
	a := "ITM-01K4A2D9Q9N7H2EA2A0P5X0T6A"
	b := "ITM-01K4A2D9Q9N7H2EA2A0P5X0T6B"
	write(t, ws, "inbox/"+a+"-a.md", "---\nid: "+a+"\ntype: item\ntitle: A\nstatus: ready\ndepends_on:\n  - "+b+"\ncreated_at: 2026-08-04T21:00:00Z\nupdated_at: 2026-08-04T21:00:00Z\n---\n")
	write(t, ws, "inbox/"+b+"-b.md", "---\nid: "+b+"\ntype: item\ntitle: B\nstatus: ready\ndepends_on:\n  - "+a+"\ncreated_at: 2026-08-04T21:00:00Z\nupdated_at: 2026-08-04T21:00:00Z\n---\n")
	r, _ := Run(ws)
	if !hasFinding(r, Error, "dependency cycle") {
		t.Errorf("expected dependency cycle, got %v", r.Findings)
	}
}

func TestValidateSecretScan(t *testing.T) {
	ws := newWS(t)
	write(t, ws, "inbox/ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F-a.md", "---\nid: ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F\ntype: item\ntitle: X\nstatus: ready\ncreated_at: 2026-08-04T21:00:00Z\nupdated_at: 2026-08-04T21:00:00Z\n---\ntoken AKIAIOSFODNN7EXAMPLE here\n")
	r, _ := Run(ws)
	if !hasFinding(r, Warning, "AWS access key") {
		t.Errorf("expected secret scan warning, got %v", r.Findings)
	}
}
