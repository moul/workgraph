---
id: ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F
type: item
title: Add run summary to Hermes worker output
status: review
kind: task
project: PRJ-01K4A2D9Q9N7H2EA2A0P5X0T00
priority: high
owner: worker:claude
reviewer: worker:moul
target_repo: git@github.com:moul/hermes.git
target_ref: main
depends_on:
  - ITM-01K4A2D9Q9N7H2EA2A0P5X0T6J
created_at: 2026-08-04T21:00:00+02:00
updated_at: 2026-08-04T22:10:00+02:00
tags:
  - hermes
  - reporting
---

# Add run summary to Hermes worker output

## Goal

Every worker run should emit a compact, human-readable summary so a coordinator
can understand the outcome without reading logs.

## Context

Runs currently only leave raw logs. A tired human on SSH cannot tell what a run
accomplished at a glance.

## Acceptance criteria

- Each finished run writes a one-paragraph summary.
- The summary never includes chain-of-thought.
- Tests cover the summary formatter.

## Constraints

Do not introduce an external hosted database.
