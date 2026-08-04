package core

import (
	"testing"

	"github.com/moul/workgraph/internal/model"
)

func TestImportIdempotent(t *testing.T) {
	e := newEngine(t)
	specs := []ImportSpec{
		{Title: "A", SourceRef: "github:moul/x#1", Source: "github"},
		{Title: "B", SourceRef: "github:moul/x#2", Source: "github"},
	}
	created, skipped, err := e.Import(specs)
	if err != nil {
		t.Fatal(err)
	}
	if created != 2 || skipped != 0 {
		t.Fatalf("first import: created=%d skipped=%d", created, skipped)
	}
	// Re-import: all skipped.
	created, skipped, err = e.Import(specs)
	if err != nil {
		t.Fatal(err)
	}
	if created != 0 || skipped != 2 {
		t.Fatalf("re-import: created=%d skipped=%d", created, skipped)
	}
	// Imported items default to inbox, carry provenance.
	g, _ := e.WS.Load()
	for _, it := range g.Items {
		if it.Status != model.StatusInbox {
			t.Errorf("imported item %s status = %q, want inbox", it.ID, it.Status)
		}
		if it.SourceRef == "" || it.SourceImportedAt == "" {
			t.Errorf("imported item %s missing provenance", it.ID)
		}
	}
}
