# Getting started — run your own private instance

## Mental model: three kinds of repo

Workgraph deliberately keeps **the tool**, **your work graph**, and **the code
you work on** in separate repositories. Don't put them in one place.

```text
┌─────────────────────────┐   go install     ┌──────────────────────────┐
│  TOOL REPO (this one)    │ ───────────────▶ │  workgraph binary on PATH │
│  github.com/moul/workgraph                   └──────────────────────────┘
│  the Go code + reference │                              │ operates on
└─────────────────────────┘                              ▼
                                   ┌───────────────────────────────────────┐
                                   │  YOUR CONTROL REPO (private, separate) │
                                   │  e.g. github.com/moul/workgraph-state  │
                                   │  workgraph.yaml, projects/, events/... │
                                   │  ← this is *your* durable work state   │
                                   └───────────────────────────────────────┘
                                                          │ launches runs into
                                                          ▼
                                   ┌───────────────────────────────────────┐
                                   │  TARGET REPOS (the code you change)    │
                                   │  github.com/moul/hermes, ...           │
                                   │  Workgraph only writes a run capsule   │
                                   │  under .workgraph/runs/ here.          │
                                   └───────────────────────────────────────┘
```

- **`moul/workgraph`** (this repo) is the **reference**: the source, the tests,
  the docs, the example workspace. You install the binary from it. You do *not*
  put your tasks here.
- **Your control repo** is a **separate, private repo** you own. It holds your
  actual projects, items, decisions, and event log. `workgraph init` creates it;
  every mutation commits and pushes to it.
- **Target repos** are the codebases you actually work on. Workgraph never
  rewrites their `CLAUDE.md`/`AGENTS.md`; it only drops a task-scoped run capsule
  under `.workgraph/runs/` when you launch a run there.

## 1. Install the tool

Requires **Go 1.23+** and **git**.

```bash
go install github.com/moul/workgraph/cmd/workgraph@latest
# or from a clone of this repo:  make install
workgraph version
```

Ensure `$(go env GOPATH)/bin` is on your `PATH`. If this repo is private to you,
set `GOPRIVATE=github.com/moul/*` with git SSH/token auth, or install from a
local clone.

## 2. Create your private instance (recommended)

Initialize a control repo locally, then create the private GitHub repo from it
in one step with `gh`:

```bash
workgraph init ~/p/workgraph-state
cd ~/p/workgraph-state
git add -A && git commit -m "init workgraph workspace"

gh repo create moul/workgraph-state --private --source=. --remote=origin --push
```

That's it — you now have a private control repo. From here, every `workgraph`
mutation auto-commits **and pushes** to it:

```bash
workgraph new project "My Stuff" --target-repo git@github.com:moul/hermes.git
workgraph new task "Try workgraph" --project my-stuff --ready
workgraph ready
```

Check the remote received the commits:

```bash
git log --oneline        # init + one commit per mutation
```

### Alternative: start from an empty GitHub repo

If you'd rather create the repo first:

```bash
gh repo create moul/workgraph-state --private
git clone git@github.com:moul/workgraph-state ~/p/workgraph-state
cd ~/p/workgraph-state
git checkout -B main                 # ensure the branch exists
workgraph init .                      # detects the existing git repo, skips git init
git add -A && git commit -m "init workgraph workspace" && git push -u origin main
```

## 3. Local-only (no GitHub at all)

Workgraph needs no remote. `workgraph init` runs `git init` for you; mutations
commit locally. Add a remote whenever you want.

```bash
workgraph init ~/p/workgraph-state
cd ~/p/workgraph-state
git add -A && git commit -m init      # first commit; mutations commit after this
workgraph new project "Solo" 
```

To avoid pushing when you have no remote yet, pass `--no-push` (or `--offline`)
on mutations, or just add a remote later.

## 4. Daily use

```bash
cd ~/p/workgraph-state
workgraph ready                       # next actionable items (the daily command)
workgraph attention                   # where you must intervene
workgraph show <id|slug|fragment>     # inspect one object
workgraph run <id> --repo ../hermes --agent claude --print   # start a round + capsule
workgraph finish RUN-... --status review --pr 123
workgraph validate                    # deterministic checks
```

You can run `workgraph` from anywhere with `-C ~/p/workgraph-state`, or set
`WORKGRAPH_DIR=~/p/workgraph-state` in your shell.

## 5. Serve your private instance (for mobile / cloud agents)

Run the gateway against your control repo so cloud agents (Claude on the web,
mobile coordinators) can reach it over HTTPS/MCP without cloning anything:

```bash
cd ~/p/workgraph-state
workgraph serve --addr :8080 --base-url https://wg.example.com --bootstrap-admin-token
```

Then mint a scoped, short-lived token for one task and share its URL:

```bash
workgraph token create --kind run --run RUN-... \
  --scope runs:context,runs:event,runs:finish --worker agent:claude
# -> paste http://<host>/t/wg_tok_... into the agent
```

Put it behind TLS (a reverse proxy or Cloudflare Tunnel) before exposing it.
Tokens are hashed at rest and never stored in Git. See
[`docs/api.md`](api.md) and [`docs/mcp.md`](mcp.md).

## 6. Multi-machine reality

If you clone your control repo onto several machines, assume clones go stale.
Every mutating command fetches first and requires a fast-forward. On divergence
it refuses with a repair hint; pass `--branch-on-conflict` to write to a
`workgraph/conflict/<id>` branch instead of overwriting, or `--offline` to defer
the sync. Nothing is ever last-writer-wins.

## 7. Adopt an existing project

```bash
workgraph discover --repo ../hermes                       # non-invasive survey
workgraph import github --repo moul/hermes --issues open  # issues -> triage items
```

Imports are idempotent by `source_ref` and never mark work `ready` for you.
