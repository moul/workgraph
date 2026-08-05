// Command workgraph is the Git-native work graph CLI for humans and agents.
//
// It is a convenience client over the same core mutation package the MCP server
// and HTTP gateway use. It is not the only viable way to use Workgraph: the
// source of truth is plain Markdown + JSONL that any tool can read.
package main

import (
	"fmt"
	"os"
)

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

// command is one CLI verb.
type command struct {
	name  string
	short string
	run   func(args []string) error
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	// Peel off pre-command global flags (git-style), e.g. `workgraph -C dir cmd`.
	for len(args) >= 1 {
		if args[0] == "-C" || args[0] == "--dir" {
			if len(args) < 2 {
				return fmt.Errorf("%s requires a directory argument", args[0])
			}
			os.Setenv("WORKGRAPH_DIR", args[1])
			args = args[2:]
			continue
		}
		break
	}
	if len(args) == 0 {
		usage()
		return nil
	}
	switch args[0] {
	case "-h", "--help", "help":
		usage()
		return nil
	case "-v", "--version", "version":
		fmt.Println("workgraph", version)
		return nil
	}

	cmds := commands()
	name := args[0]
	for _, c := range cmds {
		if c.name == name {
			return c.run(args[1:])
		}
	}
	// Support "item <sub>", "new <sub>", etc. via prefixed lookup already in table.
	return fmt.Errorf("unknown command %q (run `workgraph help`)", name)
}

func commands() []command {
	return []command{
		{"init", "initialize a new workspace", cmdInit},
		{"doctor", "check environment and workspace health", cmdDoctor},
		{"validate", "run deterministic validation", cmdValidate},
		{"index", "rebuild the JSONL indexes", cmdIndex},
		{"new", "create a project, task, or decision", cmdNew},
		{"project", "project subcommands (create)", cmdProject},
		{"item", "item subcommands (create, list, show, update, history)", cmdItem},
		{"show", "show one object", cmdShow},
		{"ready", "list next actionable items (the daily command)", cmdReady},
		{"attention", "show where a human must intervene", cmdAttention},
		{"list", "list objects with filters", cmdList},
		{"run", "start a work round and generate a launch capsule", cmdRun},
		{"finish", "finish a run and set the resulting status", cmdFinish},
		{"block", "block a run with a reason", cmdBlock},
		{"heartbeat", "record progress for a long-running run", cmdHeartbeat},
		{"link", "add a typed relation between items", cmdLink},
		{"import", "import external items (markdown, jsonl, github)", cmdImport},
		{"discover", "inspect a target repo for candidate sources (non-invasive)", cmdDiscover},
		{"history", "show an item's work-round history", cmdHistory},
		{"ontology", "show or audit the ontology manifest", cmdOntology},
		{"health", "show suggested, explainable project health", cmdHealth},
		{"ui", "generate or serve a read-only UI from the indexes", cmdUI},
		{"serve", "run the HTTP + MCP gateway", cmdServe},
		{"token", "create, list, or revoke gateway tokens", cmdToken},
		{"mcp", "run the stdio MCP server or install adapters", cmdMCP},
		{"completions", "print a shell completion script", cmdCompletions},
	}
}

func usage() {
	fmt.Print(`workgraph — a Git-native work graph for humans and agents

Usage:
  workgraph <command> [flags]

Daily loop:
  init        initialize a new workspace
  new         create a project / task / decision
  ready       list next actionable items
  show        show one object
  run         start a work round, generate a launch capsule
  finish      finish a run, set resulting status
  block       block a run with a reason

Inspect:
  list        list objects with filters
  attention   where a human must intervene
  history     an item's work-round history
  validate    deterministic validation
  index       rebuild JSONL indexes
  ontology    show / audit the ontology manifest
  doctor      environment + workspace health

Serve:
  serve       HTTP + MCP gateway
  token       manage gateway tokens
  mcp         stdio MCP server / install adapters
  completions shell completion script

Run 'workgraph <command> -h' for command flags.
`)
}
