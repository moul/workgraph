# Workgraph

> A Git-native work graph for humans and agents. Durable tasks, decisions, and
> cross-repo agent handoff — as plain Markdown + JSON in a Git repo, no daemon.

Workgraph is **not where agents think**. It is **where humans and agents agree on
durable work state**.

## Getting started

Paste this to your coding agent:

```text
Hey Claude, let's start using https://moul.github.io/workgraph/llms.txt
```

It installs the CLI, creates your private git-backed control repo, and starts
using it. Prefer to drive it yourself? The docs below cover every path.

## Docs

- **[How it works](docs/how-it-works.md)** — the model in five minutes (start here).
- **[Getting started](docs/getting-started.md)** — run your own private instance, step by step.
- **[Usage](docs/usage.md)** — CLI / HTTP API / MCP, copy-paste for humans and agents.
- **[Web interface](docs/web-interface.md)** — the read-only dashboard (`workgraph ui --serve`).
- **[Spec](docs/spec.md)** — the file format and protocol.
- **[HTTP API](docs/api.md)** · **[MCP](docs/mcp.md)** — the gateway surfaces in detail.
- **Agents:** [`llms.txt`](https://moul.github.io/workgraph/llms.txt) — the entry point.
- [`AGENTS.md`](AGENTS.md) / [`CLAUDE.md`](CLAUDE.md) — the operating contract `workgraph init` scaffolds into every control repo.

Jump straight to the common tasks:

- [Install](docs/getting-started.md#install) · [Run with Docker, no Go](docs/getting-started.md#run-with-docker-no-go)
- [Create your private control repo](docs/getting-started.md#create-your-private-control-repo)
- [Serve for cloud & mobile agents](docs/getting-started.md#serve-for-cloud--mobile-agents)
- [Adopt an existing project](docs/getting-started.md#adopt-an-existing-project)

## License

Apache-2.0 OR MIT (see [`LICENSE`](LICENSE)).
