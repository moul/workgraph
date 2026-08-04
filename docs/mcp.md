# Workgraph MCP surface

MCP is a compact API over the same core, not a richer protocol. The same
JSON-RPC 2.0 handler serves the remote HTTP endpoint (`POST /mcp`) and the local
stdio server (`workgraph mcp`).

## Install

Local stdio server:

```bash
workgraph mcp install claude    # prints: claude mcp add workgraph -- <bin> mcp
workgraph mcp install codex
```

Remote (via the gateway) — see `/setup/mcp?token=...`:

```bash
claude mcp add --transport http workgraph https://host/mcp \
  --header "Authorization: Bearer wg_tok_..."
```

## Tools

Fewer than ten, by design — too many tools hurt tool selection.

```text
init             workspace info + first-call hint
list_items       compact list; filter status/project
get_item         one item; include_body for full markdown
create_item      create (defaults to triage)
create_run       start a work round against an item
get_run_context  task-scoped context packet for a run
append_run_event append an operational event
finish_run       finish a run, set resulting status
block_run        block a run with a reason
search           substring search over items
```

Tool results are compact by default. Full bodies require `include_body: true`.
Worker identity comes from the authenticating token; in stdio mode it comes from
`--actor`.

## Resources

```text
workgraph://indexes/objects      the objects index (jsonl)
workgraph://indexes/attention    the attention queue (jsonl)
```

## Example (stdio)

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_items","arguments":{"status":"ready"}}}' \
  | workgraph mcp
```
