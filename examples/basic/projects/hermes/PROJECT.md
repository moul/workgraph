---
id: PRJ-01K4A2D9Q9N7H2EA2A0P5X0T00
type: project
title: Hermes
status: active
owner: worker:moul
target_repo: git@github.com:moul/hermes.git
target_ref: main
health: at_risk
progress: 28
created_at: 2026-08-04T21:00:00+02:00
updated_at: 2026-08-04T21:30:00+02:00
tags:
  - agents
  - reporting
---

# Hermes

## Purpose

Coordinate several coding agents as a coherent execution system.

## Current outcome

Deliver a local prototype capable of dispatching and supervising three agent
jobs and reporting their results back to the work graph.

## Current state

The execution model is defined. Run reporting and the recovery model remain
open.

## Success criteria

- One-command local installation.
- Jobs survive orchestrator restarts.
- A human can understand every active job.
- Agents can create, claim, update, and complete work items.

## Constraints

- Local-first.
- Git-versioned configuration.
- CLI-first credentials.
