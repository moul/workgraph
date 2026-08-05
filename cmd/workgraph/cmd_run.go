package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moul/workgraph/internal/model"
)

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	var o globalOpts
	addGlobal(fs, &o)
	repo := fs.String("repo", "", "target repo checkout path to receive the launch capsule")
	agent := fs.String("agent", "generic", "agent adapter: generic|claude|codex")
	worker := fs.String("worker", "", "worker identity (default: --actor)")
	print := fs.Bool("print", false, "print the launch prompt for the chosen agent")
	redactMode := fs.String("redact", "auto", "redact secrets from the capsule: auto|on|off (auto = on when the target is outside the control repo)")
	jsonOut := fs.Bool("json", false, "output JSON")
	_ = parseFlags(fs, args)

	ref, err := requireArg(fs, 0, "item reference")
	if err != nil {
		return err
	}
	e, err := engine(&o)
	if err != nil {
		return err
	}
	defer noteConflict(e)
	target := ""
	if *repo != "" {
		target, err = filepath.Abs(*repo)
		if err != nil {
			return err
		}
	}
	res, err := e.CreateRun(ref, *worker, *agent, target, *redactMode)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(res.Run)
	}
	fmt.Printf("started run %s (round %d) on %s\n", res.Run.RunID, res.Round, res.Run.ItemID)
	if res.CapsuleDir != "" {
		rel := res.CapsuleDir
		fmt.Printf("  capsule: %s\n", rel)
	}
	if *print {
		fmt.Println()
		fmt.Println(launchPrompt(*agent, res.Run.RunID, res.CapsuleDir))
	}
	return nil
}

func launchPrompt(agent, runID, capsuleDir string) string {
	promptPath := ".workgraph/runs/" + runID + "/PROMPT.md"
	if capsuleDir != "" {
		promptPath = filepath.Join(capsuleDir, "PROMPT.md")
	}
	switch agent {
	case "claude":
		return fmt.Sprintf("Read %s, then do the task. Preserve this repo's CLAUDE.md rules.", promptPath)
	case "codex":
		return fmt.Sprintf("Read %s, then do the task. Preserve this repo's AGENTS.md rules.", promptPath)
	default:
		return fmt.Sprintf("Read %s and complete the task.", promptPath)
	}
}

func cmdFinish(args []string) error {
	fs := flag.NewFlagSet("finish", flag.ExitOnError)
	var o globalOpts
	addGlobal(fs, &o)
	status := fs.String("status", "review", "resulting item status")
	summaryFile := fs.String("summary", "", "path to a result summary file")
	summaryText := fs.String("message", "", "inline summary text")
	pr := fs.String("pr", "", "PR reference (e.g. 123 or github:owner/repo#123)")
	_ = parseFlags(fs, args)

	runID, err := requireArg(fs, 0, "run id")
	if err != nil {
		return err
	}
	summary := *summaryText
	if *summaryFile != "" {
		b, err := os.ReadFile(*summaryFile)
		if err != nil {
			return err
		}
		summary = firstParagraph(string(b))
	}
	e, err := engine(&o)
	if err != nil {
		return err
	}
	defer noteConflict(e)
	it, err := e.Finish(runID, *status, summary, *pr)
	if err != nil {
		return err
	}
	fmt.Printf("finished %s: %s is now [%s]\n", runID, it.ID, it.Status)
	return nil
}

func cmdBlock(args []string) error {
	fs := flag.NewFlagSet("block", flag.ExitOnError)
	var o globalOpts
	addGlobal(fs, &o)
	reasonFlag := fs.String("reason", "", "reason for blocking")
	_ = parseFlags(fs, args)

	runID, err := requireArg(fs, 0, "run id")
	if err != nil {
		return err
	}
	reason := *reasonFlag
	if reason == "" && fs.NArg() > 1 {
		reason = fs.Arg(1)
	}
	if reason == "" {
		return fmt.Errorf("a --reason is required")
	}
	e, err := engine(&o)
	if err != nil {
		return err
	}
	defer noteConflict(e)
	it, err := e.Block(runID, reason)
	if err != nil {
		return err
	}
	fmt.Printf("blocked %s: %s is now [%s]\n", runID, it.ID, it.Status)
	return nil
}

func cmdHeartbeat(args []string) error {
	fs := flag.NewFlagSet("heartbeat", flag.ExitOnError)
	var o globalOpts
	addGlobal(fs, &o)
	msg := fs.String("message", "", "progress message")
	_ = parseFlags(fs, args)

	runID, err := requireArg(fs, 0, "run id")
	if err != nil {
		return err
	}
	m := *msg
	if m == "" && fs.NArg() > 1 {
		m = fs.Arg(1)
	}
	e, err := engine(&o)
	if err != nil {
		return err
	}
	defer noteConflict(e)
	if err := e.Heartbeat(runID, m); err != nil {
		return err
	}
	fmt.Printf("heartbeat recorded for %s\n", runID)
	return nil
}

func cmdLink(args []string) error {
	fs := flag.NewFlagSet("link", flag.ExitOnError)
	var o globalOpts
	addGlobal(fs, &o)
	_ = parseFlags(fs, args)

	if fs.NArg() < 3 {
		return fmt.Errorf("usage: workgraph link <from> <relation> <to>")
	}
	e, err := engine(&o)
	if err != nil {
		return err
	}
	defer noteConflict(e)
	it, err := e.Link(fs.Arg(0), fs.Arg(1), fs.Arg(2))
	if err != nil {
		return err
	}
	fmt.Printf("linked %s %s %s\n", it.ID, fs.Arg(1), fs.Arg(2))
	return nil
}

func cmdItemUpdate(args []string) error {
	fs := flag.NewFlagSet("item update", flag.ExitOnError)
	var o globalOpts
	addGlobal(fs, &o)
	status := fs.String("status", "", "new status")
	priority := fs.String("priority", "", "new priority")
	owner := fs.String("owner", "", "new owner (worker id)")
	reviewer := fs.String("reviewer", "", "new reviewer (worker id)")
	targetRepo := fs.String("target-repo", "", "target repo URL")
	reason := fs.String("reason", "", "message for the event")
	_ = parseFlags(fs, args)

	ref, err := requireArg(fs, 0, "item reference")
	if err != nil {
		return err
	}
	e, err := engine(&o)
	if err != nil {
		return err
	}
	defer noteConflict(e)
	if *status != "" && !e.WS.Ontology.Has("item_status", *status) {
		return fmt.Errorf("unknown status %q", *status)
	}
	msg := *reason
	if msg == "" {
		msg = "update"
	}
	it, err := e.UpdateItem(ref, func(i *model.Item) {
		if *status != "" {
			i.Status = *status
		}
		if *priority != "" {
			i.Priority = *priority
		}
		if *owner != "" {
			i.Owner = *owner
		}
		if *reviewer != "" {
			i.Reviewer = *reviewer
		}
		if *targetRepo != "" {
			i.TargetRepo = *targetRepo
		}
	}, o.expectedVersion, msg)
	if err != nil {
		return err
	}
	fmt.Printf("updated %s [%s]\n", it.ID, it.Status)
	return nil
}

func firstParagraph(s string) string {
	var para []string
	for _, ln := range strings.Split(s, "\n") {
		t := strings.TrimSpace(ln)
		if t == "" {
			if len(para) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "<!--") {
			continue
		}
		para = append(para, t)
	}
	return strings.Join(para, " ")
}
