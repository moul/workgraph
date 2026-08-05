package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moul/workgraph/internal/eventlog"
	"github.com/moul/workgraph/internal/index"
	"github.com/moul/workgraph/internal/model"
	"github.com/moul/workgraph/internal/store"
)

// cmdReady is the daily command: next actionable items (status ready), most
// important first.
func cmdReady(args []string) error {
	fs := flag.NewFlagSet("ready", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "output JSON")
	project := fs.String("project", "", "filter by project reference")
	var o globalOpts
	fs.StringVar(&o.dir, "C", "", "workspace directory")
	_ = parseFlags(fs, args)

	ws, err := openWS(&o)
	if err != nil {
		return err
	}
	g, err := ws.Load()
	if err != nil {
		return err
	}
	projID := ""
	if *project != "" {
		if o, err := g.Resolve(*project); err == nil {
			projID = o.ObjectID()
		}
	}
	var items []*model.Item
	for _, it := range g.Items {
		if it.Status != model.StatusReady {
			continue
		}
		if projID != "" && it.Project != projID {
			continue
		}
		items = append(items, it)
	}
	sort.Slice(items, func(i, j int) bool {
		if pr := priorityRank(items[i].Priority) - priorityRank(items[j].Priority); pr != 0 {
			return pr < 0
		}
		return items[i].ID < items[j].ID
	})
	return renderItems(items, *jsonOut)
}

func cmdAttention(args []string) error {
	fs := flag.NewFlagSet("attention", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "output JSON")
	var o globalOpts
	fs.StringVar(&o.dir, "C", "", "workspace directory")
	_ = parseFlags(fs, args)

	ws, err := openWS(&o)
	if err != nil {
		return err
	}
	res, err := index.Build(ws, false)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(res.Attention)
	}
	if len(res.Attention) == 0 {
		fmt.Println("nothing needs attention 🎉")
		return nil
	}
	tw := newTab()
	fmt.Fprintln(tw, "ID\tSEVERITY\tREASON\tSUMMARY")
	for _, a := range res.Attention {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", a.ID, a.Severity, a.Reason, a.Summary)
	}
	return tw.Flush()
}

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "output JSON")
	status := fs.String("status", "", "filter by status")
	project := fs.String("project", "", "filter by project reference")
	typ := fs.String("type", "", "filter by object type")
	var o globalOpts
	fs.StringVar(&o.dir, "C", "", "workspace directory")
	_ = parseFlags(fs, args)

	ws, err := openWS(&o)
	if err != nil {
		return err
	}
	res, err := index.Build(ws, false)
	if err != nil {
		return err
	}
	projID := ""
	if *project != "" {
		if g, err := ws.Load(); err == nil {
			if o, err := g.Resolve(*project); err == nil {
				projID = o.ObjectID()
			}
		}
	}
	var rows []index.ObjectLine
	for _, r := range res.Objects {
		if *status != "" && r.Status != *status {
			continue
		}
		if *typ != "" && r.Type != *typ {
			continue
		}
		if projID != "" && r.Project != projID {
			continue
		}
		rows = append(rows, r)
	}
	if *jsonOut {
		return printJSON(rows)
	}
	tw := newTab()
	fmt.Fprintln(tw, "ID\tTYPE\tSTATUS\tOWNER\tTITLE")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.ID, r.Type, r.Status, shortWorker(r.Owner), r.Title)
	}
	return tw.Flush()
}

func cmdShow(args []string) error {
	fs := flag.NewFlagSet("show", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "output JSON")
	context := fs.String("context", "standard", "context level: compact|standard|full")
	var o globalOpts
	fs.StringVar(&o.dir, "C", "", "workspace directory")
	_ = parseFlags(fs, args)

	ref, err := requireArg(fs, 0, "reference")
	if err != nil {
		return err
	}
	ws, err := openWS(&o)
	if err != nil {
		return err
	}
	g, err := ws.Load()
	if err != nil {
		return err
	}
	obj, err := g.Resolve(ref)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(showJSON(ws, obj, *context))
	}
	raw, rerr := os.ReadFile(rootJoin(ws.Root, obj.SourcePath()))
	if rerr != nil {
		return rerr
	}
	if *context == "compact" {
		fmt.Printf("%s [%s] %s\n%s\n", obj.ObjectID(), obj.ObjectStatus(), obj.ObjectTitle(), obj.SourcePath())
		return nil
	}
	fmt.Print(string(raw))
	return nil
}

