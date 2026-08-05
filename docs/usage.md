# Usage — CLI / HTTP API / MCP

Workgraph has one core mutation path exposed three ways, each usable by a human
or an agent. The recommended way to get started is to let your agent follow
[`llms.txt`](https://moul.github.io/workgraph/llms.txt); the commands below are
the manual equivalents.

## CLI — human (the daily loop)

```bash
cd ~/p/workgraph-state
workgraph new project "Hermes" --target-repo git@github.com:moul/hermes.git
workgraph new task "Add run summary" --project hermes --ready
workgraph ready                       # the daily command: next actionable items
workgraph attention                   # where you must intervene
workgraph show add-run-summary        # resolve by id, slug, or fragment
workgraph history ITM-01K...          # work-round timeline
workgraph validate                    # deterministic checks (exit 1 on error)
```

## CLI — agent (bootstrap + report back)

```bash
# coordinator: start a round; --print emits the exact launch prompt
workgraph run ITM-01K... --repo ../hermes --agent claude --print --actor agent:claude
#   -> writes ../hermes/.workgraph/runs/RUN-.../ (PROMPT.md, TASK.md, ...)

# agent, when done (the capsule's RUN.json has the exact command, -C-pointed at the control repo):
workgraph -C ~/p/workgraph-state finish RUN-01K... --status review \
  --summary .workgraph/runs/RUN-01K.../RESULT.md --pr 123
workgraph -C ~/p/workgraph-state block  RUN-01K... --reason "Need production API token"
```

Agents choose work from compact JSON, never by reading the whole repo:

```bash
workgraph ready --json
workgraph list --status ready --json
```

## HTTP API — human or agent

```bash
workgraph serve --addr :8080 --bootstrap-admin-token          # prints a one-time admin token
workgraph token create --kind run --run RUN-01K... \
  --scope runs:context,runs:event,runs:finish --worker agent:claude

TOK=wg_tok_...
curl -H "Authorization: Bearer $TOK" http://localhost:8080/api/v0/items
curl -H "Authorization: Bearer $TOK" http://localhost:8080/api/v0/runs/RUN-01K.../context
curl -H "Authorization: Bearer $TOK" -X POST \
  -d '{"Status":"review","Summary":"Opened PR","PR":"github:moul/hermes#123"}' \
  http://localhost:8080/api/v0/runs/RUN-01K.../finish
```

A cloud agent that can only read a URL gets scoped instructions at
`http://localhost:8080/t/{token}`. Full reference: [`api.md`](api.md).

## MCP — agent

```bash
workgraph mcp install claude          # local stdio server
claude mcp add --transport http workgraph https://host/mcp \
  --header "Authorization: Bearer wg_tok_..."   # remote, via the gateway
```

Tools (compact by default): `init · list_items · get_item · create_item ·
create_run · get_run_context · append_run_event · finish_run · block_run ·
search`. Full reference: [`mcp.md`](mcp.md).

## Onboarding an existing repo

```bash
workgraph discover --repo ../hermes                       # non-invasive: what could be imported?
workgraph import github --repo moul/hermes --issues open  # issues -> triage items (idempotent)
workgraph import markdown ../hermes/TODO.md --state inbox
```

## Web interface

```bash
workgraph ui --serve                  # live read-only dashboard at :8081
workgraph ui --static --out ./site    # self-contained HTML from indexes/*.jsonl
```
