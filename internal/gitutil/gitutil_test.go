package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitInit(t *testing.T) *Repo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	r := Open(dir)
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"config", "user.email", "t@t.t"},
		{"config", "user.name", "t"},
	} {
		if _, err := r.git(args...); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

func TestCommitAndClean(t *testing.T) {
	r := gitInit(t)
	if ok, _ := r.IsClean(); !ok {
		t.Error("fresh repo should be clean")
	}
	if err := os.WriteFile(filepath.Join(r.Dir, "a.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, _ := r.IsClean(); ok {
		t.Error("repo with new file should be dirty")
	}
	hash, err := r.Commit("add a", "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(hash) < 7 {
		t.Errorf("bad commit hash %q", hash)
	}
	if ok, _ := r.IsClean(); !ok {
		t.Error("repo should be clean after commit")
	}
}

func TestBlobHashMatchesLibrary(t *testing.T) {
	r := gitInit(t)
	if err := os.WriteFile(filepath.Join(r.Dir, "h.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := r.BlobHash("h.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got != "b6fc4c620b67d95f953a5c1c1230aaab5db5a1b0" {
		t.Errorf("git hash-object = %q", got)
	}
}

func TestIsRepo(t *testing.T) {
	r := gitInit(t)
	if !r.IsRepo() {
		t.Error("should be a repo")
	}
	if Open(t.TempDir()).IsRepo() {
		t.Error("empty temp dir should not be a repo")
	}
}
