// Package gateway is the thin Git-backed HTTP + MCP service. It owns the
// annoying parts — token auth, expected-version checks, context packet
// generation, event append — but never the source of truth: the Git repo does.
package gateway

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Token is a scoped capability, not an account password. Only the hash is
// persisted; the raw value is shown once at creation and never stored.
type Token struct {
	ID        string    `json:"token_id"`
	Hash      string    `json:"hash"`
	Workspace string    `json:"workspace,omitempty"`
	Project   string    `json:"project,omitempty"`
	Item      string    `json:"item,omitempty"`
	Run       string    `json:"run,omitempty"`
	Scopes    []string  `json:"scope"`
	Worker    string    `json:"worker,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedBy string    `json:"created_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Revoked   bool      `json:"revoked,omitempty"`
	Admin     bool      `json:"admin,omitempty"`
}

// Expired reports whether the token is past its expiry.
func (t *Token) Expired(now time.Time) bool {
	return !t.ExpiresAt.IsZero() && now.After(t.ExpiresAt)
}

// HasScope reports whether the token grants scope.
func (t *Token) HasScope(scope string) bool {
	for _, s := range t.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// TokenStore persists token hashes and audit metadata outside Git.
type TokenStore struct {
	path string
	mu   sync.Mutex
	toks map[string]*Token // keyed by hash
}

// OpenTokenStore loads (or creates) the token database at path.
func OpenTokenStore(path string) (*TokenStore, error) {
	ts := &TokenStore{path: path, toks: map[string]*Token{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ts, nil
		}
		return nil, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var t Token
		if err := json.Unmarshal([]byte(line), &t); err != nil {
			return nil, fmt.Errorf("gateway: token db line: %w", err)
		}
		ts.toks[t.Hash] = &t
	}
	return ts, nil
}

func (ts *TokenStore) save() error {
	if err := os.MkdirAll(filepath.Dir(ts.path), 0o755); err != nil {
		return err
	}
	var sb strings.Builder
	for _, t := range ts.toks {
		b, _ := json.Marshal(t)
		sb.Write(b)
		sb.WriteByte('\n')
	}
	return os.WriteFile(ts.path, []byte(sb.String()), 0o600)
}

// hashValue returns the storage hash for a raw token value.
func hashValue(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

// Mint creates a new token, returning the raw value (shown once) and the record.
func (ts *TokenStore) Mint(t Token, ttl time.Duration) (raw string, rec *Token, err error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, err
	}
	raw = "wg_tok_" + hex.EncodeToString(buf)
	t.Hash = hashValue(raw)
	if t.ID == "" {
		t.ID = "TOK-" + hex.EncodeToString(buf[:8])
	}
	t.CreatedAt = time.Now()
	if ttl != 0 {
		t.ExpiresAt = t.CreatedAt.Add(ttl)
	}
	ts.toks[t.Hash] = &t
	if err := ts.save(); err != nil {
		return "", nil, err
	}
	return raw, &t, nil
}

// Lookup returns the token for a raw value, or nil.
func (ts *TokenStore) Lookup(raw string) *Token {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.toks[hashValue(raw)]
}

// Authenticate returns the token if valid (exists, not revoked, not expired).
func (ts *TokenStore) Authenticate(raw string) (*Token, error) {
	t := ts.Lookup(raw)
	if t == nil {
		return nil, fmt.Errorf("unknown token")
	}
	if t.Revoked {
		return nil, fmt.Errorf("token revoked")
	}
	if t.Expired(time.Now()) {
		return nil, fmt.Errorf("token expired")
	}
	return t, nil
}

// Revoke marks a token id revoked.
func (ts *TokenStore) Revoke(tokenID string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	for _, t := range ts.toks {
		if t.ID == tokenID {
			t.Revoked = true
			return ts.save()
		}
	}
	return fmt.Errorf("gateway: token %s not found", tokenID)
}

// List returns all token records (without hashes usable to reconstruct values).
func (ts *TokenStore) List() []*Token {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	out := make([]*Token, 0, len(ts.toks))
	for _, t := range ts.toks {
		cp := *t
		out = append(out, &cp)
	}
	return out
}

// DefaultTTL returns the recommended TTL for a token kind.
func DefaultTTL(kind string) time.Duration {
	switch kind {
	case "run":
		return 24 * time.Hour
	case "item":
		return 7 * 24 * time.Hour
	default:
		return 0 // workspace tokens require an explicit expiry
	}
}