func cmdHistory(args []string) error {
	fs := flag.NewFlagSet("history", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "output JSON")
	var o globalOpts
	fs.StringVar(&o.dir, "C", "", "workspace directory")
	_ = parseFlags(fs, args)

	ref, err := requireArg(fs, 0, "item reference")
	if err != nil {
		return err
	}
	ws, err := openWS(&o)
	if err != nil {
		return err
	}
	g, err := ws.Load()
	if err != nil {
		return err
	}
	obj, err := g.Resolve(ref)
	if err != nil {
		return err
	}
	res, err := index.Build(ws, false)
	if err != nil {
		return err
	}
	var runs []index.RunLine
	for _, r := range res.Runs {
		if r.Item == obj.ObjectID() {
			runs = append(runs, r)
		}
	}
	if *jsonOut {
		return printJSON(runs)
	}
	if len(runs) == 0 {
		fmt.Printf("no runs for %s\n", obj.ObjectID())
		return nil
	}
	tw := newTab()
	fmt.Fprintln(tw, "ROUND\tWORKER\tSTATUS\tSTARTED\tRESULT")
	for _, r := range runs {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n", r.Round, shortWorker(r.Worker), r.Status, shortTime(r.StartedAt), r.Summary)
	}
	return tw.Flush()
}

// cmdItem dispatches item subcommands.
func cmdItem(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: workgraph item <create|list|show|update|history>")
	}
	switch args[0] {
	case "create":
		return newTask(args[1:])
	case "list":
		return cmdList(append([]string{"-type", "item"}, args[1:]...))
	case "show":
		return cmdShow(args[1:])
	case "update":
		return cmdItemUpdate(args[1:])
	case "history":
		return cmdHistory(args[1:])
	default:
		return fmt.Errorf("unknown item subcommand %q", args[0])
	}
}

func renderItems(items []*model.Item, jsonOut bool) error {
	if jsonOut {
		type row struct {
			ID, Title, Status, Priority, Owner, TargetRepo string
		}
		out := make([]row, 0, len(items))
		for _, it := range items {
			out = append(out, row{it.ID, it.Title, it.Status, it.Priority, it.Owner, it.TargetRepo})
		}
		return printJSON(out)
	}
	if len(items) == 0 {
		fmt.Println("no ready items")
		return nil
	}
	tw := newTab()
	fmt.Fprintln(tw, "ID\tPRIORITY\tOWNER\tTARGET\tTITLE")
	for _, it := range items {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", it.ID, orDash(it.Priority), shortWorker(it.Owner), shortRepo(it.TargetRepo), it.Title)
	}
	return tw.Flush()
}

func showJSON(ws *store.Workspace, obj model.Object, context string) any {
	m := map[string]any{
		"id":     obj.ObjectID(),
		"type":   obj.ObjectType(),
		"title":  obj.ObjectTitle(),
		"status": obj.ObjectStatus(),
		"path":   obj.SourcePath(),
	}
	if context == "compact" {
		return m
	}
	if b, ok := obj.(interface{ Body() string }); ok && context == "full" {
		m["body"] = b.Body()
	}
	if it, ok := obj.(*model.Item); ok {
		m["kind"] = it.Kind
		m["project"] = it.Project
		m["priority"] = it.Priority
		m["depends_on"] = it.DependsOn
		m["target_repo"] = it.TargetRepo
	}
	// include recent events for this object
	if evs, err := eventlog.ReadAll(ws.Root); err == nil {
		m["events"] = eventlog.ForObject(evs, obj.ObjectID())
	}
	return m
}

func priorityRank(p string) int {
	switch strings.ToLower(p) {
	case "urgent", "critical":
		return 0
	case "high":
		return 1
	case "normal", "medium", "":
		return 2
	case "low":
		return 3
	default:
		return 2
	}
}

func shortWorker(s string) string {
	s = strings.TrimPrefix(s, "worker:")
	s = strings.TrimPrefix(s, "agent:")
	s = strings.TrimPrefix(s, "human:")
	if s == "" {
		return "-"
	}
	return s
}

func shortRepo(s string) string {
	if s == "" {
		return "-"
	}
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return strings.TrimSuffix(s[i+1:], ".git")
	}
	return s
}

func shortTime(s string) string {
	if len(s) >= 16 {
		return strings.Replace(s[:16], "T", " ", 1)
	}
	return s
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func rootJoin(root, rel string) string { return filepath.Join(root, rel) }
