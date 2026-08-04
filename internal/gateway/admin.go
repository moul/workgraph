package gateway

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// handleAdminTokens mints scoped tokens. It is protected by an admin token (the
// bootstrap token or any token with admin:tokens:create). Token values are
// shown once, in the response, and never stored.
func (s *Server) handleAdminTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	if !s.adminAuth(r) {
		writeErr(w, http.StatusUnauthorized, "admin auth required")
		return
	}
	var in struct {
		Scopes   []string `json:"scopes"`
		Item     string   `json:"item"`
		Run      string   `json:"run"`
		Project  string   `json:"project"`
		Worker   string   `json:"worker"`
		Kind     string   `json:"kind"` // run|item|workspace
		TTLHours int      `json:"ttl_hours"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)

	ttl := DefaultTTL(in.Kind)
	if in.TTLHours > 0 {
		ttl = time.Duration(in.TTLHours) * time.Hour
	}
	raw, rec, err := s.Svc.Tokens.Mint(Token{
		Scopes:    in.Scopes,
		Item:      in.Item,
		Run:       in.Run,
		Project:   in.Project,
		Worker:    in.Worker,
		CreatedBy: "admin",
	}, ttl)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	base := s.BaseURL
	if base == "" {
		base = "http://" + r.Host
	}
	writeJSON(w, 201, map[string]any{
		"token":      raw, // shown once
		"token_id":   rec.ID,
		"url":        base + "/t/" + raw,
		"scopes":     rec.Scopes,
		"expires_at": rec.ExpiresAt,
	})
}

// adminAuth accepts the bootstrap token or an admin-scoped bearer token.
func (s *Server) adminAuth(r *http.Request) bool {
	h := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	h = strings.TrimSpace(h)
	if s.Bootstrap != "" && h == s.Bootstrap {
		return true
	}
	tok, err := s.Svc.Tokens.Authenticate(h)
	if err != nil {
		return false
	}
	return tok.Admin || tok.HasScope("admin:tokens:create")
}
