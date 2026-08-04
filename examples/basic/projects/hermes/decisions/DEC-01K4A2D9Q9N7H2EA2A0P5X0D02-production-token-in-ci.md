---
id: DEC-01K4A2D9Q9N7H2EA2A0P5X0D02
type: decision
title: Whether to allow a production API token in CI
status: proposed
project: PRJ-01K4A2D9Q9N7H2EA2A0P5X0T00
created_at: 2026-08-04T21:40:00+02:00
updated_at: 2026-08-04T21:40:00+02:00
---

# Whether to allow a production API token in CI

## Context

The flaky dispatcher test needs a real API token to reproduce reliably. A
proposed decision, so it surfaces as attention rather than a hard constraint.

## Decision

Proposed: mint a scoped, short-lived CI token stored as a secret name only —
never the value — in the target repo runtime config.
