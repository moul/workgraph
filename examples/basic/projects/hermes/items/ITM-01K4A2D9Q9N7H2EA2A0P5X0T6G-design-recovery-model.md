---
id: ITM-01K4A2D9Q9N7H2EA2A0P5X0T6G
type: item
title: Design the run recovery model
status: ready
kind: task
project: PRJ-01K4A2D9Q9N7H2EA2A0P5X0T00
priority: normal
target_repo: git@github.com:moul/hermes.git
target_ref: main
depends_on:
  - ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F
created_at: 2026-08-04T21:05:00+02:00
updated_at: 2026-08-04T21:05:00+02:00
tags:
  - hermes
---

# Design the run recovery model

## Goal

Define how a run is recovered when the orchestrator restarts mid-work.

## Acceptance criteria

- Leases expire and become an attention reason.
- A restarted orchestrator can resume or reassign a run.

## Constraints

Recovery must be reconstructable from Git + events only.
