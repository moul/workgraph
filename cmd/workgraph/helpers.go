package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"text/tabwriter"

	"github.com/moul/workgraph/internal/core"
	"github.com/moul/workgraph/internal/store"
)

// execGit runs git in dir and returns trimmed stdout.
func execGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return strings.TrimSpace(out.String()), err
}

// globalOpts are the mutation flags every write command accepts.
type globalOpts struct {
	dir              string
	actor            string
	noCommit         bool
	noPush           bool
	offline          bool
	branchOnConflict bool
	expectedVersion  string
}

// addGlobal registers the shared flags on fs.
func addGlobal(fs *flag.FlagSet, o *globalOpts) {
	fs.StringVar(&o.dir, "C", "", "workspace directory (default: search from cwd)")
	fs.StringVar(&o.actor, "actor", defaultActor(), "actor identity for events (e.g. human:moul, agent:claude)")
	fs.BoolVar(&o.noCommit, "no-commit", false, "do not git commit/push the change")
	fs.BoolVar(&o.noPush, "no-push", false, "commit but do not push")
	fs.BoolVar(&o.offline, "offline", false, "skip git fetch/preflight")
	fs.BoolVar(&o.branchOnConflict, "branch-on-conflict", false, "on divergence, write to a conflict branch")
	fs.StringVar(&o.expectedVersion, "expected-version", "", "optimistic concurrency: required current object version")
}

func defaultActor() string {
	if a := os.Getenv("WORKGRAPH_ACTOR"); a != "" {
		return a
	}
	if u, err := user.Current(); err == nil && u.Username != "" {
		return "human:" + u.Username
	}
	return "human:unknown"
}

// openWS finds and opens the workspace.
func openWS(o *globalOpts) (*store.Workspace, error) {
	dir := o.dir
	if dir == "" {
		dir = os.Getenv("WORKGRAPH_DIR")
	}
	if dir == "" {
		dir = "."
	}
	root, err := store.FindRoot(dir)
	if err != nil {
		return nil, err
	}
	return store.Open(root)
}

// engine opens the workspace and builds a core.Engine from the global flags.
func engine(o *globalOpts) (*core.Engine, error) {
	ws, err := openWS(o)
	if err != nil {
		return nil, err
	}
	return core.New(ws, core.Options{
		Actor:            o.actor,
		NoCommit:         o.noCommit,
		NoPush:           o.noPush,
		Offline:          o.offline,
		BranchOnConflict: o.branchOnConflict,
	}), nil
}

// parseFlags parses args allowing flags and positionals to be interspersed
// (Go's flag package stops at the first positional otherwise). It reorders
// tokens so all flags precede positionals, consulting fs to tell boolean flags
// (which take no value) from value flags.
func parseFlags(fs *flag.FlagSet, args []string) error {
	var flags, positional []string
	i := 0
	for i < len(args) {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if len(a) > 1 && a[0] == '-' {
			name := strings.TrimLeft(a, "-")
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				flags = append(flags, a)
				i++
				continue
			}
			flags = append(flags, a)
			if f := fs.Lookup(name); f != nil {
				if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && bf.IsBoolFlag() {
					i++
					continue
				}
				// value flag: consume next token as its value
				if i+1 < len(args) {
					flags = append(flags, args[i+1])
					i += 2
					continue
				}
			}
			i++
			continue
		}
		positional = append(positional, a)
		i++
	}
	return fs.Parse(append(flags, positional...))
}

// noteConflict prints a notice when the engine wrote to a non-default branch —
// loudly for a divergence conflict, quietly for opt-in branch mode — so the
// branch is never silent.
func noteConflict(e *core.Engine) {
	if b := e.ConflictBranch(); b != "" {
		fmt.Fprintf(os.Stderr, "note: control branch had diverged; changes written to conflict branch %q — reconcile before marking work done\n", b)
		return
	}
	if b := e.WorkBranch(); b != "" {
		fmt.Fprintf(os.Stderr, "note: changes committed to branch %q (mutation_policy=branch)\n", b)
	}
}

// printJSON writes v as indented JSON to stdout.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// newTab returns a tabwriter to stdout.
func newTab() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
}

// firstNonFlag returns the first positional argument, or "".
func firstArg(args []string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			return a
		}
	}
	return ""
}

// requireArg returns the positional arg at index i (post-parse) or an error.
func requireArg(fs *flag.FlagSet, i int, name string) (string, error) {
	if fs.NArg() <= i {
		return "", fmt.Errorf("missing required argument: %s", name)
	}
	return fs.Arg(i), nil
}
