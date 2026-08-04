---
id: worker:claude
type: worker
title: claude
status: active
kind: agent
capabilities:
  - create_item
  - update_item
  - claim_item
  - finish_run
  - block_run
requires_review_for:
  - archive_item
  - edit_decision
concurrency_limit: 3
created_at: 2026-08-04T21:00:00+02:00
updated_at: 2026-08-04T21:00:00+02:00
---

# claude

Coding agent worker.
