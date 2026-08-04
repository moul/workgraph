// Package frontmatter parses and serializes Markdown documents with a leading
// YAML frontmatter block delimited by "---" lines.
//
// The canonical form is:
//
//	---
//	key: value
//	---
//	# Body markdown...
//
// Frontmatter is kept deliberately flat (scalars and lists of scalars) so that
// Obsidian editing, CLI parsing, and plain text review stay predictable.
package frontmatter

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Doc is a parsed Markdown document: its raw frontmatter mapping plus the body
// text that follows the closing delimiter.
type Doc struct {
	Meta map[string]any
	Body string
}

var delim = []byte("---")

// Parse splits src into frontmatter and body. A document with no leading "---"
// delimiter is treated as all body with empty Meta (still valid). A document
// that opens a frontmatter block but never closes it is an error.
func Parse(src []byte) (*Doc, error) {
	src = bytes.TrimPrefix(src, []byte{0xEF, 0xBB, 0xBF}) // strip UTF-8 BOM
	// Normalize to \n for scanning; preserve for body via index math is fiddly,
	// so we operate on normalized text and re-emit \n bodies.
	norm := bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(norm, append(append([]byte{}, delim...), '\n')) &&
		!bytes.Equal(norm, delim) {
		return &Doc{Meta: map[string]any{}, Body: string(norm)}, nil
	}
	// Find closing delimiter on its own line.
	rest := norm[len(delim):]
	rest = bytes.TrimPrefix(rest, []byte("\n"))
	idx := findClosing(rest)
	if idx < 0 {
		return nil, fmt.Errorf("frontmatter: opening --- has no closing ---")
	}
	yamlPart := rest[:idx]
	body := rest[idx:]
	// Drop the closing delimiter line from body.
	body = dropFirstLine(body)

	meta := map[string]any{}
	if len(bytes.TrimSpace(yamlPart)) > 0 {
		if err := yaml.Unmarshal(yamlPart, &meta); err != nil {
			return nil, fmt.Errorf("frontmatter: invalid YAML: %w", err)
		}
		if meta == nil {
			meta = map[string]any{}
		}
	}
	return &Doc{Meta: meta, Body: strings.TrimLeft(string(body), "\n")}, nil
}

// findClosing returns the byte index in b where a line consisting solely of
// "---" begins, or -1.
func findClosing(b []byte) int {
	offset := 0
	for len(b) > 0 {
		nl := bytes.IndexByte(b, '\n')
		var line []byte
		if nl < 0 {
			line = b
		} else {
			line = b[:nl]
		}
		if bytes.Equal(bytes.TrimRight(line, " \t"), delim) {
			return offset
		}
		if nl < 0 {
			break
		}
		b = b[nl+1:]
		offset += nl + 1
	}
	return -1
}

func dropFirstLine(b []byte) []byte {
	if nl := bytes.IndexByte(b, '\n'); nl >= 0 {
		return b[nl+1:]
	}
	return nil
}

// Marshal serializes meta as YAML frontmatter followed by body. The body is
// emitted verbatim (a trailing newline is ensured). Keys are ordered by the
// yaml encoder's default (map key sort) unless meta is a *yaml.Node; callers
// that need stable field order should serialize via a struct with yaml tags.
func Marshal(meta any, body string) ([]byte, error) {
	var buf bytes.Buffer
	buf.Write(delim)
	buf.WriteByte('\n')
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(meta); err != nil {
		return nil, fmt.Errorf("frontmatter: encode: %w", err)
	}
	_ = enc.Close()
	buf.Write(delim)
	buf.WriteByte('\n')
	if body != "" {
		buf.WriteByte('\n')
		buf.WriteString(body)
		if !strings.HasSuffix(body, "\n") {
			buf.WriteByte('\n')
		}
	}
	return buf.Bytes(), nil
}
