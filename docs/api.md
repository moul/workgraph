# Workgraph HTTP gateway API (v0)

The gateway is a thin, Git-backed service served by the same `workgraph` binary.
It owns the annoying parts (token auth, expected-version checks, context packet
generation, event append) but never the source of truth — the Git repo does.

```bash
workgraph serve --addr :8080 --repo /srv/workgraph/state --bootstrap-admin-token
```

Every write goes through the same core mutation package as the CLI. There is no
UI-only, MCP-only, or CLI-only write path.

## Auth

All `/api/v0/*` routes require a scoped bearer token:

```http
Authorization: Bearer wg_tok_...
```

Worker identity comes from the authenticated token, never a caller-supplied
field. A run-scoped token may only touch its own run. Token values are shown once
and only hashes are stored (never in Git).

## Endpoints

```text
GET  /api/v0/items?status=ready&project=PRJ-...     items:read
POST /api/v0/items                                  items:create
       body: {"Title","Project","Kind","Ready"}
GET  /api/v0/items/{id}?include_body=true           items:read
POST /api/v0/items/{id}/runs                        runs:create
GET  /api/v0/runs/{id}/context                      runs:context
POST /api/v0/runs/{id}/events                       runs:event
       body: {"Action","Message"}
POST /api/v0/runs/{id}/finish                       runs:finish
       body: {"Status","Summary","PR"}
POST /api/v0/runs/{id}/block                        runs:block
       body: {"Reason"}
GET  /api/v0/search?q=...                            items:read
```

Pages (no auth): `/`, `/docs/api`, `/docs/mcp`, `/docs/subagents`, `/setup/mcp`.
Tokenized page: `/t/{token}`. MCP: `POST /mcp`. Admin: `POST /admin/tokens`.

## Examples

```bash
curl -H "Authorization: Bearer $TOK" http://localhost:8080/api/v0/items

curl -H "Authorization: Bearer $TOK" \
  http://localhost:8080/api/v0/runs/RUN-01K.../context

curl -H "Authorization: Bearer $TOK" -X POST \
  -d '{"Status":"review","Summary":"Opened PR","PR":"github:moul/hermes#123"}' \
  http://localhost:8080/api/v0/runs/RUN-01K.../finish
```

## Tokens

Mint from the CLI or the admin endpoint. Kinds map to default TTLs:

```text
run token   24h    scoped to one run/context/update flow
item token   7d    scoped to one item
workspace    —     broad coordinator access, explicit expiry required
```

```bash
workgraph token create --kind run --run RUN-01K... \
  --scope runs:context,runs:event,runs:finish --worker agent:claude
workgraph token list
workgraph token revoke TOK-01K...
```

```bash
curl -H "Authorization: Bearer $ADMIN" -X POST \
  -d '{"kind":"run","run":"RUN-01K...","scopes":["runs:context","runs:event","runs:finish"],"worker":"agent:claude"}' \
  http://localhost:8080/admin/tokens
```

The response includes `token` (shown once) and a copy-paste `url`
(`/t/{token}`).

## Security properties

```text
expired token is rejected          revoked token fails immediately
read token cannot append event     run token cannot touch another run
token value is never stored in Git  every token has scopes + expiry
worker identity comes from the token, not the request body
```
