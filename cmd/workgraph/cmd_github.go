package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/moul/workgraph/internal/core"
)

// importGitHub imports GitHub issues as triage items via the `gh` CLI,
// idempotent by source_ref (github:owner/repo#N). It never touches the target
// repo; it only writes items into the control workspace.
func importGitHub(args []string) error {
	fs := flag.NewFlagSet("import github", flag.ExitOnError)
	var o globalOpts
	addGlobal(fs, &o)
	repo := fs.String("repo", "", "GitHub repo (owner/name)")
	state := fs.String("issues", "open", "issue state to import (open|closed|all)")
	project := fs.String("project", "", "project reference")
	itemState := fs.String("state", "triage", "workgraph status for imported items")
	limit := fs.Int("limit", 200, "max issues to fetch")
	_ = parseFlags(fs, args)

	if *repo == "" {
		return fmt.Errorf("--repo owner/name is required")
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("the `gh` CLI is required for github import; install it or use `workgraph import json`")
	}

	out, err := ghJSON("issue", "list", "--repo", *repo, "--state", *state, "--limit", fmt.Sprint(*limit), "--json", "number,title,body,url,state")
	if err != nil {
		return err
	}
	var issues []struct {
		Number int
		Title  string
		Body   string
		URL    string
		State  string
	}
	if err := json.Unmarshal(out, &issues); err != nil {
		return fmt.Errorf("parse gh output: %w", err)
	}

	specs := make([]core.ImportSpec, 0, len(issues))
	for _, is := range issues {
		body := fmt.Sprintf("# %s\n\n%s\n\n## Source\n\n%s (state: %s)\n", is.Title, is.Body, is.URL, is.State)
		specs = append(specs, core.ImportSpec{
			Title:     is.Title,
			Body:      body,
			Source:    "github",
			SourceRef: fmt.Sprintf("github:%s#%d", *repo, is.Number),
			Status:    *itemState,
			Kind:      "task",
			Project:   *project,
		})
	}
	return runImport(&o, *project, specs)
}

func ghJSON(args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gh %v: %v: %s", args, err, errb.String())
	}
	return out.Bytes(), nil
}

// cmdDiscover inspects a target repo for candidate sources without modifying
// anything. Existing-project onboarding is non-invasive by default.
func cmdDiscover(args []string) error {
	fs := flag.NewFlagSet("discover", flag.ExitOnError)
	repo := fs.String("repo", ".", "target repo checkout path")
	jsonOut := fs.Bool("json", false, "output JSON")
	_ = parseFlags(fs, args)

	root, err := filepath.Abs(*repo)
	if err != nil {
		return err
	}

	type finding struct {
		Kind   string `json:"kind"`
		Detail string `json:"detail"`
	}
	var found []finding

	// Instruction & doc files.
	for _, name := range []string{"AGENTS.md", "CLAUDE.md", "README.md", "CONTRIBUTING.md", "TODO.md", "ROADMAP.md"} {
		if fi, err := os.Stat(filepath.Join(root, name)); err == nil && !fi.IsDir() {
			kind := "doc"
			if name == "AGENTS.md" || name == "CLAUDE.md" {
				kind = "agent_instructions"
			}
			found = append(found, finding{kind, name})
		}
	}
	// Makefile targets.
	if raw, err := os.ReadFile(filepath.Join(root, "Makefile")); err == nil {
		for _, t := range makeTargets(raw) {
			found = append(found, finding{"make_target", t})
		}
	}
	// docs/ directory.
	if fi, err := os.Stat(filepath.Join(root, "docs")); err == nil && fi.IsDir() {
		found = append(found, finding{"docs_dir", "docs/"})
	}
	// GitHub issues, if gh + a github remote are available.
	if _, err := exec.LookPath("gh"); err == nil {
		if out, err := ghJSON("issue", "list", "--limit", "1", "--json", "number"); err == nil {
			var xs []map[string]any
			if json.Unmarshal(out, &xs) == nil {
				found = append(found, finding{"github_issues", "issue tracker reachable via gh"})
			}
		}
	}

	if *jsonOut {
		return printJSON(found)
	}
	if len(found) == 0 {
		fmt.Printf("no candidate sources found in %s\n", root)
		return nil
	}
	fmt.Printf("candidate sources in %s (nothing was modified):\n", root)
	tw := newTab()
	fmt.Fprintln(tw, "KIND\tDETAIL")
	for _, f := range found {
		fmt.Fprintf(tw, "%s\t%s\n", f.Kind, f.Detail)
	}
	_ = tw.Flush()
	fmt.Println("\nNext: workgraph import github --repo owner/name --issues open  (or import markdown TODO.md)")
	return nil
}

var makeTargetRe = regexp.MustCompile(`(?m)^([a-zA-Z][a-zA-Z0-9_-]*):`)

func makeTargets(raw []byte) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range makeTargetRe.FindAllSubmatch(raw, -1) {
		t := string(m[1])
		if t == "PHONY" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}
