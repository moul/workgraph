# Workgraph

> A Git-native work graph for humans and agents: durable tasks, decisions, run
> context, status, and cross-repo agent handoff without a required daemon.

Workgraph is **not where agents think**. It is **where humans and agents agree on
durable work state**.

The source of truth is plain Markdown + YAML frontmatter + append-only JSONL
events in a Git repository. Everything else — the CLI, the JSONL indexes, the
SQLite cache, the MCP server, the HTTP gateway — is a projection over those
files. When tools fail, a tired human on SSH can still understand and repair the
state with `ls`, `sed`, `git diff`, and `jq`.

```text
source plane:   Markdown objects + events JSONL   (authoritative)
index plane:    deterministic JSONL indexes + optional SQLite cache
access plane:   HTTPS API + remote MCP + tokenized read page
adapter plane:  launch capsules + optional CLAUDE/AGENTS helpers
```

## Install

```bash
go install github.com/moul/workgraph/cmd/workgraph@latest
workgraph init
workgraph doctor
```

No local database service, no Node runtime, no Obsidian dependency, no daemon
required for the CLI.

## The daily loop

```bash
workgraph init                       # create a workspace
workgraph new project "Hermes" --target-repo git@github.com:moul/hermes.git
workgraph new task "Add run summary" --project hermes
workgraph ready                      # the daily command: next actionable items
workgraph show ITM-01K...            # inspect one item
workgraph run ITM-01K... --repo ../hermes --agent claude --print
workgraph finish RUN-01K... --status review --pr 123 --summary RESULT.md
workgraph block RUN-01K... "Need API token"
workgraph validate                   # deterministic checks
workgraph index                      # rebuild JSONL indexes
```

## Repository contract

```text
workgraph.yaml              # workspace identity and defaults
ontologies/workgraph.yaml   # the project-management ontology manifest
projects/*/PROJECT.md       # canonical project objects
projects/*/items/*.md       # canonical item objects
projects/*/decisions/*.md   # canonical decision objects
workers/*.md                # worker profiles
events/*.jsonl              # append-only semantic event log
indexes/*.jsonl             # deterministic generated compact indexes
.workgraph/                 # ignored cache and local runtime state
```

Authoritative: `workgraph.yaml`, the Markdown object files, `events/*.jsonl`.
Committed but generated: `indexes/*.jsonl`. Ignored: `.workgraph/`.

## Object model

Four object types: `project`, `item`, `decision`, `worker`. An **item** is the
durable issue. A **round** (`RUN-...`) is one worker's bounded attempt to move an
item forward. An **event** is one immutable fact. IDs are immutable; filenames
and paths are ergonomic aliases (filename must start with the ID).

Status vocabulary (deliberately tiny):

```text
inbox  triage  ready  in_progress  blocked  review  done  cancelled  archived
```

## Access surfaces

```bash
workgraph serve --addr :8080 --repo /srv/workgraph/state --bootstrap-admin-token
workgraph token create --scope runs:context,runs:event --run RUN-...
workgraph mcp install claude
workgraph ui --static           # read-only HTML site from indexes/*.jsonl
workgraph import markdown TODO.md --state inbox
```

The gateway exposes the same core mutation package as the CLI over HTTPS + remote
MCP + a tokenized `/t/{token}` page for zero-install cloud agents. No UI-only,
MCP-only, or CLI-only write path.

## Documentation

- [`docs/spec.md`](docs/spec.md) — the file format and protocol specification.
- [`docs/api.md`](docs/api.md) — the HTTP gateway API.
- [`docs/mcp.md`](docs/mcp.md) — the MCP surface.
- [`AGENTS.md`](AGENTS.md) — how an agent uses a Workgraph repo.
- [`examples/basic`](examples/basic) — a runnable example workspace.

## License

Apache-2.0 OR MIT (see [`LICENSE`](LICENSE)).
