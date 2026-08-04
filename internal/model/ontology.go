package model

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Ontology is the deterministic contract for valid project state. It is loaded
// from ontologies/workgraph.yaml and consulted by validation.
type Ontology struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`

	ObjectTypes      []string `yaml:"object_types"`
	ItemKinds        []string `yaml:"item_kinds"`
	ItemStatuses     []string `yaml:"item_statuses"`
	DecisionStatuses []string `yaml:"decision_statuses"`
	ProjectStatuses  []string `yaml:"project_statuses"`
	WorkerKinds      []string `yaml:"worker_kinds"`
	HealthValues     []string `yaml:"health_values"`
	RelationTypes    []string `yaml:"relation_types"`
	EventActions     []string `yaml:"event_actions"`
	AttentionReasons []string `yaml:"attention_reasons"`
	Capabilities     []string `yaml:"capabilities"`
	TokenScopes      []string `yaml:"token_scopes"`

	sets map[string]map[string]bool
}

// OntologyPath is the canonical location of the manifest within a workspace.
const OntologyPath = "ontologies/workgraph.yaml"

// LoadOntology reads the ontology manifest from a workspace root. If the file
// is absent, DefaultOntology is returned so a partially-initialized repo still
// validates against sane defaults.
func LoadOntology(root string) (*Ontology, error) {
	p := filepath.Join(root, OntologyPath)
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			o := DefaultOntology()
			o.index()
			return o, nil
		}
		return nil, fmt.Errorf("ontology: read %s: %w", p, err)
	}
	var o Ontology
	if err := yaml.Unmarshal(raw, &o); err != nil {
		return nil, fmt.Errorf("ontology: parse %s: %w", p, err)
	}
	o.index()
	return &o, nil
}

func (o *Ontology) index() {
	o.sets = map[string]map[string]bool{
		"object_type":      set(o.ObjectTypes),
		"item_kind":        set(o.ItemKinds),
		"item_status":      set(o.ItemStatuses),
		"decision_status":  set(o.DecisionStatuses),
		"project_status":   set(o.ProjectStatuses),
		"worker_kind":      set(o.WorkerKinds),
		"health":           set(o.HealthValues),
		"relation":         set(o.RelationTypes),
		"event_action":     set(o.EventActions),
		"attention_reason": set(o.AttentionReasons),
		"capability":       set(o.Capabilities),
		"token_scope":      set(o.TokenScopes),
	}
}

func set(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

// Has reports whether value is a member of the named vocabulary (e.g.
// "item_status", "event_action"). Unknown vocabulary names return false.
func (o *Ontology) Has(vocab, value string) bool {
	if o.sets == nil {
		o.index()
	}
	m, ok := o.sets[vocab]
	if !ok {
		return false
	}
	return m[value]
}

// DefaultOntology returns the built-in v0.1 ontology, used when a workspace has
// no manifest yet (e.g. right after `workgraph init` scaffolds files).
func DefaultOntology() *Ontology {
	return &Ontology{
		Name:             "workgraph",
		Version:          "0.1",
		ObjectTypes:      []string{TypeProject, TypeItem, TypeDecision, TypeWorker},
		ItemKinds:        []string{"task", "bug", "question", "experiment", "review", "epic", "incident", "idea"},
		ItemStatuses:     []string{StatusInbox, StatusTriage, StatusReady, StatusInProgress, StatusBlocked, StatusReview, StatusDone, StatusCancelled, StatusArchived},
		DecisionStatuses: []string{"proposed", "accepted", "superseded", "rejected"},
		ProjectStatuses:  []string{"active", "paused", "done", "archived"},
		WorkerKinds:      []string{"human", "agent"},
		HealthValues:     []string{"unknown", "on_track", "at_risk", "blocked"},
		RelationTypes:    []string{"parent_of", "depends_on", "blocks", "blocked_by", "derived_from", "duplicates", "duplicate_of", "supersedes", "superseded_by", "targets_repo", "constrained_by_decision"},
		EventActions: []string{
			"item.created", "item.updated", "item.status_changed", "item.claimed", "item.linked", "item.archived",
			"decision.created", "decision.status_changed", "project.created",
			"run.created", "run.started", "run.heartbeat", "run.blocked", "run.released",
			"run.review_requested", "run.reviewed", "run.finished", "run.artifact_attached", "run.superseded",
			"token.created", "token.revoked",
		},
		AttentionReasons: []string{"blocked_by_human", "review_assigned_to_human", "lease_expired", "missing_dependency", "no_heartbeat", "target_behind_remote", "new_triage_item", "manual_override", "blocked_without_blocked_by", "proposed_decision"},
		Capabilities:     []string{"create_item", "update_item", "claim_item", "finish_run", "block_run", "archive_item", "edit_decision", "override_conflict"},
		TokenScopes:      []string{"items:read", "items:create", "runs:create", "runs:context", "runs:event", "runs:finish", "runs:block", "admin:tokens:create", "admin:tokens:revoke"},
	}
}
