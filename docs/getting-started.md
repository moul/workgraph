# Getting started — run your own private instance

## Mental model: three kinds of repo

Workgraph deliberately keeps **the tool**, **your work graph**, and **the code
you work on** in separate repositories. Don't put them in one place.

```text
moul/workgraph        the tool + reference      (install the binary from it)
your control repo     your durable work state   (private; workgraph init creates it)
target repos          the code you change       (capsules only, never rewritten)
```

- **`moul/workgraph`** (this repo) is the **reference**: source, tests, docs,
  example. You install the binary from it — you don't put your tasks here.
- **Your control repo** is a **separate, private** repo holding your projects,
  items, decisions, and event log. `workgraph init` creates it; every mutation
  commits and pushes to it.
- **Target repos** are the codebases you work on. Workgraph never rewrites their
  `CLAUDE.md`/`AGENTS.md`; it only drops a task-scoped run capsule under
  `.workgraph/runs/` when you launch a run there.

## Install

Requires **Go 1.23+** and **git**.

```bash
go install github.com/moul/workgraph/cmd/workgraph@latest
# or from a clone of this repo:  make install   (stamps the version from git)
workgraph version
```

Ensure `$(go env GOPATH)/bin` is on your `PATH`.

## Run with Docker (no Go)

Prefer a container? Mount your control repo as a volume — no Go toolchain needed:

```bash
docker run --rm -v "$PWD":/workspace ghcr.io/moul/workgraph ready
docker run --rm -p 8080:8080 -v "$PWD":/workspace ghcr.io/moul/workgraph serve --addr :8080
```

The image bundles `git`. For mutations, pass your identity so commits are
attributed:

```bash
docker run --rm -v "$PWD":/workspace \
  -e GIT_AUTHOR_NAME=you -e GIT_AUTHOR_EMAIL=you@example.com \
  -e GIT_COMMITTER_NAME=you -e GIT_COMMITTER_EMAIL=you@example.com \
  ghcr.io/moul/workgraph new task "Try it" --project demo --ready
```

## Create your private control repo

Initialize locally, then create the private GitHub repo from it in one step:

```bash
workgraph init ~/p/workgraph-state
cd ~/p/workgraph-state
git add -A && git commit -m "init workgraph workspace"
gh repo create <you>/workgraph-state --private --source=. --remote=origin --push
```

From here, every `workgraph` mutation auto-commits **and pushes** to your private
repo:

```bash
workgraph new project "My Stuff" --target-repo git@github.com:<you>/hermes.git
workgraph new task "Try workgraph" --project my-stuff --ready
workgraph ready
```

### Alternative: start from an empty GitHub repo

```bash
gh repo create <you>/workgraph-state --private
git clone git@github.com:<you>/workgraph-state ~/p/workgraph-state
cd ~/p/workgraph-state
git checkout -B main
workgraph init .                      # detects the existing git repo, skips git init
git add -A && git commit -m "init workgraph workspace" && git push -u origin main
```

## Local-only (no GitHub)

Workgraph needs no remote. `workgraph init` runs `git init` for you; mutations
commit locally. Add a remote whenever you want.

```bash
workgraph init ~/p/workgraph-state
cd ~/p/workgraph-state
git add -A && git commit -m init      # first commit; mutations commit after this
workgraph new project "Solo"
```

Use `--no-push` (or `--offline`) on mutations until you add a remote.

## Daily use

```bash
cd ~/p/workgraph-state
workgraph ready                       # next actionable items (the daily command)
workgraph attention                   # where you must intervene
workgraph show <id|slug|fragment>     # inspect one object
workgraph run <id> --repo ../hermes --agent claude --print   # start a round + capsule
workgraph finish RUN-... --status review --pr 123
workgraph validate                    # deterministic checks
```

Run from anywhere with `-C ~/p/workgraph-state`, or set
`WORKGRAPH_DIR=~/p/workgraph-state`. Full command reference and the HTTP/MCP
surfaces: [usage.md](usage.md).

## Serve for cloud & mobile agents

Run the gateway against your control repo so cloud agents reach it over
HTTPS/MCP without cloning anything:

```bash
cd ~/p/workgraph-state
workgraph serve --addr :8080 --base-url https://wg.example.com --bootstrap-admin-token
workgraph token create --kind run --run RUN-... \
  --scope runs:context,runs:event,runs:finish --worker agent:claude
# -> paste http://<host>/t/wg_tok_... into the agent
```

Put it behind TLS before exposing it. Tokens are hashed at rest and never stored
in Git. See [api.md](api.md) and [mcp.md](mcp.md).

## Multi-machine

Assume clones go stale. Every mutating command fetches first and requires a
fast-forward. On divergence it refuses with a repair hint; pass
`--branch-on-conflict` to write to a `workgraph/conflict/<id>` branch instead of
overwriting, or `--offline` to defer the sync. Never last-writer-wins.

## Adopt an existing project

```bash
workgraph discover --repo ../hermes                       # non-invasive survey
workgraph import github --repo moul/hermes --issues open  # issues -> triage items
```

Imports are idempotent by `source_ref` and never mark work `ready` for you.
