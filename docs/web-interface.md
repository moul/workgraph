# Web interface

A self-contained, dependency-free **read-only dashboard** rendered from
`indexes/*.jsonl`. Like everything else, it's a projection — the Markdown files
remain the source of truth, and this view never writes.

<!-- screenshot: media/web.png — coming soon -->
_A screenshot will live here._

## Run it

```bash
workgraph ui --serve                  # live dashboard at :8081 (rebuilt per request)
workgraph ui --static --out ./site    # self-contained index.html, no server
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

## Roadmap

Today the interface is read-only. A **writeable** local UI — claim, status
changes, and finish/block from the browser through the same core mutation path,
with the object version shown and stale writes refused — is tracked in
[#10](https://github.com/moul/workgraph/issues/10).
