package model

import (
	"testing"

	"github.com/moul/workgraph/internal/frontmatter"
)

func TestDecodeItem(t *testing.T) {
	src := []byte("---\nid: ITM-1\ntype: item\nkind: task\ntitle: Add run summary\nproject: PRJ-1\nstatus: ready\npriority: high\ndepends_on:\n  - ITM-2\ntags:\n  - hermes\ncustom_field: keepme\ncreated_at: 2026-08-04T21:00:00+02:00\nupdated_at: 2026-08-04T21:00:00+02:00\n---\n# Body\n")
	doc, err := frontmatter.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	var it Item
	if err := Decode(doc.Meta, &it); err != nil {
		t.Fatal(err)
	}
	if it.ID != "ITM-1" || it.Kind != "task" || it.Status != "ready" {
		t.Fatalf("decoded = %+v", it)
	}
	if len(it.DependsOn) != 1 || it.DependsOn[0] != "ITM-2" {
		t.Errorf("depends_on = %v", it.DependsOn)
	}
	if it.Extra["custom_field"] != "keepme" {
		t.Errorf("extra did not preserve unknown key: %v", it.Extra)
	}
}

func TestOntologyDefaults(t *testing.T) {
	o := DefaultOntology()
	o.index()
	cases := []struct {
		vocab, val string
		want       bool
	}{
		{"item_status", "ready", true},
		{"item_status", "planned", false}, // deliberately excluded from v0.1
		{"event_action", "run.finished", true},
		{"event_action", "run.exploded", false},
		{"relation", "depends_on", true},
		{"relation", "related_to", false}, // no generic relation
		{"token_scope", "runs:context", true},
		{"nope", "x", false},
	}
	for _, c := range cases {
		if got := o.Has(c.vocab, c.val); got != c.want {
			t.Errorf("Has(%q,%q) = %v, want %v", c.vocab, c.val, got, c.want)
		}
	}
}
