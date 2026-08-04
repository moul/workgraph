package store

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ConfigFile is the workspace identity/defaults file at the workspace root.
const ConfigFile = "workgraph.yaml"

// Config is the parsed workgraph.yaml. Schema version belongs here, not on each
// object, so migrations are a single boring file rewrite.
type Config struct {
	SchemaVersion  string `yaml:"schema_version"`
	WorkspaceID    string `yaml:"workspace_id"`
	Name           string `yaml:"name,omitempty"`
	DefaultBranch  string `yaml:"default_branch,omitempty"`
	IndexPolicy    string `yaml:"index_policy,omitempty"`    // committed | ignored
	MutationPolicy string `yaml:"mutation_policy,omitempty"` // direct | branch
	BranchPrefix   string `yaml:"branch_prefix,omitempty"`
	CommitMode     string `yaml:"commit_mode,omitempty"` // auto | manual
	CreatedBy      string `yaml:"created_by,omitempty"`
	CreatedAt      string `yaml:"created_at,omitempty"`

	Extra map[string]any `yaml:",inline"`
}

// DefaultConfig returns sane defaults for a fresh workspace.
func DefaultConfig() Config {
	return Config{
		SchemaVersion:  "0.1",
		DefaultBranch:  "main",
		IndexPolicy:    "committed",
		MutationPolicy: "direct",
		BranchPrefix:   "workgraph/",
		CommitMode:     "auto",
	}
}

// LoadConfig reads workgraph.yaml from root.
func LoadConfig(root string) (*Config, error) {
	p := filepath.Join(root, ConfigFile)
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", p, err)
	}
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", p, err)
	}
	return &cfg, nil
}

// Save writes the config back to root/workgraph.yaml.
func (c *Config) Save(root string) error {
	raw, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	return os.WriteFile(filepath.Join(root, ConfigFile), raw, 0o644)
}
