package main

import (
	"flag"
	"fmt"
)

func cmdCompletions(args []string) error {
	fs := flag.NewFlagSet("completions", flag.ExitOnError)
	_ = parseFlags(fs, args)
	shell := "zsh"
	if fs.NArg() > 0 {
		shell = fs.Arg(0)
	}
	switch shell {
	case "zsh":
		fmt.Print(zshCompletion)
	case "bash":
		fmt.Print(bashCompletion)
	default:
		return fmt.Errorf("unsupported shell %q (want zsh|bash)", shell)
	}
	return nil
}

const zshCompletion = `#compdef workgraph
_workgraph() {
  local -a cmds
  cmds=(
    'init:initialize a workspace'
    'new:create a project/task/decision'
    'ready:list next actionable items'
    'show:show one object'
    'run:start a work round'
    'finish:finish a run'
    'block:block a run'
    'list:list objects'
    'attention:where a human must intervene'
    'history:item work-round history'
    'validate:deterministic validation'
    'index:rebuild indexes'
    'ontology:show/audit ontology'
    'doctor:environment health'
    'serve:HTTP + MCP gateway'
    'token:manage gateway tokens'
    'mcp:stdio MCP server'
    'completions:completion script'
  )
  _describe 'command' cmds
}
_workgraph "$@"
`

const bashCompletion = `# bash completion for workgraph
_workgraph() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  local cmds="init new ready show run finish block list attention history validate index ontology doctor serve token mcp completions"
  COMPREPLY=( $(compgen -W "$cmds" -- "$cur") )
}
complete -F _workgraph workgraph
`
