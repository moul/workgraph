// Package model defines the canonical Workgraph object types and the ontology
// manifest that validates them.
//
// Every object serializes to flat YAML frontmatter plus a Markdown body. Struct
// field order determines frontmatter key order, which keeps diffs stable and
// human-readable. Unknown frontmatter keys are preserved through the inline
// Extra map so a rewrite never silently drops fields a human or future schema
// added.
package model

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// ObjectType values.
const (
	TypeProject  = "project"
	TypeItem     = "item"
	TypeDecision = "decision"
	TypeWorker   = "worker"
)

// Item statuses.
const (
	StatusInbox      = "inbox"
	StatusTriage     = "triage"
	StatusReady      = "ready"
	StatusInProgress = "in_progress"
	StatusBlocked    = "blocked"
	StatusReview     = "review"
	StatusDone       = "done"
	StatusCancelled  = "cancelled"
	StatusArchived   = "archived"
)

// Object is the shared surface of every canonical object.
type Object interface {
	ObjectID() string
	ObjectType() string
	ObjectTitle() string
	ObjectStatus() string
	SourcePath() string
	SetSourcePath(string)
}

// Common holds the fields required on every object.
type Common struct {
	ID        string `yaml:"id"`
	Type      string `yaml:"type"`
	Title     string `yaml:"title"`
	Status    string `yaml:"status"`
	CreatedAt string `yaml:"created_at"`
	UpdatedAt string `yaml:"updated_at"`

	// path is the repo-relative source file; it is not serialized.
	path string `yaml:"-"`
	// Body is the Markdown body; not part of frontmatter.
	body string `yaml:"-"`
}

func (c *Common) ObjectID() string       { return c.ID }
func (c *Common) ObjectType() string     { return c.Type }
func (c *Common) ObjectTitle() string    { return c.Title }
func (c *Common) ObjectStatus() string   { return c.Status }
func (c *Common) SourcePath() string     { return c.path }
func (c *Common) SetSourcePath(p string) { c.path = p }
func (c *Common) Body() string           { return c.body }
func (c *Common) SetBody(b string)       { c.body = b }

// Project is a durable context boundary.
type Project struct {
	Common     `yaml:",inline"`
	Owner      string         `yaml:"owner,omitempty"`
	TargetRepo string         `yaml:"target_repo,omitempty"`
	TargetRef  string         `yaml:"target_ref,omitempty"`
	Health     string         `yaml:"health,omitempty"`
	Progress   int            `yaml:"progress,omitempty"`
	TargetAt   string         `yaml:"target_at,omitempty"`
	Tags       []string       `yaml:"tags,omitempty"`
	Extra      map[string]any `yaml:",inline"`
}

// Item is the durable issue/problem, often equivalent to an issue.
type Item struct {
	Common `yaml:",inline"`

	Kind    string `yaml:"kind,omitempty"`
	Project string `yaml:"project,omitempty"`

	Priority string `yaml:"priority,omitempty"`
	Owner    string `yaml:"owner,omitempty"`
	Reviewer string `yaml:"reviewer,omitempty"`

	Parent      string   `yaml:"parent,omitempty"`
	Goal        string   `yaml:"goal,omitempty"`
	DependsOn   []string `yaml:"depends_on,omitempty"`
	Blocks      []string `yaml:"blocks,omitempty"`
	BlockedBy   []string `yaml:"blocked_by,omitempty"`
	DerivedFrom []string `yaml:"derived_from,omitempty"`
	Related     []string `yaml:"related,omitempty"`
	DuplicateOf string   `yaml:"duplicate_of,omitempty"`

	TargetRepo string `yaml:"target_repo,omitempty"`
	TargetRef  string `yaml:"target_ref,omitempty"`
	TargetPath string `yaml:"target_path,omitempty"`

	ContextPolicy  string `yaml:"context_policy,omitempty"`
	ParallelPolicy string `yaml:"parallel_policy,omitempty"`
	Approval       string `yaml:"approval,omitempty"`

	DueAt        string `yaml:"due_at,omitempty"`
	StartedAt    string `yaml:"started_at,omitempty"`
	CompletedAt  string `yaml:"completed_at,omitempty"`
	Progress     int    `yaml:"progress,omitempty"`
	ProgressMode string `yaml:"progress_mode,omitempty"`
	Estimate     int    `yaml:"estimate_minutes,omitempty"`

	Attention       bool   `yaml:"attention,omitempty"`
	AttentionUntil  string `yaml:"attention_until,omitempty"`
	AttentionReason string `yaml:"attention_reason,omitempty"`

	// Active claim / lease (mirrors the currently active run).
	ClaimedAt  string `yaml:"claimed_at,omitempty"`
	LeaseUntil string `yaml:"lease_until,omitempty"`
	RunID      string `yaml:"run_id,omitempty"`

	Source           string `yaml:"source,omitempty"`
	SourceRef        string `yaml:"source_ref,omitempty"`
	SourceImportedAt string `yaml:"source_imported_at,omitempty"`

	Tags  []string       `yaml:"tags,omitempty"`
	Extra map[string]any `yaml:",inline"`
}

// Decision is a durable constraint on future work.
type Decision struct {
	Common `yaml:",inline"`

	Project      string         `yaml:"project,omitempty"`
	Supersedes   []string       `yaml:"supersedes,omitempty"`
	SupersededBy string         `yaml:"superseded_by,omitempty"`
	Tags         []string       `yaml:"tags,omitempty"`
	Extra        map[string]any `yaml:",inline"`
}

// Worker is a human or agent that performs work.
type Worker struct {
	Common `yaml:",inline"`

	Kind              string         `yaml:"kind,omitempty"`
	Capabilities      []string       `yaml:"capabilities,omitempty"`
	RequiresReviewFor []string       `yaml:"requires_review_for,omitempty"`
	ConcurrencyLimit  int            `yaml:"concurrency_limit,omitempty"`
	Tags              []string       `yaml:"tags,omitempty"`
	Extra             map[string]any `yaml:",inline"`
}

// Decode fills v from a frontmatter meta map by round-tripping through YAML.
// This lets callers parse once with the frontmatter package and then decode
// into the appropriate typed struct.
func Decode(meta map[string]any, v any) error {
	raw, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("model: re-encode meta: %w", err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(false)
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("model: decode: %w", err)
	}
	return nil
}
