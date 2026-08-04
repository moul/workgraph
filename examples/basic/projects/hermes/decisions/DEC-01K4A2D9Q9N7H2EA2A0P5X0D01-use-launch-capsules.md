---
id: DEC-01K4A2D9Q9N7H2EA2A0P5X0D01
type: decision
title: Use launch capsules instead of mutating AGENTS.md
status: accepted
project: PRJ-01K4A2D9Q9N7H2EA2A0P5X0T00
created_at: 2026-08-04T21:00:00+02:00
updated_at: 2026-08-04T21:00:00+02:00
---

# Use launch capsules instead of mutating AGENTS.md

## Context

Target repos already have their own CLAUDE.md / AGENTS.md. Rewriting them per
task is risky and leaks context.

## Decision

Generate a task-scoped launch capsule under `.workgraph/runs/RUN-.../` and point
the agent at it. Never mutate the target repo's root instruction files.

## Consequences

Agents use the target repo's native instructions for *how* to work, and the
capsule for *what* and *why*.
