---
id: ITM-01K4A2D9Q9N7H2EA2A0P5X0T6H
type: item
title: Fix flaky dispatcher test
status: blocked
kind: bug
project: PRJ-01K4A2D9Q9N7H2EA2A0P5X0T00
priority: high
target_repo: git@github.com:moul/hermes.git
target_ref: main
blocked_by:
  - DEC-01K4A2D9Q9N7H2EA2A0P5X0D02
created_at: 2026-08-04T21:10:00+02:00
updated_at: 2026-08-04T21:40:00+02:00
tags:
  - hermes
  - flaky
---

# Fix flaky dispatcher test

## Goal

The dispatcher test fails intermittently under load. Make it deterministic.

## Context

Blocked pending a decision on whether to allow a real production API token in
CI, tracked by DEC-...D02.

## Acceptance criteria

- The test passes 100 consecutive runs.
