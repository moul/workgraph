package validate

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/moul/workgraph/internal/store"
)

// repoRoot walks up from this test file to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	// internal/validate/fixtures_test.go -> module root is two dirs up + internal
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func TestBrokenFixtures(t *testing.T) {
	root := filepath.Join(repoRoot(t), "testdata", "broken")
	ws, err := store.Open(root)
	if err != nil {
		t.Fatalf("open broken workspace: %v", err)
	}
	rep, err := Run(ws)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Errors() == 0 {
		t.Fatalf("expected the broken fixtures to produce errors, got none")
	}
	want := []string{
		"unknown object",      // broken depends_on reference
		"unknown item status", // "planned"
		"does not start with id",
		"malformed frontmatter",
		"dependency cycle",
		"duplicate id",
		"unknown event action",
	}
	for _, w := range want {
		if !hasFinding(rep, Error, w) {
			t.Errorf("expected an error containing %q; findings:\n%s", w, dump(rep))
		}
	}
	// The AWS key in a body must be flagged (warning).
	if !hasFinding(rep, Warning, "AWS access key") {
		t.Errorf("expected the secret scan to flag the AWS key")
	}
}

func TestExampleBasicIsClean(t *testing.T) {
	root := filepath.Join(repoRoot(t), "examples", "basic")
	ws, err := store.Open(root)
	if err != nil {
		t.Fatalf("open example workspace: %v", err)
	}
	rep, err := Run(ws)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Errors() != 0 {
		t.Fatalf("examples/basic must validate clean, got:\n%s", dump(rep))
	}
}

func dump(r *Report) string {
	s := ""
	for _, f := range r.Findings {
		s += f.String() + "\n"
	}
	return s
}
