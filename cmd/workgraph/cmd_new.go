package main

import (
	"flag"
	"fmt"
)

func cmdNew(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: workgraph new <project|task|decision> \"Title\" [flags]")
	}
	switch args[0] {
	case "project":
		return newProject(args[1:])
	case "task", "item":
		return newTask(args[1:])
	case "decision":
		return newDecision(args[1:])
	default:
		return fmt.Errorf("unknown new subject %q (want project|task|decision)", args[0])
	}
}

func cmdProject(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: workgraph project create \"Title\" [flags]")
	}
	switch args[0] {
	case "create":
		return newProject(args[1:])
	default:
		return fmt.Errorf("unknown project subcommand %q", args[0])
	}
}

func newProject(args []string) error {
	fs := flag.NewFlagSet("new project", flag.ExitOnError)
	var o globalOpts
	addGlobal(fs, &o)
	targetRepo := fs.String("target-repo", "", "target git repo URL")
	targetRef := fs.String("target-ref", "main", "target git ref")
	jsonOut := fs.Bool("json", false, "output JSON")
	_ = parseFlags(fs, args)

	title, err := requireArg(fs, 0, "title")
	if err != nil {
		return err
	}
	e, err := engine(&o)
	if err != nil {
		return err
	}
	defer noteConflict(e)
	p, err := e.CreateProject(title, *targetRepo, *targetRef)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(map[string]string{"id": p.ID, "title": p.Title, "path": p.SourcePath()})
	}
	fmt.Printf("created project %s (%s)\n  %s\n", p.ID, p.Title, p.SourcePath())
	return nil
}

func newTask(args []string) error {
	fs := flag.NewFlagSet("new task", flag.ExitOnError)
	var o globalOpts
	addGlobal(fs, &o)
	project := fs.String("project", "", "project reference")
	kind := fs.String("kind", "task", "item kind (task|bug|question|experiment|review|epic|incident|idea)")
	ready := fs.Bool("ready", false, "create as ready instead of triage")
	jsonOut := fs.Bool("json", false, "output JSON")
	_ = parseFlags(fs, args)

	title, err := requireArg(fs, 0, "title")
	if err != nil {
		return err
	}
	e, err := engine(&o)
	if err != nil {
		return err
	}
	defer noteConflict(e)
	it, err := e.CreateItem(title, *project, *kind, *ready)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(map[string]string{"id": it.ID, "title": it.Title, "status": it.Status, "path": it.SourcePath()})
	}
	fmt.Printf("created item %s [%s] (%s)\n  %s\n", it.ID, it.Status, it.Title, it.SourcePath())
	return nil
}

func newDecision(args []string) error {
	fs := flag.NewFlagSet("new decision", flag.ExitOnError)
	var o globalOpts
	addGlobal(fs, &o)
	project := fs.String("project", "", "project reference")
	status := fs.String("status", "proposed", "decision status (proposed|accepted|superseded|rejected)")
	jsonOut := fs.Bool("json", false, "output JSON")
	_ = parseFlags(fs, args)

	title, err := requireArg(fs, 0, "title")
	if err != nil {
		return err
	}
	e, err := engine(&o)
	if err != nil {
		return err
	}
	defer noteConflict(e)
	d, err := e.CreateDecision(title, *project, *status)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(map[string]string{"id": d.ID, "title": d.Title, "status": d.Status, "path": d.SourcePath()})
	}
	fmt.Printf("created decision %s [%s] (%s)\n  %s\n", d.ID, d.Status, d.Title, d.SourcePath())
	return nil
}
