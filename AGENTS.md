# AGENTS.md — how to work in a Workgraph repository

This repo is a **Workgraph control repo**. It records durable work state for
humans and agents. It is not where you think; it is where you record what you
did and what is true now.

## Golden rules

1. **Current state = object frontmatter.** Events explain *how* the repo got
   there; frontmatter wins for *what is true now*.
2. **IDs are identity.** Filenames and paths are aliases. Never trust a path as
   an identity; resolve by ID.
3. **Every mutation writes an event.** Prefer the CLI/MCP so the event and the
   object change land in one commit. Manual edits are valid but must keep
   `workgraph validate` green.
4. **Never invent vocabulary.** Object types, item kinds, statuses, relation
   types, event actions, and attention reasons all come from
   `ontologies/workgraph.yaml`. Unknown values are validation errors.
5. **No secrets in source.** API keys, tokens, credentials, raw production logs,
   private customer data — never. Secret *names*, env var names, and vault paths
   are fine.
6. **Deletion is rare.** Prefer `cancelled`, `archived`, `duplicate_of`, or
   `superseded`. Physical deletion is for scaffold-never-committed, secrets, and
   generated caches only.

## Choosing and starting work

```bash
workgraph ready --json          # compact next-actionable items
workgraph show <id>             # one item, standard context
workgraph run <id> --repo <target> --agent claude --print
```

`workgraph run` creates a run, appends `run.created`/`run.started`, and writes a
**launch capsule** into the target repo at `.workgraph/runs/RUN-.../`. Read
`PROMPT.md` in that capsule and complete the task. Preserve the target repo's own
`CLAUDE.md`/`AGENTS.md` rules — Workgraph tells you *what* and *why*; the target
repo tells you *how* to work there.

## Reporting back

```bash
workgraph finish RUN-... --status review --summary .workgraph/runs/RUN-.../RESULT.md --pr 123
workgraph block  RUN-... "Need production API token"
```

## Creating work

Agents may create items, but new items default to `triage` unless created as a
direct child of an active item with `--ready`. Small checkboxes inside an item
body are fine and need no permission — they are local implementation detail, not
durable work rounds.
