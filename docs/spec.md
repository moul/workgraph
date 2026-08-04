# Workgraph file-format & protocol specification (v0.1)

Workgraph's source of truth is a Git repository of Markdown + YAML + JSONL.
Everything else is a projection. This document is the contract.

## Planes

```text
source plane:   Markdown objects + events JSONL   (authoritative)
index plane:    deterministic JSONL indexes + optional SQLite cache
access plane:   HTTPS API + remote MCP + tokenized read page
adapter plane:  launch capsules + optional CLAUDE/AGENTS helpers
```

Only the source plane is authoritative. If current state (frontmatter) and event
history disagree, **frontmatter wins** and validation reports the drift. Events
are never replayed blindly to reconstruct truth.

## Repository contract

```text
workgraph.yaml              # workspace identity and defaults
ontologies/workgraph.yaml   # the ontology manifest (validation contract)
projects/<slug>/PROJECT.md  # canonical project objects
projects/<slug>/items/*.md  # canonical item objects
projects/<slug>/decisions/*.md
inbox/*.md                  # project-less inbox items
workers/*.md                # worker profiles
events/YYYY-MM.jsonl        # append-only semantic event log
indexes/*.jsonl             # deterministic generated compact indexes
.workgraph/                 # ignored cache + local runtime state (tokens, sqlite)
```

## Identity

An ID is `<PREFIX>-<ULID>` (`ITM-01K4A2...`), immutable, creation-time-sortable,
safe to mint concurrently. Prefixes: `PRJ`, `ITM`, `DEC`, `RUN`, `EVT`, `TOK`,
`WKG`. Workers use the readable `worker:<slug>` form. Prefixes are cosmetic —
parsers must not trust them for dispatch beyond a hint.

Invariants:

```text
filename starts with the id           (except PROJECT.md and workers/<slug>.md)
frontmatter id matches filename id
links use ids, never paths
indexes include both id and path
```

The CLI resolves references by full id, case-insensitive id fragment, filename
slug, title slug, or project directory name — but always writes and prints the
full id.

## Objects

Four types: `project`, `item`, `decision`, `worker`. Required fields on every
object:

```yaml
id: ITM-01K...
type: item
title: Human title
status: ready
created_at: 2026-08-04T21:30:00+02:00
updated_at: 2026-08-04T21:30:00+02:00
```

Object **version** is derived, never stored: it is the Git blob id of the file
(`blob:<sha1>`, identical to `git hash-object`), emitted into indexes and run
capsules. Do not put `version` in frontmatter.

Items add `kind`, `project`, `priority`, `owner`, `depends_on`, `blocked_by`,
`target_repo`, `parallel_policy`, and more (all optional, all flat). Unknown
frontmatter keys are preserved through a rewrite, never dropped.

Body sections are canonical extraction points for capsules and summaries:

```text
## Goal    ## Outcome    ## Context    ## Acceptance criteria    ## Constraints
```

## Status vocabulary (tiny by design)

```text
inbox  triage  ready  in_progress  blocked  review  done  cancelled  archived
```

Extra status names are a tax on every agent. Nuance lives in derived attention
reasons and events, not in new statuses.

## Rounds and events

```text
item  = the durable issue/problem
round = one worker's bounded attempt to move the item forward (RUN-...)
event = one immutable fact inside or around that attempt
```

An item can have many rounds without spawning new items. A `single`
`parallel_policy` item must not have two active owners; the later claimant fails
expected-version or commits to a conflict branch.

Events are append-only JSONL facts:

```json
{"id":"EVT-...","at":"...","actor":"agent:claude","action":"run.finished","object":"ITM-...","run":"RUN-...","status":"review","summary":"Opened PR #123."}
```

Actions come from the ontology (`item.*`, `run.*`, `decision.*`, `project.*`,
`token.*`). Unknown actions are validation errors.

## Indexes

Deterministic, diff-friendly, always rebuildable, committed by default:

```text
indexes/objects.jsonl     # one compact line per object, with version + summary
indexes/links.jsonl       # {from, rel, to}
indexes/runs.jsonl        # one line per work round
indexes/attention.jsonl   # derived human-attention queue
```

The validator warns when a committed index is stale (`workgraph index` fixes
it). The human-debuggable bar:

```bash
jq -r 'select(.status=="ready") | [.id,.title,.target_repo] | @tsv' indexes/objects.jsonl
```

## Attention (mostly derived)

Rules produce attention so stale manual flags cannot rot:

```text
blocked_without_blocked_by   review_assigned_to_human   missing_dependency
lease_expired                new_triage_item            proposed_decision
blocked_by_human             manual_override (honored until attention_until)
```

## Launch capsules

A run capsule is a task-scoped contract copied into the target repo at
`.workgraph/runs/RUN-.../`:

```text
RUN.json  PROMPT.md  TASK.md  PROJECT.md  LINKS.md  RULES.md  RESULT.md
```

Budgets keep it readable: `PROMPT.md ≤ 200` lines, `TASK.md ≤ 300`,
`PROJECT.md ≤ 150`, `LINKS.md ≤ 100`. The capsule never copies the target repo's
own CLAUDE.md/AGENTS.md — those say *how* to work there; the capsule says *what*
and *why*.

## Git behavior

Every mutating command: fetch → require fast-forward (unless `--offline` or
`--branch-on-conflict`) → verify expected version → write object + event →
rebuild committed indexes → commit → push. No silent last-writer-wins.

## Validation (stricter than search)

`workgraph validate` detects duplicate IDs, missing required fields, unknown
vocabulary (via the ontology), broken references, dependency and parent cycles,
filename/id mismatches, malformed YAML, invalid dates, expired leases, stale
indexes, and obvious secrets. Errors block; warnings inform.

## Permissions

Worker profiles declare `capabilities` and `requires_review_for`. Enforcement is
CLI-level in v0 — the point is to make dangerous operations visible and testable.
Trust remains Git-based: a human can review every commit. Agent-created items
default to `triage` unless created as a child of an active item with `--ready`.

## Secrets

The control repo may be pushed to GitHub. Secret *values* never belong in source
or capsules; secret *names*, env var names, and vault paths are fine. Validation
includes a lightweight secret scan.
