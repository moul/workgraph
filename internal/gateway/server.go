package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Server is the HTTP + MCP gateway.
type Server struct {
	Svc       *Service
	Addr      string
	BaseURL   string
	Bootstrap string // one-time bootstrap admin token value (optional)
	mcp       *MCPHandler
}

// NewServer wires a Server around a Service.
func NewServer(svc *Service, addr, baseURL, bootstrap string) *Server {
	s := &Server{Svc: svc, Addr: addr, BaseURL: baseURL, Bootstrap: bootstrap}
	s.mcp = &MCPHandler{Svc: svc, auth: s.authFor}
	return s
}

// Handler returns the root http.Handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc("/docs/api", s.handleDocsAPI)
	mux.HandleFunc("/docs/mcp", s.handleDocsMCP)
	mux.HandleFunc("/docs/subagents", s.handleDocsSubagents)
	mux.HandleFunc("/setup/mcp", s.handleSetupMCP)
	mux.HandleFunc("/t/", s.handleTokenPage)
	mux.HandleFunc("/api/v0/", s.handleAPI)
	mux.HandleFunc("/mcp", s.mcp.ServeHTTP)
	mux.HandleFunc("/admin/tokens", s.handleAdminTokens)
	mux.HandleFunc("/ui", s.handleUI)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintln(w, "ok") })
	return logMiddleware(mux)
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	srv := &http.Server{
		Addr:              s.Addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

// authFor extracts and validates the bearer token from a request.
func (s *Server) authFor(r *http.Request) (*Token, error) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return nil, fmt.Errorf("missing bearer token")
	}
	return s.Svc.Tokens.Authenticate(strings.TrimSpace(h[len("Bearer "):]))
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// ---- API ----

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	tok, err := s.authFor(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, err.Error())
		return
	}
	actor := tok.Worker
	if actor == "" {
		actor = "agent:token"
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v0/")
	parts := strings.Split(strings.Trim(path, "/"), "/")

	switch {
	case len(parts) == 1 && parts[0] == "items" && r.Method == http.MethodGet:
		s.requireScope(w, tok, "items:read", func() {
			items, err := s.Svc.ListItems(r.URL.Query().Get("status"), r.URL.Query().Get("project"))
			if err != nil {
				writeErr(w, 500, err.Error())
				return
			}
			writeJSON(w, 200, items)
		})
	case len(parts) == 1 && parts[0] == "items" && r.Method == http.MethodPost:
		s.requireScope(w, tok, "items:create", func() {
			var in struct {
				Title, Project, Kind string
				Ready                bool
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			it, err := s.Svc.CreateItem(actor, in.Title, in.Project, in.Kind, in.Ready)
			if err != nil {
				writeErr(w, 400, err.Error())
				return
			}
			writeJSON(w, 201, map[string]string{"id": it.ID, "status": it.Status})
		})
	case len(parts) == 2 && parts[0] == "items" && r.Method == http.MethodGet:
		s.requireScope(w, tok, "items:read", func() {
			m, err := s.Svc.GetItem(parts[1], r.URL.Query().Get("include_body") == "true")
			if err != nil {
				writeErr(w, 404, err.Error())
				return
			}
			writeJSON(w, 200, m)
		})
	case len(parts) == 3 && parts[0] == "items" && parts[2] == "runs" && r.Method == http.MethodPost:
		s.requireScope(w, tok, "runs:create", func() {
			res, err := s.Svc.CreateRun(actor, parts[1], tok.Worker)
			if err != nil {
				writeErr(w, 400, err.Error())
				return
			}
			writeJSON(w, 201, res.Run)
		})
	case len(parts) == 3 && parts[0] == "runs" && parts[2] == "context" && r.Method == http.MethodGet:
		s.requireScope(w, tok, "runs:context", func() {
			s.enforceRun(w, tok, parts[1], func() {
				data, err := s.Svc.RunContext(parts[1])
				if err != nil {
					writeErr(w, 404, err.Error())
					return
				}
				writeJSON(w, 200, data)
			})
		})
	case len(parts) == 3 && parts[0] == "runs" && parts[2] == "events" && r.Method == http.MethodPost:
		s.requireScope(w, tok, "runs:event", func() {
			s.enforceRun(w, tok, parts[1], func() {
				var in struct{ Action, Message string }
				_ = json.NewDecoder(r.Body).Decode(&in)
				if in.Action == "" {
					in.Action = "run.heartbeat"
				}
				if err := s.Svc.AppendRunEvent(actor, parts[1], in.Action, in.Message); err != nil {
					writeErr(w, 400, err.Error())
					return
				}
				writeJSON(w, 201, map[string]string{"status": "ok"})
			})
		})
	case len(parts) == 3 && parts[0] == "runs" && parts[2] == "finish" && r.Method == http.MethodPost:
		s.requireScope(w, tok, "runs:finish", func() {
			s.enforceRun(w, tok, parts[1], func() {
				var in struct{ Status, Summary, PR string }
				_ = json.NewDecoder(r.Body).Decode(&in)
				it, err := s.Svc.FinishRun(actor, parts[1], in.Status, in.Summary, in.PR)
				if err != nil {
					writeErr(w, 400, err.Error())
					return
				}
				writeJSON(w, 200, map[string]string{"item": it.ID, "status": it.Status})
			})
		})
	case len(parts) == 3 && parts[0] == "runs" && parts[2] == "block" && r.Method == http.MethodPost:
		s.requireScope(w, tok, "runs:block", func() {
			s.enforceRun(w, tok, parts[1], func() {
				var in struct{ Reason string }
				_ = json.NewDecoder(r.Body).Decode(&in)
				it, err := s.Svc.BlockRun(actor, parts[1], in.Reason)
				if err != nil {
					writeErr(w, 400, err.Error())
					return
				}
				writeJSON(w, 200, map[string]string{"item": it.ID, "status": it.Status})
			})
		})
	case len(parts) == 1 && parts[0] == "search" && r.Method == http.MethodGet:
		s.requireScope(w, tok, "items:read", func() {
			res, err := s.Svc.Search(r.URL.Query().Get("q"))
			if err != nil {
				writeErr(w, 500, err.Error())
				return
			}
			writeJSON(w, 200, res)
		})
	default:
		writeErr(w, http.StatusNotFound, "no such endpoint: "+r.Method+" "+r.URL.Path)
	}
}

// requireScope runs fn only if the token has scope.
func (s *Server) requireScope(w http.ResponseWriter, tok *Token, scope string, fn func()) {
	if tok.Admin || tok.HasScope(scope) {
		fn()
		return
	}
	writeErr(w, http.StatusForbidden, "token lacks scope "+scope)
}

// enforceRun refuses run-scoped tokens from touching a different run.
func (s *Server) enforceRun(w http.ResponseWriter, tok *Token, runID string, fn func()) {
	if tok.Run != "" && tok.Run != runID {
		writeErr(w, http.StatusForbidden, "token is scoped to a different run")
		return
	}
	fn()
}
