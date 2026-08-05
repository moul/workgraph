package store

import "fmt"

// CurrentSchema is the schema version this build understands. Schema version
// lives in workgraph.yaml (not on each object) so a migration is a single,
// boring file rewrite.
const CurrentSchema = "0.1"

// Migration transforms a workspace from one schema version to the next. Apply
// returns a human-readable list of the changes it made. Migrations must be
// deterministic file rewrites — no hidden database steps.
type Migration struct {
	From  string
	To    string
	Apply func(w *Workspace) ([]string, error)
}

// migrations is the ordered registry. It is empty at 0.1 (the first schema);
// future schema bumps append entries here.
var migrations []Migration

// Plan returns the migrations that would run to bring cfgVersion up to
// CurrentSchema, in order. It returns an error when the workspace is newer than
// this build understands.
func Plan(cfgVersion string) ([]Migration, error) {
	if cfgVersion == "" {
		cfgVersion = CurrentSchema
	}
	if cfgVersion == CurrentSchema {
		return nil, nil
	}
	// Walk the chain from cfgVersion forward.
	var out []Migration
	cur := cfgVersion
	for cur != CurrentSchema {
		next := findMigration(cur)
		if next == nil {
			return nil, fmt.Errorf("no migration path from schema %q toward %q (this build understands %q)", cur, CurrentSchema, CurrentSchema)
		}
		out = append(out, *next)
		cur = next.To
	}
	return out, nil
}

func findMigration(from string) *Migration {
	for i := range migrations {
		if migrations[i].From == from {
			return &migrations[i]
		}
	}
	return nil
}
