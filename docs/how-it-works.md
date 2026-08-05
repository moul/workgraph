# How it works

The source of truth is plain **Markdown + YAML frontmatter + append-only JSONL
events** in a Git repository. The CLI, the JSONL indexes, the MCP server, and the
HTTP gateway are all **projections** over those files — when tools fail, a tired
human on SSH still has `ls`, `git diff`, and `jq`.

```text
source plane:   Markdown objects + events JSONL   (authoritative)
index plane:    deterministic JSONL indexes + optional SQLite cache
access plane:   HTTPS API + remote MCP + tokenized read page
adapter plane:  launch capsules + optional CLAUDE/AGENTS helpers
```

## Three kinds of repo, never mixed

```text
moul/workgraph        the tool + reference      (install the binary from it)
your control repo     your durable work state   (private; workgraph init creates it)
target repos          the code you change       (capsules only, never rewritten)
```

- **The tool** (`moul/workgraph`) is the reference: source, tests, docs, example.
- **Your control repo** is a separate, private repo holding your projects, items,
  decisions, and event log. Every mutation commits and pushes to it.
- **Target repos** are the codebases you work on. Workgraph never rewrites their
  `CLAUDE.md`/`AGENTS.md`; it only drops a task-scoped run capsule under
  `.workgraph/runs/` when you launch a run there.

## Object model

Four object types: `project` · `item` · `decision` · `worker`.

- An **item** is the durable issue/problem — often equivalent to a GitHub issue.
- A **round** (`RUN-...`) is one worker's bounded attempt to move an item forward.
  An item can have many rounds without spawning new items.
- An **event** is one immutable fact. Frontmatter is *current state*; events
  explain *how it got there* (never replayed blindly).

IDs are immutable prefixed ULIDs (`ITM-01K…`); filenames and paths are ergonomic
aliases (the filename must start with the ID). Object *version* is derived — the
Git blob hash — never stored in frontmatter.

## One tiny status vocabulary

```text
inbox  triage  ready  in_progress  blocked  review  done  cancelled  archived
```

Extra status names are a tax on every agent. Nuance lives in derived **attention
reasons** and events, not in new statuses. Attention is mostly *derived* (blocked
without a blocker, review awaiting a human, stale lease, missing dependency,
proposed decision, …) so manual flags can't rot.

## An ontology agents can't invent around

`ontologies/workgraph.yaml` is the deterministic contract: the allowed object
types, item kinds, statuses, relation types, event actions, attention reasons,
capabilities, and token scopes. `workgraph validate` rejects anything outside it
— stricter than the indexer, so nobody comes to rely on forgiving cached
behavior the files don't actually encode.

For the full file format and protocol, see [`spec.md`](spec.md).
