package frontmatter

import (
	"strings"
	"testing"
)

func TestParseBasic(t *testing.T) {
	src := "---\nid: ITM-1\ntitle: Hello\ntags:\n  - a\n  - b\n---\n\n# Body\n\ntext\n"
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Meta["id"] != "ITM-1" {
		t.Errorf("id = %v", doc.Meta["id"])
	}
	if doc.Meta["title"] != "Hello" {
		t.Errorf("title = %v", doc.Meta["title"])
	}
	tags, ok := doc.Meta["tags"].([]any)
	if !ok || len(tags) != 2 {
		t.Errorf("tags = %#v", doc.Meta["tags"])
	}
	if !strings.HasPrefix(doc.Body, "# Body") {
		t.Errorf("body = %q", doc.Body)
	}
}

func TestParseNoFrontmatter(t *testing.T) {
	doc, err := Parse([]byte("just body\nmore\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Meta) != 0 {
		t.Errorf("meta = %#v", doc.Meta)
	}
	if doc.Body != "just body\nmore\n" {
		t.Errorf("body = %q", doc.Body)
	}
}

func TestParseUnclosed(t *testing.T) {
	_, err := Parse([]byte("---\nid: x\nno closing\n"))
	if err == nil {
		t.Fatal("expected error for unclosed frontmatter")
	}
}

func TestParseInvalidYAML(t *testing.T) {
	_, err := Parse([]byte("---\nid: : : bad\n\t- weird\n---\nbody\n"))
	if err == nil {
		t.Fatal("expected YAML error")
	}
}

func TestParseCRLF(t *testing.T) {
	src := "---\r\nid: ITM-1\r\n---\r\nbody\r\n"
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Meta["id"] != "ITM-1" {
		t.Errorf("id = %v", doc.Meta["id"])
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	meta := map[string]any{"id": "ITM-1", "title": "Hello"}
	out, err := Marshal(meta, "# Body\n")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Meta["id"] != "ITM-1" || doc.Meta["title"] != "Hello" {
		t.Errorf("round trip meta = %#v", doc.Meta)
	}
	if strings.TrimSpace(doc.Body) != "# Body" {
		t.Errorf("round trip body = %q", doc.Body)
	}
}
