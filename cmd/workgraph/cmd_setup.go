package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/moul/workgraph/internal/gitutil"
	"github.com/moul/workgraph/internal/id"
	"github.com/moul/workgraph/internal/index"
	"github.com/moul/workgraph/internal/model"
	"github.com/moul/workgraph/internal/store"
	"github.com/moul/workgraph/internal/validate"
)

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	name := fs.String("workspace", "", "workspace name")
	noGit := fs.Bool("no-git", false, "do not run git init")
	_ = parseFlags(fs, args)

	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, store.ConfigFile)); err == nil {
		return fmt.Errorf("workspace already initialized at %s", dir)
	}

	// workgraph.yaml
	cfg := store.DefaultConfig()
	cfg.WorkspaceID = id.NewWorkspace()
	cfg.Name = *name
	cfg.CreatedBy = defaultActor()
	cfg.CreatedAt = time.Now().Format(time.RFC3339)
	if err := cfg.Save(dir); err != nil {
		return err
	}

	// ontology manifest
	ontDir := filepath.Join(dir, "ontologies")
	if err := os.MkdirAll(ontDir, 0o755); err != nil {
		return err
	}
	ontRaw, _ := yaml.Marshal(model.DefaultOntology())
	if err := os.WriteFile(filepath.Join(ontDir, "workgraph.yaml"), ontRaw, 0o644); err != nil {
		return err
	}

	// scaffold directories
	for _, d := range []string{"projects", "workers", "events", "indexes", "inbox"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			return err
		}
	}
	// keep empty dirs in git
	for _, d := range []string{"projects", "inbox"} {
		_ = os.WriteFile(filepath.Join(dir, d, ".gitkeep"), nil, 0o644)
	}

	// default worker for the current user
	actor := defaultActor()
	slug := actor
	if i := indexByte(actor, ':'); i >= 0 {
		slug = actor[i+1:]
	}
	wk := &model.Worker{
		Common:       model.Common{ID: id.Worker(slug), Type: model.TypeWorker, Title: slug, Status: "active", CreatedAt: cfg.CreatedAt, UpdatedAt: cfg.CreatedAt},
		Kind:         "human",
		Capabilities: []string{"create_item", "update_item", "claim_item", "finish_run", "block_run", "archive_item", "edit_decision"},
	}
	ws, err := store.Open(dir)
	if err != nil {
		return err
	}
	if _, err := ws.Save(wk); err != nil {
		return err
	}

	// Agent guides. We write BOTH files because different agents read different
	// names: Claude Code reads CLAUDE.md, Codex (and the broader convention)
	// reads AGENTS.md. Same content in both so control-repo guidance is picked
	// up regardless of which agent operates it. (This is the *control* repo, not
	// a target repo — Workgraph never mutates a target repo's instruction files.)
	_ = os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(controlRepoAgentGuide), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(controlRepoAgentGuide), 0o644)

	// .gitignore
	_ = os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".workgraph/index.sqlite\n.workgraph/cache/\n.workgraph/gateway.db\n"), 0o644)

	// initial indexes
	if _, err := index.Build(ws, true); err != nil {
		return err
	}

	// git init
	if !*noGit {
		repo := gitutil.Open(dir)
		if !repo.IsRepo() {
			_, _ = execGit(dir, "init", "--quiet")
		}
	}

	fmt.Printf("Initialized your Workgraph control repo at %s\n", dir)
	fmt.Printf("  workspace_id: %s\n\n", cfg.WorkspaceID)
	fmt.Println("This is YOUR private work-state repo, separate from the workgraph tool.")
	fmt.Println("Scaffolded CLAUDE.md + AGENTS.md so any agent operating this repo has the contract.")
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", dir)
	fmt.Println("  git add -A && git commit -m \"init workgraph workspace\"")
	fmt.Println("  gh repo create <you>/workgraph-state --private --source=. --remote=origin --push")
	fmt.Println("  workgraph new project \"My Stuff\" --target-repo git@github.com:<you>/repo.git")
	fmt.Println("\nGuide: https://github.com/moul/workgraph/blob/main/docs/getting-started.md")
	return nil
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	var o globalOpts
	fs.StringVar(&o.dir, "C", "", "workspace directory")
	_ = parseFlags(fs, args)

	fmt.Println("workgraph doctor")
	ws, err := openWS(&o)
	if err != nil {
		fmt.Println("  workspace:      NOT FOUND —", err)
		return nil
	}
	fmt.Printf("  workspace:      %s\n", ws.Root)
	fmt.Printf("  workspace_id:   %s\n", ws.Config.WorkspaceID)
	fmt.Printf("  schema_version: %s\n", ws.Config.SchemaVersion)
	fmt.Printf("  ontology:       %s v%s\n", ws.Ontology.Name, ws.Ontology.Version)

	repo := gitutil.Open(ws.Root)
	if repo.IsRepo() {
		branch, _ := repo.CurrentBranch()
		clean, _ := repo.IsClean()
		fmt.Printf("  git:            repo on %s (clean=%v)\n", branch, clean)
	} else {
		fmt.Println("  git:            not a git repository")
	}

	g, err := ws.Load()
	if err != nil {
		fmt.Println("  load:           ERROR —", err)
		return nil
	}
	fmt.Printf("  objects:        %d projects, %d items, %d decisions, %d workers\n", len(g.Projects), len(g.Items), len(g.Decisions), len(g.Workers))

	rep, _ := validate.Run(ws)
	fmt.Printf("  validate:       %d errors, %d findings\n", rep.Errors(), len(rep.Findings))
	return nil
}

func cmdValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "output findings as JSON")
	var o globalOpts
	fs.StringVar(&o.dir, "C", "", "workspace directory")
	_ = parseFlags(fs, args)

	ws, err := openWS(&o)
	if err != nil {
		return err
	}
	rep, err := validate.Run(ws)
	if err != nil {
		return err
	}
	if *jsonOut {
		type f struct {
			Severity string `json:"severity"`
			Object   string `json:"object"`
			Message  string `json:"message"`
		}
		out := make([]f, 0, len(rep.Findings))
		for _, x := range rep.Findings {
			out = append(out, f{string(x.Severity), x.Object, x.Message})
		}
		if err := printJSON(out); err != nil {
			return err
		}
	} else {
		for _, x := range rep.Findings {
			fmt.Println(x.String())
		}
		fmt.Printf("\n%d error(s), %d finding(s)\n", rep.Errors(), len(rep.Findings))
	}
	if rep.Errors() > 0 {
		os.Exit(1)
	}
	return nil
}

func cmdIndex(args []string) error {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	var o globalOpts
	fs.StringVar(&o.dir, "C", "", "workspace directory")
	_ = parseFlags(fs, args)

	ws, err := openWS(&o)
	if err != nil {
		return err
	}
	res, err := index.Build(ws, true)
	if err != nil {
		return err
	}
	fmt.Printf("Rebuilt indexes: %d objects, %d links, %d runs, %d attention\n", len(res.Objects), len(res.Links), len(res.Runs), len(res.Attention))
	return nil
}
