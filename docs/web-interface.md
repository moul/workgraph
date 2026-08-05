# Web interface

A self-contained, dependency-free **read-only dashboard** rendered from
`indexes/*.jsonl`. Like everything else, it's a projection — the Markdown files
remain the source of truth, and this view never writes.

<!-- screenshot: media/web.png — coming soon -->
_A screenshot will live here._

## Run it

```bash
workgraph ui --serve                  # live read-only dashboard at :8081 (rebuilt per request)
workgraph ui --serve --write          # writeable: status changes from the browser (binds to localhost)
workgraph ui --static --out ./site    # folder: index.html + data/*.json + assets/*.svg (for CI -> Pages)
```

Or reach it on a running gateway at **`/ui`**:

```bash
workgraph serve --addr :8080
# open http://localhost:8080/ui
```

## What it shows

- **Needs attention** — the derived queue, as severity-colored cards. This is the
  point of the product: where a human must intervene.
- **Projects** — cards with status and health.
- **Items** — a filterable table (by text, status, kind) with colored status
  pills, priority, owner, and target repo; click a column header to sort.
- **Recent runs** — the latest work rounds with worker, status, and result.

Theme-aware (light/dark), no external requests, safe to open from `file://` or
serve from GitHub Pages.

## Writeable mode

`workgraph ui --serve --write` adds an inline status control to each item. Changes
go through the **same core mutation path as the CLI** (an event is written, the
object committed to Git) and carry the object's version, so a **stale write is
refused with a 409** rather than silently overwriting concurrent edits. Because
it mutates the repo, write mode **binds to localhost** and honors the usual
`--actor` / `--no-push` / `--offline` flags.

Next: richer actions (finish/block a run, edit fields) and live updates over a
WebSocket ([#18](https://github.com/moul/workgraph/issues/18)).
