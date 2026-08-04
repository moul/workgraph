package core

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/moul/workgraph/internal/eventlog"
	"github.com/moul/workgraph/internal/id"
	"github.com/moul/workgraph/internal/index"
	"github.com/moul/workgraph/internal/model"
)

// CreateProject creates a new project object + event.
func (e *Engine) CreateProject(title, targetRepo, targetRef string) (*model.Project, error) {
	if _, err := e.preflight(true); err != nil {
		return nil, err
	}
	ts := now()
	p := &model.Project{
		Common:     model.Common{ID: id.NewProject(), Type: model.TypeProject, Title: title, Status: "active", CreatedAt: ts, UpdatedAt: ts},
		Owner:      e.Opt.Actor,
		TargetRepo: targetRepo,
		TargetRef:  targetRef,
		Health:     "unknown",
	}
	p.SetBody(fmt.Sprintf("# %s\n\n## Purpose\n\n_Describe the project._\n\n## Current outcome\n\n## Success criteria\n\n- \n\n## Constraints\n\n- \n", title))
	path, err := e.WS.Save(p)
	if err != nil {
		return nil, err
	}
	if err := e.appendEvent(eventlog.Event{ID: id.NewEvent(), Action: "project.created", Object: p.ID, Project: p.ID, Message: "Created project " + title}); err != nil {
		return nil, err
	}
	if _, err := e.commit("project("+p.ID+"): create "+title, path); err != nil {
		return p, err
	}
	return p, nil
}

// CreateItem creates a new item. Newly created items default to triage unless
// ready is true, matching the agent-safety rule.
func (e *Engine) CreateItem(title, projectRef, kind string, ready bool) (*model.Item, error) {
	if _, err := e.preflight(true); err != nil {
		return nil, err
	}
	g, err := e.WS.Load()
	if err != nil {
		return nil, err
	}
	projectID := ""
	if projectRef != "" {
		o, err := g.Resolve(projectRef)
		if err != nil {
			return nil, err
		}
		if o.ObjectType() != model.TypeProject {
			return nil, fmt.Errorf("core: %s is not a project", o.ObjectID())
		}
		projectID = o.ObjectID()
	}
	if kind == "" {
		kind = "task"
	}
	if !e.WS.Ontology.Has("item_kind", kind) {
		return nil, fmt.Errorf("core: unknown item kind %q", kind)
	}
	status := model.StatusTriage
	if ready {
		status = model.StatusReady
	}
	ts := now()
	it := &model.Item{
		Common:  model.Common{ID: id.NewItem(), Type: model.TypeItem, Title: title, Status: status, CreatedAt: ts, UpdatedAt: ts},
		Kind:    kind,
		Project: projectID,
	}
	it.SetBody(fmt.Sprintf("# %s\n\n## Goal\n\n_What outcome does this produce?_\n\n## Context\n\n## Acceptance criteria\n\n- \n\n## Constraints\n\n", title))
	path, err := e.WS.Save(it)
	if err != nil {
		return nil, err
	}
	if err := e.appendEvent(eventlog.Event{ID: id.NewEvent(), Action: "item.created", Object: it.ID, Project: projectID, To: status, Message: "Created item " + title}); err != nil {
		return nil, err
	}
	if _, err := e.commit("item("+it.ID+"): create "+title, path); err != nil {
		return it, err
	}
	return it, nil
}

