package id

import "testing"

func TestNewAndValid(t *testing.T) {
	got := NewItem()
	if !Valid(got) {
		t.Fatalf("NewItem() = %q, not valid", got)
	}
	if Prefix(got) != PrefixItem {
		t.Errorf("Prefix(%q) = %q, want %q", got, Prefix(got), PrefixItem)
	}
	if len(ULID(got)) != 26 {
		t.Errorf("ULID(%q) len = %d, want 26", got, len(ULID(got)))
	}
}

func TestUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		v := NewItem()
		if seen[v] {
			t.Fatalf("duplicate id generated: %q", v)
		}
		seen[v] = true
	}
}

func TestValid(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F", true},
		{"PRJ-01K4A2D9Q9N7H2EA2A0P5X0T6F", true},
		{"worker:codex", true},
		{"worker:", false},
		{"ITM-tooshort", false},
		{"lowercase-01K4A2D9Q9N7H2EA2A0P5X0T6F", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := Valid(tt.in); got != tt.want {
			t.Errorf("Valid(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestSlugify(t *testing.T) {
	tests := map[string]string{
		"Add run summary":      "add-run-summary",
		"  Trim  Me  ":         "trim-me",
		"Weird!!!chars@@@here": "weird-chars-here",
		"UPPER":                "upper",
		"--already--slugged--": "already--slugged", // internal double kept? no
	}
	// Adjust expectation for internal collapsing.
	tests["--already--slugged--"] = "already-slugged"
	for in, want := range tests {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFilenameRoundTrip(t *testing.T) {
	oid := NewItem()
	name := Filename(oid, "Add run summary")
	if got := IDFromFilename(name); got != oid {
		t.Errorf("IDFromFilename(%q) = %q, want %q", name, got, oid)
	}
	// No slug.
	name2 := Filename(oid, "")
	if got := IDFromFilename(name2); got != oid {
		t.Errorf("IDFromFilename(%q) = %q, want %q", name2, got, oid)
	}
}
