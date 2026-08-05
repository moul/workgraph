package gateway

import (
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/moul/workgraph/internal/index"
	"github.com/moul/workgraph/internal/webui"
)

func page(w http.ResponseWriter, title, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title><style>body{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;max-width:52rem;margin:2rem auto;padding:0 1rem;line-height:1.5;color:#111}a{color:#0b5}code,pre{background:#f4f4f4;padding:.1rem .3rem;border-radius:3px}pre{padding:.8rem;overflow-x:auto}h1,h2{line-height:1.2}</style></head><body>%s</body></html>`, html.EscapeString(title), body)
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	page(w, "Workgraph gateway", `
<h1>Workgraph</h1>
<p>A Git-native work graph for humans and agents. This gateway is a thin,
Git-backed service; the source of truth is a Git repository of Markdown + JSONL.</p>
<h2>Access</h2>
<ul>
<li><a href="/ui">Dashboard</a> (read-only)</li>
<li><a href="/docs/api">HTTP API docs</a></li>
<li><a href="/docs/mcp">MCP docs</a></li>
<li><a href="/docs/subagents">Coordinator / subagent workflow</a></li>
<li><a href="/setup/mcp">MCP setup</a></li>
</ul>
<p>You reach the API with a scoped bearer token:
<code>Authorization: Bearer wg_tok_...</code>. Ask a coordinator for a token URL
(<code>/t/{token}</code>).</p>`)
}

func (s *Server) handleTokenPage(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimPrefix(r.URL.Path, "/t/")
	tok := s.Svc.Tokens.Lookup(raw)
	base := s.BaseURL
	if base == "" {
		base = "http://" + r.Host
	}
	if tok == nil {
		w.WriteHeader(http.StatusNotFound)
		page(w, "Unknown token", "<h1>Unknown token</h1><p>This token is not recognized.</p>")
		return
	}
	status := "active"
	if tok.Revoked {
		status = "REVOKED"
	} else if tok.Expired(time.Now()) {
		status = "EXPIRED"
	}
	scopeList := strings.Join(tok.Scopes, ", ")
	ctx := ""
	if tok.Run != "" {
		ctx = fmt.Sprintf("<p>Run context: <code>%s</code>. Fetch it at <code>GET %s/api/v0/runs/%s/context</code>.</p>", html.EscapeString(tok.Run), base, html.EscapeString(tok.Run))
	} else if tok.Item != "" {
		ctx = fmt.Sprintf("<p>Item scope: <code>%s</code>.</p>", html.EscapeString(tok.Item))
	}
	body := fmt.Sprintf(`
<h1>You have Workgraph access</h1>
<pre>Token status: %s
Scopes:       %s
API:          %s/api/v0
MCP:          %s/mcp</pre>
%s
<h2>If you can install/configure MCP</h2>
<p>Open <a href="/setup/mcp?token=%s">/setup/mcp</a>. Connect to <code>%s/mcp</code>
with header <code>Authorization: Bearer %s</code>.</p>
<h2>If you can call HTTP</h2>
<pre>curl -H "Authorization: Bearer %s" %s/api/v0/items</pre>
<h2>If you can run subagents</h2>
<p>See <a href="/docs/subagents">the subagent workflow</a>: fetch the run
context, launch a subagent with it, and post events back.</p>
<h2>If you cannot run subagents</h2>
<p>Do the task yourself using the run context above, then report progress with
<code>POST %s/api/v0/runs/RUN-.../events</code>.</p>
<p><em>This page never reveals more than the token scope allows.</em></p>`,
		status, html.EscapeString(scopeList), base, base, ctx,
		html.EscapeString(raw), base, html.EscapeString(raw),
		html.EscapeString(raw), base, base)
	page(w, "Workgraph token", body)
}

// handleUI serves the read-only dashboard, rebuilt from the index on each
// request so the served view stays fresh.
func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	ws, err := s.Svc.workspace()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	res, err := index.Build(ws, false)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, webui.Render(res))
}

func (s *Server) handleDocsAPI(w http.ResponseWriter, r *http.Request) {
	page(w, "Workgraph API", `
<h1>HTTP API v0</h1>
<p>All routes require <code>Authorization: Bearer wg_tok_...</code>. Machine
reads return JSON.</p>
<pre>GET  /api/v0/items?status=ready&project=PRJ-...
POST /api/v0/items                      {"Title","Project","Kind","Ready"}
GET  /api/v0/items/{id}?include_body=true
POST /api/v0/items/{id}/runs
GET  /api/v0/runs/{id}/context
POST /api/v0/runs/{id}/events           {"Action","Message"}
POST /api/v0/runs/{id}/finish           {"Status","Summary","PR"}
POST /api/v0/runs/{id}/block            {"Reason"}
GET  /api/v0/search?q=...</pre>
<h2>Scopes</h2>
<pre>items:read items:create runs:create runs:context runs:event runs:finish runs:block</pre>
<p>A run-scoped token may only touch its own run. Worker identity comes from the
token, never a caller-supplied field.</p>`)
}

func (s *Server) handleDocsMCP(w http.ResponseWriter, r *http.Request) {
	page(w, "Workgraph MCP", `
<h1>MCP</h1>
<p>Remote MCP endpoint: <code>POST /mcp</code> (JSON-RPC 2.0), authenticated with
<code>Authorization: Bearer wg_tok_...</code>.</p>
<h2>Tools</h2>
<pre>init            list_items       get_item        create_item
create_run      get_run_context  append_run_event
finish_run      block_run        search</pre>
<p>Tool results are compact by default. Full bodies require
<code>include_body: true</code> or <code>context_level: "full"</code>.</p>`)
}

func (s *Server) handleDocsSubagents(w http.ResponseWriter, r *http.Request) {
	page(w, "Workgraph subagents", `
<h1>Coordinator / subagent workflow</h1>
<ol>
<li>Coordinator asks the gateway to create a run for an item.</li>
<li>Gateway claims the item (or opens a conflict branch) and writes
<code>run.created</code>.</li>
<li>Coordinator fetches <code>/runs/{id}/context</code> and launches a subagent
with it.</li>
<li>Subagent works in the target repo using the target repo's own rules.</li>
<li>Subagent reports progress via <code>/runs/{id}/events</code> and completion
via <code>/runs/{id}/finish</code>.</li>
</ol>
<p>Workgraph prepares context and records work state. It does not "run the
task".</p>`)
}

func (s *Server) handleSetupMCP(w http.ResponseWriter, r *http.Request) {
	base := s.BaseURL
	if base == "" {
		base = "http://" + r.Host
	}
	tokenParam := r.URL.Query().Get("token")
	body := fmt.Sprintf(`
<h1>MCP setup</h1>
<p>Add this remote MCP server to your client:</p>
<pre>URL:    %s/mcp
Header: Authorization: Bearer %s</pre>
<h2>Claude Code</h2>
<pre>claude mcp add --transport http workgraph %s/mcp \
  --header "Authorization: Bearer %s"</pre>
<h2>Codex</h2>
<pre>codex mcp add workgraph --url %s/mcp \
  --header "Authorization: Bearer %s"</pre>`,
		base, orPlaceholder(tokenParam), base, orPlaceholder(tokenParam), base, orPlaceholder(tokenParam))
	page(w, "Workgraph MCP setup", body)
}

func orPlaceholder(s string) string {
	if s == "" {
		return "wg_tok_..."
	}
	return html.EscapeString(s)
}
