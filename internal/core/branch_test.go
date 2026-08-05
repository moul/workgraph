package core

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moul/workgraph/internal/store"
)

// TestBranchMutationPolicy asserts that under mutation_policy=branch a mutation
// lands on a derived work branch instead of the default branch.
func TestBranchMutationPolicy(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	git(t, base, "init", "--bare", "--quiet", origin)

	wsDir := filepath.Join(base, "ws")
	git(t, base, "clone", "--quiet", origin, wsDir)
	git(t, wsDir, "checkout", "-B", "main")
	git(t, wsDir, "config", "user.email", "w@w.w")
	git(t, wsDir, "config", "user.name", "w")

	cfg := store.DefaultConfig()
	cfg.WorkspaceID = "WKG-BRANCH"
	cfg.MutationPolicy = "branch"
	cfg.BranchPrefix = "workgraph/"
	if err := cfg.Save(wsDir); err != nil {
		t.Fatal(err)
	}
	git(t, wsDir, "add", "-A")
	git(t, wsDir, "commit", "--quiet", "-m", "init")
	git(t, wsDir, "push", "--quiet", "-u", "origin", "HEAD:main")

	ws, _ := store.Open(wsDir)
	e := New(ws, Options{Actor: "human:w"})
	it, err := e.CreateItem("Add run summary", "", "task", true)
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	wb := e.WorkBranch()
	if wb == "" || !strings.HasPrefix(wb, "workgraph/") {
		t.Fatalf("work branch = %q, want workgraph/ prefix", wb)
	}
	if !strings.Contains(wb, it.ID) {
		t.Errorf("work branch %q should contain item id %q", wb, it.ID)
	}
	// The branch must exist on origin; main must not carry the item.
	remotes := git(t, wsDir, "ls-remote", "--heads", "origin")
	if !strings.Contains(remotes, wb) {
		t.Errorf("work branch %q not pushed to origin:\n%s", wb, remotes)
	}
	// Default branch (main) should not contain the new item file.
	filesOnMain := git(t, wsDir, "ls-tree", "-r", "--name-only", "origin/main")
	if strings.Contains(filesOnMain, it.ID) {
		t.Errorf("item %q leaked onto main; branch mode should isolate it", it.ID)
	}
}
