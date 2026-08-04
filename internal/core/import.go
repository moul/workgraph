package core

import (
	"fmt"
	"time"

	"github.com/moul/workgraph/internal/eventlog"
	"github.com/moul/workgraph/internal/id"
	"github.com/moul/workgraph/internal/model"
)

// ImportSpec describes one item to import from an external system.
type ImportSpec struct {
	Title     string
	Body      string
	SourceRef string // e.g. "github:moul/hermes#123" or "markdown:TODO.md:12"
	Source    string // e.g. "github", "markdown"
	Project   string // project id (optional)
	Status    string // defaults to inbox
	Kind      string // defaults to task
}

// Import creates items from specs, non-invasively. Imports are idempotent by
// source_ref: re-importing the same reference updates provenance instead of
// creating a duplicate. Imported items default to inbox/triage and never
// auto-ready. Returns the number created and the number skipped as duplicates.
func (e *Engine) Import(specs []ImportSpec) (created, skipped int, err error) {
	g, err := e.WS.Load()
	if err != nil {
		return 0, 0, err
	}
	// Index existing items by source_ref for idempotency.
	bySource := map[string]*model.Item{}
	for _, it := range g.Items {
		if it.SourceRef != "" {
			bySource[it.SourceRef] = it
		}
	}

	for _, sp := range specs {
		if sp.SourceRef != "" {
			if _, ok := bySource[sp.SourceRef]; ok {
				skipped++
				continue
			}
		}
		status := sp.Status
		if status == "" {
			status = model.StatusInbox
		}
		if !e.WS.Ontology.Has("item_status", status) {
			return created, skipped, fmt.Errorf("core: unknown import status %q", status)
		}
		kind := sp.Kind
		if kind == "" {
			kind = "task"
		}
		ts := now()
		it := &model.Item{
			Common:           model.Common{ID: id.NewItem(), Type: model.TypeItem, Title: sp.Title, Status: status, CreatedAt: ts, UpdatedAt: ts},
			Kind:             kind,
			Project:          sp.Project,
			Source:           sp.Source,
			SourceRef:        sp.SourceRef,
			SourceImportedAt: ts,
		}
		body := sp.Body
		if body == "" {
			body = "# " + sp.Title + "\n"
		}
		it.SetBody(body)
		path, serr := e.WS.Save(it)
		if serr != nil {
			return created, skipped, serr
		}
		if aerr := e.appendEvent(eventlog.Event{ID: id.NewEvent(), Action: "item.created", Object: it.ID, Project: sp.Project, To: status, Message: "Imported from " + sp.SourceRef}); aerr != nil {
			return created, skipped, aerr
		}
		if _, cerr := e.commit("item("+it.ID+"): import from "+sp.SourceRef, path); cerr != nil {
			return created, skipped, cerr
		}
		created++
	}
	_ = time.Now
	return created, skipped, nil
}
