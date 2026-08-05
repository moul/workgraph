package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moul/workgraph/internal/store"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s (in %s): %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeConfig(t *testing.T, dir string) {
	t.Helper()
	cfg := store.DefaultConfig()
	cfg.WorkspaceID = "WKG-CONFLICT"
	if err := cfg.Save(dir); err != nil {
		t.Fatal(err)
	}
}

// TestBranchOnConflict simulates two clones diverging against one origin and
// asserts a mutation with BranchOnConflict lands on a conflict branch instead
// of refusing or overwriting.
func TestBranchOnConflict(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	git(t, base, "init", "--bare", "--quiet", origin)

	// Clone A, initialize the workspace, push main.
	aDir := filepath.Join(base, "A")
	git(t, base, "clone", "--quiet", origin, aDir)
	git(t, aDir, "checkout", "-B", "main")
	git(t, aDir, "config", "user.email", "a@a.a")
	git(t, aDir, "config", "user.name", "a")
	writeConfig(t, aDir)
	git(t, aDir, "add", "-A")
	git(t, aDir, "commit", "--quiet", "-m", "init workspace")
	git(t, aDir, "push", "--quiet", "-u", "origin", "HEAD:main")

	// Clone B from origin (before A advances).
	bDir := filepath.Join(base, "B")
	git(t, base, "clone", "--quiet", origin, bDir)
	git(t, bDir, "config", "user.email", "b@b.b")
	git(t, bDir, "config", "user.name", "b")
	git(t, bDir, "checkout", "-B", "main", "origin/main")

	// B makes a local commit -> diverges.
	if err := os.WriteFile(filepath.Join(bDir, "local.txt"), []byte("b-only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, bDir, "add", "-A")
	git(t, bDir, "commit", "--quiet", "-m", "b local change")

	// A advances origin.
	aws, _ := store.Open(aDir)
	ae := New(aws, Options{Actor: "human:a"})
	if _, err := ae.CreateProject("From A", "", ""); err != nil {
		t.Fatalf("A CreateProject: %v", err)
	}

	// B mutates with BranchOnConflict: origin advanced + B has a local commit ->
	// fast-forward fails -> conflict branch.
	bws, _ := store.Open(bDir)
	be := New(bws, Options{Actor: "human:b", BranchOnConflict: true})
	if _, err := be.CreateProject("From B", "", ""); err != nil {
		t.Fatalf("B CreateProject: %v", err)
	}

	branch := be.ConflictBranch()
	if branch == "" {
		t.Fatal("expected a conflict branch to be set")
	}
	if !strings.HasPrefix(branch, "workgraph/conflict/") {
		t.Errorf("conflict branch = %q, want workgraph/conflict/ prefix", branch)
	}
	// The conflict branch must exist on origin, and main must be untouched by B.
	remoteBranches := git(t, bDir, "ls-remote", "--heads", "origin")
	if !strings.Contains(remoteBranches, branch) {
		t.Errorf("conflict branch %q not pushed to origin:\n%s", branch, remoteBranches)
	}
	cur := git(t, bDir, "rev-parse", "--abbrev-ref", "HEAD")
	if cur != branch {
		t.Errorf("B is on %q, want the conflict branch %q", cur, branch)
	}
}

// TestRefuseWithoutBranchFlag asserts the default remains a hard refusal.
func TestRefuseWithoutBranchFlag(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	git(t, base, "init", "--bare", "--quiet", origin)
	aDir := filepath.Join(base, "A")
	git(t, base, "clone", "--quiet", origin, aDir)
	git(t, aDir, "checkout", "-B", "main")
	git(t, aDir, "config", "user.email", "a@a.a")
	git(t, aDir, "config", "user.name", "a")
	writeConfig(t, aDir)
	git(t, aDir, "add", "-A")
	git(t, aDir, "commit", "--quiet", "-m", "init")
	git(t, aDir, "push", "--quiet", "-u", "origin", "HEAD:main")

	bDir := filepath.Join(base, "B")
	git(t, base, "clone", "--quiet", origin, bDir)
	git(t, bDir, "config", "user.email", "b@b.b")
	git(t, bDir, "config", "user.name", "b")
	git(t, bDir, "checkout", "-B", "main", "origin/main")
	_ = os.WriteFile(filepath.Join(bDir, "local.txt"), []byte("x\n"), 0o644)
	git(t, bDir, "add", "-A")
	git(t, bDir, "commit", "--quiet", "-m", "b local")

	aws, _ := store.Open(aDir)
	if _, err := New(aws, Options{Actor: "a"}).CreateProject("A2", "", ""); err != nil {
		t.Fatal(err)
	}

	bws, _ := store.Open(bDir)
	_, err := New(bws, Options{Actor: "b"}).CreateProject("B2", "", "")
	if err == nil {
		t.Fatal("expected refusal without --branch-on-conflict")
	}
	if !strings.Contains(err.Error(), "branch-on-conflict") {
		t.Errorf("refusal message should suggest --branch-on-conflict, got: %v", err)
	}
}
