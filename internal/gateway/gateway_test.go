package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/moul/workgraph/internal/core"
	"github.com/moul/workgraph/internal/store"
)

func setup(t *testing.T) (*Server, *Token, string) {
	t.Helper()
	root := t.TempDir()
	cfg := store.DefaultConfig()
	cfg.WorkspaceID = "WKG-T"
	if err := cfg.Save(root); err != nil {
		t.Fatal(err)
	}
	ws, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	e := core.New(ws, core.Options{Actor: "human:test", NoCommit: true})
	p, _ := e.CreateProject("Hermes", "", "")
	if _, err := e.CreateItem("Add run summary", p.ID, "task", true); err != nil {
		t.Fatal(err)
	}

	ts, _ := OpenTokenStore(root + "/.workgraph/gateway.db")
	svc := &Service{Root: root, Tokens: ts, NoPush: true}
	raw, rec, _ := ts.Mint(Token{Scopes: []string{"items:read", "items:create", "runs:create", "runs:context"}, Worker: "agent:tester"}, time.Hour)
	return NewServer(svc, ":0", "http://localhost", ""), rec, raw
}

func TestAPIListRequiresAuth(t *testing.T) {
	srv, _, _ := setup(t)
	h := srv.Handler()

	req := httptest.NewRequest("GET", "/api/v0/items", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no-auth = %d, want 401", rec.Code)
	}
}

func TestAPIListWithToken(t *testing.T) {
	srv, _, raw := setup(t)
	h := srv.Handler()

	req := httptest.NewRequest("GET", "/api/v0/items", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code = %d body=%s", rec.Code, rec.Body.String())
	}
	var items []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &items)
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
}

func TestScopeEnforced(t *testing.T) {
	root := t.TempDir()
	cfg := store.DefaultConfig()
	cfg.WorkspaceID = "WKG-T"
	_ = cfg.Save(root)
	ws, _ := store.Open(root)
	_ = core.New(ws, core.Options{Actor: "x", NoCommit: true})
	ts, _ := OpenTokenStore(root + "/.workgraph/gateway.db")
	svc := &Service{Root: root, Tokens: ts, NoPush: true}
	srv := NewServer(svc, ":0", "", "")

	// read-only token cannot create.
	raw, _, _ := ts.Mint(Token{Scopes: []string{"items:read"}}, time.Hour)
	req := httptest.NewRequest("POST", "/api/v0/items", strings.NewReader(`{"Title":"x"}`))
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("read token create = %d, want 403", rec.Code)
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	srv, _, _ := setup(t)
	raw, _, _ := srv.Svc.Tokens.Mint(Token{Scopes: []string{"items:read"}}, -time.Hour)
	req := httptest.NewRequest("GET", "/api/v0/items", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expired token = %d, want 401", rec.Code)
	}
}

func TestMCPDispatch(t *testing.T) {
	srv, _, _ := setup(t)
	resp := srv.mcp.Dispatch("agent:test", []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if resp.Error != nil {
		t.Fatalf("mcp error: %v", resp.Error)
	}
	m := resp.Result.(map[string]any)
	if len(m["tools"].([]map[string]any)) < 8 {
		t.Fatalf("too few tools")
	}
}

func TestTokenRevoke(t *testing.T) {
	srv, rec, raw := setup(t)
	if err := srv.Svc.Tokens.Revoke(rec.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.Svc.Tokens.Authenticate(raw); err == nil {
		t.Fatal("revoked token still authenticates")
	}
}
