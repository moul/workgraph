// Package gitutil wraps the git CLI for the small set of operations Workgraph
// needs: sync preflight, commit, and push. Git is the audit log and the
// conflict-resolution mechanism; there is no reimplementation of git here.
package gitutil

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Repo is a git working tree rooted at Dir.
type Repo struct {
	Dir string
}

// Open returns a Repo. It does not verify that Dir is a git repository; use
// IsRepo for that.
func Open(dir string) *Repo { return &Repo{Dir: dir} }

func (r *Repo) git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// IsRepo reports whether Dir is inside a git work tree.
func (r *Repo) IsRepo() bool {
	out, err := r.git("rev-parse", "--is-inside-work-tree")
	return err == nil && out == "true"
}

// CurrentBranch returns the current branch name.
func (r *Repo) CurrentBranch() (string, error) {
	return r.git("rev-parse", "--abbrev-ref", "HEAD")
}

// Fetch updates remote-tracking refs. A failure (offline) is returned so the
// caller can decide whether to warn or refuse.
func (r *Repo) Fetch() error {
	_, err := r.git("fetch", "--quiet")
	return err
}

// BehindAhead returns how many commits the local branch is behind and ahead of
// its upstream. If there is no upstream, it returns (0,0,nil).
func (r *Repo) BehindAhead() (behind, ahead int, err error) {
	out, err := r.git("rev-list", "--left-right", "--count", "@{upstream}...HEAD")
	if err != nil {
		// No upstream configured is not fatal.
		if strings.Contains(err.Error(), "no upstream") || strings.Contains(err.Error(), "unknown revision") {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return 0, 0, nil
	}
	fmt.Sscanf(fields[0], "%d", &behind)
	fmt.Sscanf(fields[1], "%d", &ahead)
	return behind, ahead, nil
}

// IsClean reports whether the working tree has no uncommitted changes.
func (r *Repo) IsClean() (bool, error) {
	out, err := r.git("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out == "", nil
}

// Add stages the given paths (repo-relative).
func (r *Repo) Add(paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	_, err := r.git(append([]string{"add", "--"}, paths...)...)
	return err
}

// Commit stages paths and creates a commit. If paths is empty, all staged
// changes are committed. Returns the new commit hash.
func (r *Repo) Commit(message string, paths ...string) (string, error) {
	if err := r.Add(paths...); err != nil {
		return "", err
	}
	if _, err := r.git("commit", "-m", message); err != nil {
		return "", err
	}
	return r.git("rev-parse", "HEAD")
}

// Push pushes the current branch to its upstream (or origin HEAD).
func (r *Repo) Push() error {
	branch, err := r.CurrentBranch()
	if err != nil {
		return err
	}
	_, err = r.git("push", "origin", branch)
	return err
}

// CheckoutNewBranch creates and switches to a new branch.
func (r *Repo) CheckoutNewBranch(name string) error {
	_, err := r.git("checkout", "-b", name)
	return err
}

// Checkout switches to an existing branch.
func (r *Repo) Checkout(name string) error {
	_, err := r.git("checkout", name)
	return err
}

// EnsureBranch checks out an existing branch or creates it if missing, then
// returns to it. It is idempotent.
func (r *Repo) EnsureBranch(name string) error {
	if _, err := r.git("rev-parse", "--verify", "--quiet", "refs/heads/"+name); err == nil {
		return r.Checkout(name)
	}
	return r.CheckoutNewBranch(name)
}

// PushBranch pushes a specific branch to origin, setting upstream.
func (r *Repo) PushBranch(name string) error {
	_, err := r.git("push", "--set-upstream", "origin", name)
	return err
}

// FastForward attempts to fast-forward the current branch to its upstream.
func (r *Repo) FastForward() error {
	_, err := r.git("merge", "--ff-only", "@{upstream}")
	return err
}

// BlobHash returns git's object id for a repo-relative file as staged/working.
func (r *Repo) BlobHash(path string) (string, error) {
	return r.git("hash-object", "--", path)
}