// CreateDecision creates a decision object + event.
func (e *Engine) CreateDecision(title, projectRef, status string) (*model.Decision, error) {
	if _, err := e.preflight(true); err != nil {
		return nil, err
	}
	if status == "" {
		status = "proposed"
	}
	if !e.WS.Ontology.Has("decision_status", status) {
		return nil, fmt.Errorf("core: unknown decision status %q", status)
	}
	projectID := ""
	if projectRef != "" {
		g, err := e.WS.Load()
		if err != nil {
			return nil, err
		}
		o, err := g.Resolve(projectRef)
		if err != nil {
			return nil, err
		}
		projectID = o.ObjectID()
	}
	ts := now()
	d := &model.Decision{
		Common:  model.Common{ID: id.NewDecision(), Type: model.TypeDecision, Title: title, Status: status, CreatedAt: ts, UpdatedAt: ts},
		Project: projectID,
	}
	d.SetBody(fmt.Sprintf("# %s\n\n## Context\n\n## Decision\n\n## Consequences\n", title))
	path, err := e.WS.Save(d)
	if err != nil {
		return nil, err
	}
	if err := e.appendEvent(eventlog.Event{ID: id.NewEvent(), Action: "decision.created", Object: d.ID, Project: projectID, To: status, Message: "Created decision " + title}); err != nil {
		return nil, err
	}
	if _, err := e.commit("decision("+d.ID+"): create "+title, path); err != nil {
		return d, err
	}
	return d, nil
}

// ItemVersion returns the current derived blob version of an item's file.
func (e *Engine) ItemVersion(it *model.Item) string {
	raw, err := os.ReadFile(filepath.Join(e.WS.Root, it.SourcePath()))
	if err != nil {
		return ""
	}
	return index.BlobVersion(raw)
}

// UpdateItem applies a patch to an item with optimistic concurrency. If
// expectedVersion is non-empty it must match the current version or the update
// is refused — never a silent last-writer-wins.
func (e *Engine) UpdateItem(ref string, patch func(*model.Item), expectedVersion, message string) (*model.Item, error) {
	if _, err := e.preflight(true); err != nil {
		return nil, err
	}
	it, _, err := e.resolveItem(ref)
	if err != nil {
		return nil, err
	}
	if expectedVersion != "" {
		cur := e.ItemVersion(it)
		if cur != expectedVersion {
			return nil, fmt.Errorf("core: version conflict for %s: expected %s but current is %s", it.ID, expectedVersion, cur)
		}
	}
	before := it.Status
	patch(it)
	it.UpdatedAt = now()
	path, err := e.WS.Save(it)
	if err != nil {
		return nil, err
	}
	action := "item.updated"
	ev := eventlog.Event{ID: id.NewEvent(), Action: action, Object: it.ID, Project: it.Project, Message: message}
	if it.Status != before {
		ev.Action = "item.status_changed"
		ev.From = before
		ev.To = it.Status
	}
	if err := e.appendEvent(ev); err != nil {
		return nil, err
	}
	msg := message
	if msg == "" {
		msg = "update"
	}
	if _, err := e.commit("item("+it.ID+"): "+msg, path); err != nil {
		return it, err
	}
	return it, nil
}

// Link adds a typed relation from one item to a target id, validating the
// relation against the ontology.
func (e *Engine) Link(fromRef, rel, toRef string) (*model.Item, error) {
	if !e.WS.Ontology.Has("relation", rel) {
		return nil, fmt.Errorf("core: unknown relation %q", rel)
	}
	g, err := e.WS.Load()
	if err != nil {
		return nil, err
	}
	to, err := g.Resolve(toRef)
	if err != nil {
		return nil, err
	}
	return e.UpdateItem(fromRef, func(it *model.Item) {
		switch rel {
		case "depends_on":
			it.DependsOn = appendUnique(it.DependsOn, to.ObjectID())
		case "blocks":
			it.Blocks = appendUnique(it.Blocks, to.ObjectID())
		case "blocked_by":
			it.BlockedBy = appendUnique(it.BlockedBy, to.ObjectID())
		case "derived_from":
			it.DerivedFrom = appendUnique(it.DerivedFrom, to.ObjectID())
		case "duplicate_of":
			it.DuplicateOf = to.ObjectID()
		case "parent_of":
			// handled on the child side; store as parent on the target instead
		}
	}, "", "link "+rel+" "+to.ObjectID())
}

func appendUnique(xs []string, x string) []string {
	for _, v := range xs {
		if v == x {
			return xs
		}
	}
	return append(xs, x)
}
