package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// MCPHandler implements a compact Model Context Protocol server (JSON-RPC 2.0)
// over the same Service the HTTP API uses. It is not a richer protocol — just a
// compact API over the same core. The same handler serves the remote HTTP
// endpoint and the stdio server.
type MCPHandler struct {
	Svc  *Service
	auth func(*http.Request) (*Token, error) // nil for stdio (trusted actor)
	// Actor is used when auth is nil (stdio mode).
	Actor string
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ServeHTTP handles POST /mcp.
func (h *MCPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	actor := "agent:mcp"
	if h.auth != nil {
		tok, err := h.auth(r)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, err.Error())
			return
		}
		if tok.Worker != "" {
			actor = tok.Worker
		}
	}
	body, _ := io.ReadAll(r.Body)
	resp := h.Dispatch(actor, body)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// Dispatch handles one JSON-RPC message and returns the response object.
func (h *MCPHandler) Dispatch(actor string, raw []byte) rpcResponse {
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return rpcResponse{JSONRPC: "2.0", Error: &rpcError{-32700, "parse error"}}
	}
	res, rerr := h.handle(actor, req)
	out := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	if rerr != nil {
		out.Error = rerr
	} else {
		out.Result = res
	}
	return out
}

func (h *MCPHandler) handle(actor string, req rpcRequest) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": "2025-06-18",
			"serverInfo":      map[string]string{"name": "workgraph", "version": "0.1"},
			"capabilities":    map[string]any{"tools": map[string]any{}, "resources": map[string]any{}},
		}, nil
	case "tools/list":
		return map[string]any{"tools": toolSpecs()}, nil
	case "tools/call":
		return h.toolCall(actor, req.Params)
	case "resources/list":
		return map[string]any{"resources": resourceSpecs()}, nil
	case "ping":
		return map[string]any{}, nil
	default:
		return nil, &rpcError{-32601, "method not found: " + req.Method}
	}
}

func (h *MCPHandler) toolCall(actor string, params json.RawMessage) (any, *rpcError) {
	var p struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{-32602, "invalid params"}
	}
	a := p.Arguments
	str := func(k string) string {
		if v, ok := a[k].(string); ok {
			return v
		}
		return ""
	}
	boolean := func(k string) bool {
		v, _ := a[k].(bool)
		return v
	}

	var result any
	var err error
	switch p.Name {
	case "init":
		result = map[string]any{"workspace": h.Svc.Root, "hint": "use list_items then get_item; create_run to start a work round"}
	case "list_items":
		result, err = h.Svc.ListItems(str("status"), str("project"))
	case "get_item":
		result, err = h.Svc.GetItem(str("item_id"), boolean("include_body"))
	case "create_item":
		var it any
		it, err = h.Svc.CreateItem(actor, str("title"), str("project"), str("kind"), boolean("ready"))
		result = it
	case "create_run":
		var rr any
		rr, err = h.Svc.CreateRun(actor, str("item_id"), str("worker_id"))
		result = rr
	case "get_run_context":
		result, err = h.Svc.RunContext(str("run_id"))
	case "append_run_event":
		action := str("action")
		if action == "" {
			action = "run.heartbeat"
		}
		err = h.Svc.AppendRunEvent(actor, str("run_id"), action, str("message"))
		result = map[string]string{"status": "ok"}
	case "finish_run":
		var it any
		it, err = h.Svc.FinishRun(actor, str("run_id"), str("status"), str("summary"), str("pr"))
		result = it
	case "block_run":
		var it any
		it, err = h.Svc.BlockRun(actor, str("run_id"), str("reason"))
		result = it
	case "search":
		result, err = h.Svc.Search(str("query"))
	default:
		return nil, &rpcError{-32601, "unknown tool: " + p.Name}
	}
	if err != nil {
		return toolText(fmt.Sprintf("error: %v", err), true), nil
	}
	b, _ := json.MarshalIndent(result, "", "  ")
	return toolText(string(b), false), nil
}

// toolText wraps content in the MCP tool-result shape.
func toolText(text string, isErr bool) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": isErr,
	}
}

func toolSpecs() []map[string]any {
	obj := func(props map[string]any, required ...string) map[string]any {
		return map[string]any{"type": "object", "properties": props, "required": required}
	}
	strProp := map[string]any{"type": "string"}
	boolProp := map[string]any{"type": "boolean"}
	return []map[string]any{
		{"name": "init", "description": "Return workspace info and a first-call hint.", "inputSchema": obj(map[string]any{})},
		{"name": "list_items", "description": "List items (compact). Filter by status/project.", "inputSchema": obj(map[string]any{"status": strProp, "project": strProp})},
		{"name": "get_item", "description": "Get one item; include_body for the full markdown.", "inputSchema": obj(map[string]any{"item_id": strProp, "include_body": boolProp}, "item_id")},
		{"name": "create_item", "description": "Create an item (defaults to triage).", "inputSchema": obj(map[string]any{"title": strProp, "project": strProp, "kind": strProp, "ready": boolProp}, "title")},
		{"name": "create_run", "description": "Start a work round against an item.", "inputSchema": obj(map[string]any{"item_id": strProp, "worker_id": strProp}, "item_id")},
		{"name": "get_run_context", "description": "Get the task-scoped context packet for a run.", "inputSchema": obj(map[string]any{"run_id": strProp}, "run_id")},
		{"name": "append_run_event", "description": "Append an operational event to a run.", "inputSchema": obj(map[string]any{"run_id": strProp, "action": strProp, "message": strProp}, "run_id")},
		{"name": "finish_run", "description": "Finish a run and set the resulting status.", "inputSchema": obj(map[string]any{"run_id": strProp, "status": strProp, "summary": strProp, "pr": strProp}, "run_id")},
		{"name": "block_run", "description": "Block a run with a reason.", "inputSchema": obj(map[string]any{"run_id": strProp, "reason": strProp}, "run_id")},
		{"name": "search", "description": "Substring search over items.", "inputSchema": obj(map[string]any{"query": strProp}, "query")},
	}
}

func resourceSpecs() []map[string]any {
	return []map[string]any{
		{"uri": "workgraph://indexes/objects", "name": "objects index", "mimeType": "application/jsonl"},
		{"uri": "workgraph://indexes/attention", "name": "attention queue", "mimeType": "application/jsonl"},
	}
}
