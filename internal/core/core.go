// Package core is the single mutation path shared by the CLI, the MCP server,
// and the HTTP gateway. There is no UI-only, MCP-only, or CLI-only write path:
// every mutation writes an object and an event, rebuilds committed indexes, and
// (unless disabled) commits — and pushes — in one coherent step.
package core

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/moul/workgraph/internal/eventlog"
	"github.com/moul/workgraph/internal/gitutil"
	"github.com/moul/workgraph/internal/id"
	"github.com/moul/workgraph/internal/index"
	"github.com/moul/workgraph/internal/model"
	"github.com/moul/workgraph/internal/store"
)

// Options controls how a mutation touches git.
type Options struct {
	Actor            string // e.g. "human:moul" or "agent:claude"
	NoCommit         bool   // skip git commit/push (tests, manual repair)
	NoPush           bool   // commit but do not push
	Offline          bool   // skip fetch/preflight; mark needs_sync semantics
	BranchOnConflict bool   // on non-fast-forward, write to a conflict branch
}

// Engine performs mutations against a workspace.
type Engine struct {
	WS   *store.Workspace
	Repo *gitutil.Repo
	Opt  Options

	// conflictBranch, once set by preflight, redirects every subsequent commit
	// in this engine's lifetime onto one conflict branch instead of the default
	// branch. It is stable for the whole command so nested mutations land
	// together, never producing two active owners on the default branch.
	conflictBranch string
	// branchHint is the per-object work branch used under mutation_policy=branch.
	branchHint string
}

// targetBranch returns the non-default branch commits should land on: the
// conflict branch takes precedence over a branch-policy work branch.
func (e *Engine) targetBranch() string {
	if e.conflictBranch != "" {
		return e.conflictBranch
	}
	return e.branchHint
}

// New returns an Engine for the workspace at ws with the given options.
func New(ws *store.Workspace, opt Options) *Engine {
	if opt.Actor == "" {
		opt.Actor = "human:unknown"
	}
	return &Engine{WS: ws, Repo: gitutil.Open(ws.Root), Opt: opt}
}

// now returns an RFC3339 timestamp.
func now() string { return time.Now().Format(time.RFC3339) }

// preflight runs the git sync check before a mutation. Read paths call this
// with mutating=false. It never aborts on offline; it returns a warning string.
func (e *Engine) preflight(mutating bool) (warn string, err error) {
	if e.Opt.Offline || e.Opt.NoCommit || !e.Repo.IsRepo() {
		return "", nil
	}
	if ferr := e.Repo.Fetch(); ferr != nil {
		return "fetch failed (working offline): " + ferr.Error(), nil
	}
	behind, _, berr := e.Repo.BehindAhead()
	if berr != nil {
		return "", nil
	}
	if behind == 0 {
		return "", nil
	}
	if !mutating {
		return fmt.Sprintf("control branch is %d commit(s) behind upstream", behind), nil
	}
	// Mutating: try to fast-forward.
	if ffErr := e.Repo.FastForward(); ffErr != nil {
		if e.Opt.BranchOnConflict {
			if e.conflictBranch == "" {
				e.conflictBranch = e.WS.Config.BranchPrefix + "conflict/" + shortConflictID()
			}
			return "diverged; writing to conflict branch " + e.conflictBranch, nil
		}
		return "", fmt.Errorf("control branch is %d commit(s) behind and cannot fast-forward; re-run with --branch-on-conflict or reconcile manually", behind)
	}
	return "", nil
}

// shortConflictID returns a short, sortable id fragment for a conflict branch.
func shortConflictID() string {
	u := id.ULID(id.NewEvent())
	if len(u) >= 12 {
		return strings.ToLower(u[:12])
	}
	return strings.ToLower(u)
}

// commit rebuilds committed indexes, then commits (and pushes) the given source
// paths plus the indexes and events, using message. Returns the commit hash or
// "" when NoCommit.
func (e *Engine) commit(message string, paths ...string) (string, error) {
	// Rebuild committed indexes so they never drift from source.
	if e.WS.Config.IndexPolicy == "committed" {
		if _, err := index.Build(e.WS, true); err != nil {
			return "", err
		}
	}
	if e.Opt.NoCommit || !e.Repo.IsRepo() {
		return "", nil
	}
	// In branch mutation mode, derive a stable per-object work branch from the
	// first source path (once per engine, so nested commits land together).
	if e.conflictBranch == "" && e.branchHint == "" && e.WS.Config.MutationPolicy == "branch" {
		if b := deriveBranchName(e.WS.Config.BranchPrefix, paths); b != "" {
			e.branchHint = b
		}
	}
	// Redirect commits onto the active non-default branch, if any.
	if tb := e.targetBranch(); tb != "" {
		if berr := e.Repo.EnsureBranch(tb); berr != nil {
			return "", fmt.Errorf("core: cannot switch to branch %s: %w", tb, berr)
		}
	}
	// Stage source paths, events, and indexes.
	all := append([]string{}, paths...)
	all = append(all, "events", "indexes")
	hash, err := e.Repo.Commit(message, all...)
	if err != nil {
		return "", err
	}
	if !e.Opt.NoPush && !e.Opt.Offline {
		if perr := e.push(); perr != nil {
			// A push failure is surfaced but does not undo the local commit.
			return hash, fmt.Errorf("committed %s but push failed: %w", hash[:min(7, len(hash))], perr)
		}
	}
	return hash, nil
}

// push sends the current work to origin — the active non-default branch when
// one is set, otherwise the default branch.
func (e *Engine) push() error {
	if tb := e.targetBranch(); tb != "" {
		return e.Repo.PushBranch(tb)
	}
	return e.Repo.Push()
}

// ConflictBranch returns the conflict branch this engine wrote to, or "" if it
// stayed on the default branch. Callers surface this to the user.
func (e *Engine) ConflictBranch() string { return e.conflictBranch }

// WorkBranch returns the non-default branch this engine wrote to (conflict or
// branch-policy), or "" if it committed on the default branch.
func (e *Engine) WorkBranch() string { return e.targetBranch() }

// deriveBranchName builds a deterministic work-branch name from the first
// source path being committed. For an item file it yields
// "<prefix><ITM-...-slug>"; for a PROJECT.md it uses the project directory name.
func deriveBranchName(prefix string, paths []string) string {
	for _, p := range paths {
		if p == "" || p == "events" || p == "indexes" {
			continue
		}
		base := strings.TrimSuffix(path.Base(p), ".md")
		if base == "PROJECT" {
			base = path.Base(path.Dir(p))
		}
		if base == "" {
			continue
		}
		return prefix + base
	}
	return ""
}

// appendEvent writes an event with defaults filled in from the engine actor.
func (e *Engine) appendEvent(ev eventlog.Event) error {
	if ev.Actor == "" {
		ev.Actor = e.Opt.Actor
	}
	if ev.At == "" {
		ev.At = now()
	}
	return eventlog.Append(e.WS.Root, ev)
}

// resolveItem loads the graph and resolves ref to an item.
func (e *Engine) resolveItem(ref string) (*model.Item, *store.Graph, error) {
	g, err := e.WS.Load()
	if err != nil {
		return nil, nil, err
	}
	o, err := g.Resolve(ref)
	if err != nil {
		return nil, nil, err
	}
	it, ok := o.(*model.Item)
	if !ok {
		return nil, nil, fmt.Errorf("core: %s is a %s, not an item", o.ObjectID(), o.ObjectType())
	}
	return it, g, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
