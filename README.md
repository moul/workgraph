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

## This repo is the tool — your work lives in a *separate* repo

`moul/workgraph` (this repo) is the **reference**: the Go source, the tests, the
docs, and an example workspace. You install the binary from it — you don't put
your tasks here.

Your actual work graph lives in **your own, separate, private control repo**
that `workgraph init` creates. Three kinds of repo, never mixed:

```text
moul/workgraph        the tool + reference      (this repo; go install)
your control repo     your durable work state   (private; workgraph init)
target repos          the code you change       (hermes, ...; capsules only)
```

**Want to give it a try? →  [`docs/getting-started.md`](docs/getting-started.md)**
walks through standing up your private instance in about two minutes.

## Install

Requires **Go 1.23+** and **git** on `PATH`. No database service, no Node
runtime, no Obsidian dependency, no daemon.

```bash
# from the public module
go install github.com/moul/workgraph/cmd/workgraph@latest

# or from a local clone (works offline; stamps the version from git)
git clone https://github.com/moul/workgraph && cd workgraph && make install
```

Make sure `$(go env GOPATH)/bin` is on your `PATH`, then:

```bash
workgraph version
workgraph doctor
```

> Private repo? Either use the local-clone install, or set
> `GOPRIVATE=github.com/moul/*` with git SSH/token auth configured.

## Quickstart: your private instance

Create a **separate private repo** for your work graph, then use it. (Full guide
with diagrams, local-only, and self-hosting: [`docs/getting-started.md`](docs/getting-started.md).)

```bash
workgraph init ~/p/workgraph-state
cd ~/p/workgraph-state
git add -A && git commit -m "init workgraph workspace"

# create the private GitHub repo from this folder and push it
gh repo create <you>/workgraph-state --private --source=. --remote=origin --push

# from now on, every mutation auto-commits and pushes to your private repo
workgraph new project "My Stuff" --target-repo git@github.com:<you>/hermes.git
workgraph new task "Try workgraph" --project my-stuff --ready
workgraph ready
```

No GitHub? `workgraph init` already runs `git init`; commit once and you're
working locally — add a remote whenever you like (use `--no-push` on mutations
until then).

## Usage

Workgraph has one core mutation path exposed three ways — **CLI**, **HTTP API**,
and **MCP** — each usable by a human or an agent. Pick the row that matches you.

### 1. CLI — human (the daily loop)

```bash
workgraph init ~/work && cd ~/work
git init && git add -A && git commit -m init      # workgraph commits mutations after this

workgraph new project "Hermes" --target-repo git@github.com:moul/hermes.git
workgraph new task "Add run summary" --project hermes --ready
workgraph ready                       # the daily command: next actionable items
workgraph attention                   # where you must intervene
workgraph show add-run-summary        # resolve by id, slug, or fragment
workgraph history ITM-01K...          # work-round timeline
workgraph validate                    # deterministic checks (exit 1 on error)
```

### 2. CLI — agent (bootstrap + report back)

An agent starts a work round, reads the generated capsule in the target repo,
does the task, and reports back — all through the same binary:

```bash
# coordinator: start a round; --print emits the exact launch prompt
workgraph run ITM-01K... --repo ../hermes --agent claude --print --actor agent:claude
#   -> writes ../hermes/.workgraph/runs/RUN-.../ (PROMPT.md, TASK.md, ...)
#   -> prints: "Read .workgraph/runs/RUN-.../PROMPT.md, then do the task. Preserve this repo's CLAUDE.md rules."

# agent, when done:
workgraph finish RUN-01K... --status review --summary .workgraph/runs/RUN-.../RESULT.md --pr 123
# or if stuck:
workgraph block  RUN-01K... --reason "Need production API token"
```

Agents choose work from compact JSON, never by reading the whole repo:

```bash
workgraph ready --json
workgraph list --status ready --json
```

### 3. HTTP API — human or agent

Start the gateway and mint a scoped token, then use plain `curl` (human) or any
HTTP client (agent). Worker identity comes from the token, never the request.

```bash
workgraph serve --addr :8080 --bootstrap-admin-token          # prints a one-time admin token
workgraph token create --kind run --run RUN-01K... \
  --scope runs:context,runs:event,runs:finish --worker agent:claude
#   -> token created (shown once): wg_tok_...

TOK=wg_tok_...
curl -H "Authorization: Bearer $TOK" http://localhost:8080/api/v0/items
curl -H "Authorization: Bearer $TOK" http://localhost:8080/api/v0/runs/RUN-01K.../context
curl -H "Authorization: Bearer $TOK" -X POST \
  -d '{"Status":"review","Summary":"Opened PR","PR":"github:moul/hermes#123"}' \
  http://localhost:8080/api/v0/runs/RUN-01K.../finish
```

A cloud agent that can only read a URL gets scoped instructions at
`http://localhost:8080/t/{token}`. Full reference: [`docs/api.md`](docs/api.md).

### 4. MCP — agent

Local stdio server (register once with your agent):

```bash
workgraph mcp install claude          # prints: claude mcp add workgraph -- <bin> mcp
workgraph mcp install codex
```

Remote MCP over the gateway (for cloud agents), from `/setup/mcp?token=...`:

```bash
claude mcp add --transport http workgraph https://host/mcp \
  --header "Authorization: Bearer wg_tok_..."
```

Tools (compact by default): `init · list_items · get_item · create_item ·
create_run · get_run_context · append_run_event · finish_run · block_run ·
search`. Full reference: [`docs/mcp.md`](docs/mcp.md).

### Onboarding an existing repo

```bash
workgraph discover --repo ../hermes                       # non-invasive: what could be imported?
workgraph import github --repo moul/hermes --issues open  # issues -> triage items (idempotent)
workgraph import markdown ../hermes/TODO.md --state inbox
```

### Read-only web view

```bash
workgraph ui --serve                  # live read-only projection at :8081
workgraph ui --static --out ./site    # self-contained HTML from indexes/*.jsonl
```

> Screenshots of the web interface will land here once the write UI ships
> (tracked in [#10](https://github.com/moul/workgraph/issues/10) /
> [#12](https://github.com/moul/workgraph/issues/12)).

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
workgraph import github --repo moul/hermes --issues open   # existing-project onboarding
workgraph discover --repo ../hermes                        # non-invasive: what could be imported?
```

The gateway exposes the same core mutation package as the CLI over HTTPS + remote
MCP + a tokenized `/t/{token}` page for zero-install cloud agents. No UI-only,
MCP-only, or CLI-only write path.

## Documentation

- [`docs/getting-started.md`](docs/getting-started.md) — **run your own private instance** (start here).
- [`docs/spec.md`](docs/spec.md) — the file format and protocol specification.
- [`docs/api.md`](docs/api.md) — the HTTP gateway API.
- [`docs/mcp.md`](docs/mcp.md) — the MCP surface.
- [`AGENTS.md`](AGENTS.md) / [`CLAUDE.md`](CLAUDE.md) — the agent operating
  contract. Claude Code reads `CLAUDE.md`; Codex reads `AGENTS.md`. `workgraph
  init` scaffolds **both** into your control repo with the same content so any
  agent operating it is covered.
- [`examples/basic`](examples/basic) — a runnable example workspace.

## License

Apache-2.0 OR MIT (see [`LICENSE`](LICENSE)).
