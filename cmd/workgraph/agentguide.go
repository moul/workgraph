package main

// controlRepoAgentGuide is scaffolded into a new control repo as both CLAUDE.md
// and AGENTS.md so any agent — Claude Code (CLAUDE.md) or Codex and the broader
// convention (AGENTS.md) — reads the same operating contract. Kept short on
// purpose: it is loaded into agent context and should point at the CLI/MCP for
// deeper detail rather than duplicating it.
const controlRepoAgentGuide = `# Working in this Workgraph control repo

This repository records durable work state for humans and agents. It is not
where you think; it is where you record what you did and what is true now. The
source of truth is Markdown + YAML frontmatter + append-only JSONL events.

## Golden rules

1. **Current state = object frontmatter.** Events explain *how* the repo got
   here; frontmatter wins for *what is true now*.
2. **IDs are identity.** Filenames and paths are aliases — resolve by ID.
3. **Every mutation writes an event.** Prefer the ` + "`workgraph`" + ` CLI / MCP so the
   event and the object change land in one commit. Manual edits are valid but
   must keep ` + "`workgraph validate`" + ` green.
4. **Never invent vocabulary.** Object types, item kinds, statuses, relations,
   event actions, and attention reasons all come from
   ` + "`ontologies/workgraph.yaml`" + `. Unknown values are validation errors.
5. **No secrets in source.** Keys, tokens, credentials, raw production logs —
   never. Secret *names*, env var names, and vault paths are fine.
6. **Deletion is rare.** Prefer ` + "`cancelled`" + `, ` + "`archived`" + `, ` + "`duplicate_of`" + `, or
   ` + "`superseded`" + `.

## Choosing and starting work

    workgraph ready --json          # compact next-actionable items
    workgraph show <id>             # one item, standard context
    workgraph run <id> --repo <target> --agent claude --print

` + "`workgraph run`" + ` creates a run and writes a launch capsule into the target repo
at ` + "`.workgraph/runs/RUN-.../`" + `. Read its ` + "`PROMPT.md`" + ` and do the task. Preserve
the target repo's own CLAUDE.md / AGENTS.md — Workgraph tells you *what* and
*why*; the target repo tells you *how* to work there.

## Reporting back

    workgraph finish RUN-... --status review --summary .workgraph/runs/RUN-.../RESULT.md --pr 123
    workgraph block  RUN-... "Need production API token"

## Creating work

New items default to ` + "`triage`" + ` unless created as a child of an active item with
` + "`--ready`" + `. Checkboxes inside an item body are fine and need no permission — they
are local implementation detail, not durable work rounds.
`
