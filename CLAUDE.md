# CLAUDE.md

This is the **Workgraph tool repo** (Go) — the reference implementation, not a
control repo. Your tasks don't live here; the code does.

## Developing the tool

- `make test` runs the whole suite; `make install` builds the binary (version is
  stamped from git).
- Keep it dependency-light: stdlib plus `gopkg.in/yaml.v3` and `oklog/ulid`. No
  cgo.
- Conventional, single-line commits (`feat:` / `fix:` / `docs:` / `test:`). Do
  not add Claude co-authoring lines.
- The shared mutation path is `internal/core` — CLI, MCP, and the HTTP gateway
  all go through it. There is no UI-only, MCP-only, or CLI-only write path.

## The contract this tool implements

Claude Code reads this file; Codex and the broader convention read `AGENTS.md`.
The agent operating contract (how an agent uses a Workgraph *control* repo) is
kept in `AGENTS.md` and imported here so both are covered:

@AGENTS.md
