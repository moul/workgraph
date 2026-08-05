# Workgraph

> A Git-native work graph for humans and agents. Durable tasks, decisions, and
> cross-repo agent handoff — as plain Markdown + JSON in a Git repo, no daemon.

Workgraph is **not where agents think**. It is **where humans and agents agree on
durable work state**.

## Getting started

> Hey Claude, let's start using https://moul.github.io/workgraph/llms.txt

<details>
<summary><b>Advanced usages</b></summary>

By default — following [`llms.txt`](https://moul.github.io/workgraph/llms.txt) —
your agent installs the CLI, creates a **private, git-backed control repo**, and
drives it through the CLI. That's the recommended path. Reach for these only if
you want a specific setup.

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

From then on every change auto-commits and pushes there. **No GitHub?**
`workgraph init` already runs `git init` — commit once and work locally.

**Use it directly.** One core mutation path, three surfaces — each for a human or
an agent (copy-paste examples: [`docs/usage.md`](docs/usage.md)):

- **CLI** — `workgraph ready`, `run`, `finish` (the daily loop and agent handoff)
- **HTTP API** — scoped-token gateway for cloud agents and mobile coordinators
- **MCP** — local stdio + remote, a compact tool surface

Full walkthrough (tool vs control vs target repos, self-hosting, multi-machine):
[`docs/getting-started.md`](docs/getting-started.md).

</details>

## Web interface

```bash
workgraph ui --serve      # live read-only dashboard
```

<!-- screenshot: docs/media/web.png — coming soon -->
_A screenshot of the web interface will live here._

## Docs

- [How it works](docs/how-it-works.md) — the model in five minutes.
- **Agents:** [`llms.txt`](https://moul.github.io/workgraph/llms.txt) — the entry point.
- [Getting started](docs/getting-started.md) · [Usage](docs/usage.md) · [Spec](docs/spec.md) · [HTTP API](docs/api.md) · [MCP](docs/mcp.md)
- [`AGENTS.md`](AGENTS.md) / [`CLAUDE.md`](CLAUDE.md) — the operating contract `workgraph init` scaffolds into every control repo.

## License

Apache-2.0 OR MIT (see [`LICENSE`](LICENSE)).
