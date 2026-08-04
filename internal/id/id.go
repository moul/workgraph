// Package id generates and validates Workgraph object identifiers.
//
// An ID is an immutable, globally unique, creation-time-sortable string of the
// form "<PREFIX>-<ULID>", e.g. "ITM-01K4A2D9Q9N7H2EA2A0P5X0T6F". The prefix is
// cosmetic and indicates the object kind; parsers must not trust it for
// dispatch beyond a hint. Worker IDs are the exception: they use the readable
// "worker:<slug>" form because workers are few and human-chosen.
package id

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"

	"github.com/oklog/ulid/v2"
)

// Prefixes for each ID kind.
const (
	PrefixProject   = "PRJ"
	PrefixItem      = "ITM"
	PrefixDecision  = "DEC"
	PrefixRun       = "RUN"
	PrefixEvent     = "EVT"
	PrefixToken     = "TOK"
	PrefixWorkspace = "WKG"
)

// WorkerPrefix is the scheme for worker identities.
const WorkerPrefix = "worker:"

var idRe = regexp.MustCompile(`^[A-Z]{2,5}-[0-9A-HJKMNP-TV-Z]{26}$`)

// New returns a fresh ID with the given prefix. Entropy comes from crypto/rand,
// so IDs are safe to mint concurrently across distributed agents without a
// central sequence.
func New(prefix string) string {
	u := ulid.MustNew(ulid.Now(), rand.Reader)
	return prefix + "-" + u.String()
}

// NewItem, NewProject, etc. are convenience constructors.
func NewItem() string      { return New(PrefixItem) }
func NewProject() string   { return New(PrefixProject) }
func NewDecision() string  { return New(PrefixDecision) }
func NewRun() string       { return New(PrefixRun) }
func NewEvent() string     { return New(PrefixEvent) }
func NewToken() string     { return New(PrefixToken) }
func NewWorkspace() string { return New(PrefixWorkspace) }

// Worker returns a worker identity for the given slug, e.g. Worker("codex")
// -> "worker:codex".
func Worker(slug string) string { return WorkerPrefix + slug }

// IsWorker reports whether s is a worker identity.
func IsWorker(s string) bool { return strings.HasPrefix(s, WorkerPrefix) }

// Valid reports whether s is a syntactically valid prefixed ULID id or a worker
// identity.
func Valid(s string) bool {
	if IsWorker(s) {
		return len(s) > len(WorkerPrefix)
	}
	return idRe.MatchString(s)
}

// Prefix returns the prefix portion of an id (without the dash), or "" if s has
// no recognizable prefix.
func Prefix(s string) string {
	if IsWorker(s) {
		return "worker"
	}
	i := strings.IndexByte(s, '-')
	if i <= 0 {
		return ""
	}
	return s[:i]
}

// ULID returns the ULID portion of a prefixed id, or "" if none.
func ULID(s string) string {
	i := strings.IndexByte(s, '-')
	if i < 0 || i+1 >= len(s) {
		return ""
	}
	return s[i+1:]
}

// Filename builds a canonical object filename: "<id>-<slug>.md". The filename
// must start with the id so that filename and identity never drift.
func Filename(objID, slug string) string {
	slug = Slugify(slug)
	if slug == "" {
		return objID + ".md"
	}
	return objID + "-" + slug + ".md"
}

// IDFromFilename extracts the id from a canonical filename, or "" if the name
// does not start with a valid id.
func IDFromFilename(name string) string {
	name = strings.TrimSuffix(name, ".md")
	// id is "<PREFIX>-<26 char ulid>"; take first two dash-separated fields.
	parts := strings.SplitN(name, "-", 3)
	if len(parts) < 2 {
		return ""
	}
	cand := parts[0] + "-" + parts[1]
	if Valid(cand) {
		return cand
	}
	return ""
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify lowercases s and collapses runs of non-alphanumerics into single
// dashes, trimming leading/trailing dashes. It is deterministic and lossy;
// slugs are for humans, IDs are for machines.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonSlug.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 60 {
		s = strings.Trim(s[:60], "-")
	}
	return s
}

// Errorf is a small helper so callers get consistent id-related errors.
func Errorf(format string, a ...any) error { return fmt.Errorf("id: "+format, a...) }
