# Workgraph

> A Git-native work graph for humans and agents. Durable tasks, decisions, and
> cross-repo agent handoff — as plain Markdown + JSON in a Git repo, no daemon.

Workgraph is **not where agents think**. It is **where humans and agents agree on
durable work state**.

## Getting started

The fastest way is to let your coding agent set it up. Tell it:

> **Install `workgraph` and open https://moul.github.io/workgraph/llms.txt, then follow it.**

Your agent installs the CLI, creates your private work repo, and starts using it.
That's the whole onboarding.

<details>
<summary><b>Prefer to do it yourself, or pick a specific install mode?</b></summary>

Workgraph runs in a **folder** — ideally a **git repo**, because it's git-native:
every change is a commit you can review, revert, branch, and sync. `llms.txt` is
the recommended path; below are the manual equivalents if you want a particular
setup your agent's default might not choose.

**Install the CLI** (Go 1.23+):

```bash
go install github.com/moul/workgraph/cmd/workgraph@latest   # from the public module
# or, from a clone: make install   (stamps the version from git)
```

**Create your private control repo** — a *separate* folder where your work lives
(this repo is the tool; your tasks don't go here):

```bash
workgraph init ~/p/workgraph-state
cd ~/p/workgraph-state
git add -A && git commit -m "init workgraph workspace"
gh repo create <you>/workgraph-state --private --source=. --remote=origin --push
```

From then on, every `workgraph` change auto-commits and pushes to your private
repo. **No GitHub?** `workgraph init` already runs `git init` — commit once and
work locally; add a remote whenever you like.

Full walkthrough (tool vs control vs target repos, self-hosting, multi-machine):
[`docs/getting-started.md`](docs/getting-started.md).

</details>

## Web interface

```bash
workgraph ui --serve      # live read-only dashboard
```

<!-- screenshot: docs/media/web.png — coming soon -->
_A screenshot of the web interface will live here._

## How it works

The source of truth is plain Markdown + YAML frontmatter + append-only JSONL
events in a Git repo. The CLI, indexes, MCP server, and HTTP gateway are all
projections over those files — when tools fail, you still have `ls`, `git diff`,
and `jq`.

```text
moul/workgraph        the tool + reference      (this repo; go install)
your control repo     your durable work state   (private; workgraph init)
target repos          the code you change       (capsules only, never rewritten)
```

Four object types (`project` · `item` · `decision` · `worker`), one tiny status
vocabulary (`inbox triage ready in_progress blocked review done cancelled
archived`), and a small ontology that agents can't invent their way around.

## Use it

One core mutation path, three surfaces — each for a human or an agent:

- **CLI** — `workgraph ready`, `run`, `finish` (the daily loop and agent handoff)
- **HTTP API** — scoped-token gateway for cloud agents and mobile coordinators
- **MCP** — local stdio + remote, a compact tool surface

Copy-paste examples for all three: **[`docs/usage.md`](docs/usage.md)**.

## Docs

- **Agents:** [`llms.txt`](https://moul.github.io/workgraph/llms.txt) — the entry point.
- [Getting started](docs/getting-started.md) · [Usage](docs/usage.md) · [Spec](docs/spec.md) · [HTTP API](docs/api.md) · [MCP](docs/mcp.md)
- [`AGENTS.md`](AGENTS.md) / [`CLAUDE.md`](CLAUDE.md) — the operating contract `workgraph init` scaffolds into every control repo.

## License

Apache-2.0 OR MIT (see [`LICENSE`](LICENSE)).
