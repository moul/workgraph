package main

import "testing"

func TestMakeTargets(t *testing.T) {
	mk := []byte(".PHONY: test\ntest:\n\tgo test ./...\n\nbuild: deps\n\tgo build\n\ndeps:\n\tgo mod tidy\n")
	got := makeTargets(mk)
	want := map[string]bool{"test": true, "build": true, "deps": true}
	if len(got) != 3 {
		t.Fatalf("got %v, want 3 targets", got)
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("unexpected target %q", g)
		}
		if g == "PHONY" {
			t.Errorf(".PHONY must be excluded")
		}
	}
}
