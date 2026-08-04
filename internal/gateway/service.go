package gateway

import (
	"fmt"

	"github.com/moul/workgraph/internal/capsule"
	"github.com/moul/workgraph/internal/core"
	"github.com/moul/workgraph/internal/eventlog"
	"github.com/moul/workgraph/internal/index"
	"github.com/moul/workgraph/internal/model"
	"github.com/moul/workgraph/internal/store"
)

// Service is the shared business layer behind both the HTTP API and the MCP
// server. Every write goes through the same core mutation engine the CLI uses.
type Service struct {
	Root   string
	Tokens *TokenStore
	// NoPush disables git push (useful for single-tenant local gateways).
	NoPush bool
}

func (s *Service) engine(actor string) (*core.Engine, error) {
	ws, err := store.Open(s.Root)
	if err != nil {
		return nil, err
	}
	return core.New(ws, core.Options{Actor: actor, NoPush: s.NoPush}), nil
}

func (s *Service) workspace() (*store.Workspace, error) { return store.Open(s.Root) }

// ListItems returns compact object lines filtered by status/project.
func (s *Service) ListItems(status, project string) ([]index.ObjectLine, error) {
	ws, err := s.workspace()
	if err != nil {
		return nil, err
	}
	res, err := index.Build(ws, false)
	if err != nil {
		return nil, err
	}
	var out []index.ObjectLine
	for _, r := range res.Objects {
		if r.Type != model.TypeItem {
			continue
		}
		if status != "" && r.Status != status {
			continue
		}
		if project != "" && r.Project != project {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

// GetItem returns one item with optional body.
func (s *Service) GetItem(ref string, includeBody bool) (map[string]any, error) {
	ws, err := s.workspace()
	if err != nil {
		return nil, err
	}
	g, err := ws.Load()
	if err != nil {
		return nil, err
	}
	o, err := g.Resolve(ref)
	if err != nil {
		return nil, err
	}
	m := map[string]any{"id": o.ObjectID(), "type": o.ObjectType(), "title": o.ObjectTitle(), "status": o.ObjectStatus(), "path": o.SourcePath()}
	if it, ok := o.(*model.Item); ok {
		m["kind"] = it.Kind
		m["project"] = it.Project
		m["priority"] = it.Priority
		m["target_repo"] = it.TargetRepo
		m["depends_on"] = it.DependsOn
		if includeBody {
			m["body"] = it.Body()
		}
	}
	return m, nil
}

// CreateItem creates a triage (or ready) item.
func (s *Service) CreateItem(actor, title, project, kind string, ready bool) (*model.Item, error) {
	e, err := s.engine(actor)
	if err != nil {
		return nil, err
	}
	return e.CreateItem(title, project, kind, ready)
}

// CreateRun starts a run and returns the capsule contract.
func (s *Service) CreateRun(actor, itemRef, worker string) (*core.RunResult, error) {
	e, err := s.engine(actor)
	if err != nil {
		return nil, err
	}
	// No target repo path on the gateway side: the capsule is returned as data.
	return e.CreateRun(itemRef, worker, "generic", "")
}

// RunContext returns the same content the CLI writes to the capsule, as data.
func (s *Service) RunContext(runID string) (*capsule.Data, error) {
	ws, err := s.workspace()
	if err != nil {
		return nil, err
	}
	evs, err := eventlog.ReadAll(ws.Root)
	if err != nil {
		return nil, err
	}
	var itemID, worker string
	round := 0
	for _, ev := range eventlog.ForRun(evs, runID) {
		if ev.Object != "" {
			itemID = ev.Object
		}
		if ev.Worker != "" {
			worker = ev.Worker
		}
		if ev.Round != 0 {
			round = ev.Round
		}
	}
	if itemID == "" {
		return nil, fmt.Errorf("gateway: no item for run %s", runID)
	}
	g, err := ws.Load()
	if err != nil {
		return nil, err
	}
	o := g.ByID(itemID)
	it, ok := o.(*model.Item)
	if !ok {
		return nil, fmt.Errorf("gateway: %s is not an item", itemID)
	}
	var proj *model.Project
	if p := g.ByID(it.Project); p != nil {
		proj, _ = p.(*model.Project)
	}
	var decisions []*model.Decision
	for _, d := range g.Decisions {
		if d.Status == "accepted" && (d.Project == "" || d.Project == it.Project) {
			decisions = append(decisions, d)
		}
	}
	var deps []*model.Item
	for _, dep := range it.DependsOn {
		if d, ok := g.ByID(dep).(*model.Item); ok {
			deps = append(deps, d)
		}
	}
	run := capsule.Run{
		RunID:         runID,
		ItemID:        it.ID,
		Round:         round,
		Worker:        worker,
		TargetRepo:    it.TargetRepo,
		TargetRef:     it.TargetRef,
		FinishCommand: fmt.Sprintf("POST /api/v0/runs/%s/finish", runID),
		BlockCommand:  fmt.Sprintf("POST /api/v0/runs/%s/block", runID),
	}
	return &capsule.Data{Run: run, Item: it, Project: proj, Decisions: decisions, DependsOn: deps}, nil
}

// AppendRunEvent appends an operational event to a run.
func (s *Service) AppendRunEvent(actor, runID, action, message string) error {
	ws, err := s.workspace()
	if err != nil {
		return err
	}
	if !ws.Ontology.Has("event_action", action) {
		return fmt.Errorf("gateway: unknown event action %q", action)
	}
	return eventlog.Append(ws.Root, eventlog.Event{Action: action, Run: runID, Actor: actor, Message: message})
}

// FinishRun completes a run.
func (s *Service) FinishRun(actor, runID, status, summary, pr string) (*model.Item, error) {
	e, err := s.engine(actor)
	if err != nil {
		return nil, err
	}
	return e.Finish(runID, status, summary, pr)
}

// BlockRun blocks a run.
func (s *Service) BlockRun(actor, runID, reason string) (*model.Item, error) {
	e, err := s.engine(actor)
	if err != nil {
		return nil, err
	}
	return e.Block(runID, reason)
}

// Search does a substring search over object titles and summaries.
func (s *Service) Search(query string) ([]index.ObjectLine, error) {
	ws, err := s.workspace()
	if err != nil {
		return nil, err
	}
	res, err := index.Build(ws, false)
	if err != nil {
		return nil, err
	}
	return index.Search(res.Objects, query), nil
}
