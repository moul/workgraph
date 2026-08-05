package core

import (
	"path/filepath"
	"testing"
)

func TestShouldRedact(t *testing.T) {
	e := newEngine(t) // e.WS.Root is a temp control repo
	inside := filepath.Join(e.WS.Root, "sub", "target")
	outside := t.TempDir()

	cases := []struct {
		mode, target string
		want         bool
	}{
		{"on", inside, true},
		{"on", outside, true},
		{"off", inside, false},
		{"off", outside, false},
		{"auto", inside, false}, // target inside the control repo -> no redact
		{"auto", outside, true}, // target outside -> redact by default
		{"", outside, true},     // empty mode == auto
	}
	for _, c := range cases {
		if got := e.shouldRedact(c.mode, c.target); got != c.want {
			t.Errorf("shouldRedact(%q, outside=%v) = %v, want %v", c.mode, c.target == outside, got, c.want)
		}
	}
}
