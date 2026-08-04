package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/moul/workgraph/internal/core"
)

func cmdImport(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: workgraph import <markdown|json> <file> [flags]")
	}
	switch args[0] {
	case "markdown":
		return importMarkdown(args[1:])
	case "json", "jsonl":
		return importJSON(args[1:])
	default:
		return fmt.Errorf("unknown import source %q (want markdown|json)", args[0])
	}
}

var checkboxRe = regexp.MustCompile(`^\s*[-*]\s+\[[ xX]\]\s+(.+)$`)

// importMarkdown turns unchecked/checked checkboxes in a Markdown file into
// inbox items, one per line, idempotent by source_ref.
func importMarkdown(args []string) error {
	fs := flag.NewFlagSet("import markdown", flag.ExitOnError)
	var o globalOpts
	addGlobal(fs, &o)
	project := fs.String("project", "", "project reference")
	status := fs.String("state", "inbox", "status for imported items (inbox|triage)")
	_ = parseFlags(fs, args)

	file, err := requireArg(fs, 0, "markdown file")
	if err != nil {
		return err
	}
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	var specs []core.ImportSpec
	sc := bufio.NewScanner(f)
	ln := 0
	for sc.Scan() {
		ln++
		m := checkboxRe.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		title := strings.TrimSpace(m[1])
		specs = append(specs, core.ImportSpec{
			Title:     title,
			Source:    "markdown",
			SourceRef: fmt.Sprintf("markdown:%s:%d", baseName(file), ln),
			Project:   *project,
			Status:    *status,
		})
	}
	return runImport(&o, *project, specs)
}

// importJSON imports items from a JSONL file of {title, body, source_ref, ...}.
func importJSON(args []string) error {
	fs := flag.NewFlagSet("import json", flag.ExitOnError)
	var o globalOpts
	addGlobal(fs, &o)
	project := fs.String("project", "", "project reference")
	status := fs.String("state", "triage", "status for imported items")
	_ = parseFlags(fs, args)

	file, err := requireArg(fs, 0, "json/jsonl file")
	if err != nil {
		return err
	}
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	var specs []core.ImportSpec
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var in struct {
			Title, Body, SourceRef, Source, Kind, Status string
		}
		if err := json.Unmarshal([]byte(line), &in); err != nil {
			return fmt.Errorf("import json: %w", err)
		}
		if in.Status == "" {
			in.Status = *status
		}
		if in.Source == "" {
			in.Source = "json"
		}
		specs = append(specs, core.ImportSpec{
			Title: in.Title, Body: in.Body, SourceRef: in.SourceRef,
			Source: in.Source, Kind: in.Kind, Status: in.Status, Project: *project,
		})
	}
	return runImport(&o, *project, specs)
}

func runImport(o *globalOpts, projectRef string, specs []core.ImportSpec) error {
	e, err := engine(o)
	if err != nil {
		return err
	}
	// Resolve project reference to an id for all specs.
	if projectRef != "" {
		g, err := e.WS.Load()
		if err != nil {
			return err
		}
		p, err := g.Resolve(projectRef)
		if err != nil {
			return err
		}
		for i := range specs {
			specs[i].Project = p.ObjectID()
		}
	}
	created, skipped, err := e.Import(specs)
	if err != nil {
		return err
	}
	fmt.Printf("imported %d item(s), skipped %d duplicate(s)\n", created, skipped)
	return nil
}

func baseName(p string) string {
	if i := strings.LastIndexAny(p, "/\\"); i >= 0 {
		return p[i+1:]
	}
	return p
}
