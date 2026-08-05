package core

import (
	"fmt"

	"github.com/moul/workgraph/internal/capsule"
	"github.com/moul/workgraph/internal/eventlog"
	"github.com/moul/workgraph/internal/id"
	"github.com/moul/workgraph/internal/model"
)

// RunResult is returned by CreateRun.
type RunResult struct {
	Run        capsule.Run
	CapsuleDir string // "" when no target repo was provided
	Round      int
}

// CreateRun starts a new work round against an item: it claims the item
// (single parallel policy), appends run.created + run.started, and — when
// targetRepo is a local path — writes a launch capsule there.
func (e *Engine) CreateRun(itemRef, worker, agent, targetRepoPath string) (*RunResult, error) {
	if _, err := e.preflight(true); err != nil {
		return nil, err
	}
	it, g, err := e.resolveItem(itemRef)
	if err != nil {
		return nil, err
	}
	if worker == "" {
		worker = e.Opt.Actor
	}

	// Conflict-free claim: a "single" item must not have two active owners.
	policy := it.ParallelPolicy
	if policy == "" {
		policy = "single"
	}
	if policy == "single" && it.RunID != "" && it.Status == model.StatusInProgress {
		return nil, fmt.Errorf("core: item %s already has an active run %s (parallel_policy=single)", it.ID, it.RunID)
	}

	round := countRounds(e, it.ID) + 1
	runID := id.NewRun()

	// Resolve project + accepted decisions + dependency items for the capsule.
	var proj *model.Project
	if it.Project != "" {
		if o := g.ByID(it.Project); o != nil {
			proj, _ = o.(*model.Project)
		}
	}
	var decisions []*model.Decision
	for _, d := range g.Decisions {
		if d.Status == "accepted" && (d.Project == "" || d.Project == it.Project) {
			decisions = append(decisions, d)
		}
	}
	var deps []*model.Item
	for _, dep := range it.DependsOn {
		if o := g.ByID(dep); o != nil {
			if di, ok := o.(*model.Item); ok {
				deps = append(deps, di)
			}
		}
	}

	run := capsule.Run{
		RunID:         runID,
		ItemID:        it.ID,
		Round:         round,
		Worker:        worker,
		Agent:         agent,
		ControlRepo:   e.WS.Config.WorkspaceID,
		ControlRef:    e.WS.Config.DefaultBranch,
		ObjectVersion: e.ItemVersion(it),
		TargetRepo:    it.TargetRepo,
		TargetRef:     orElse(it.TargetRef, "main"),
		TargetPath:    it.TargetPath,
		// Embed the control-repo path with -C so the commands are copy-paste
		// runnable from inside the target repo, where the worker actually is.
		FinishCommand: fmt.Sprintf("workgraph -C %s finish %s --status review --summary .workgraph/runs/%s/RESULT.md", e.WS.Root, runID, runID),
		BlockCommand:  fmt.Sprintf("workgraph -C %s block %s --reason \"...\"", e.WS.Root, runID),
	}

	// Claim the item.
	if _, err := e.UpdateItem(it.ID, func(i *model.Item) {
		i.Status = model.StatusInProgress
		i.Owner = worker
		i.RunID = runID
		i.ClaimedAt = now()
	}, "", "claim for run "+runID); err != nil {
		return nil, err
	}

	if err := e.appendEvent(eventlog.Event{ID: id.NewEvent(), Action: "run.created", Object: it.ID, Project: it.Project, Run: runID, Worker: worker, Round: round, Message: "Run created"}); err != nil {
		return nil, err
	}
	if err := e.appendEvent(eventlog.Event{ID: id.NewEvent(), Action: "run.started", Object: it.ID, Project: it.Project, Run: runID, Worker: worker, Round: round}); err != nil {
		return nil, err
	}

	res := &RunResult{Run: run, Round: round}
	if targetRepoPath != "" {
		dir, err := capsule.Generate(targetRepoPath, capsule.Data{
			Run:       run,
			Item:      it,
			Project:   proj,
			Decisions: decisions,
			DependsOn: deps,
		})
		if err != nil {
			return res, err
		}
		res.CapsuleDir = dir
	}

	if _, err := e.commit(fmt.Sprintf("run(%s): start round %d on %s", runID, round, it.ID)); err != nil {
		return res, err
	}
	return res, nil
}

// Finish completes a run: it appends run.finished and moves the item to the
// resulting status (review/done/blocked/...).
func (e *Engine) Finish(runID, status, summary, pr string) (*model.Item, error) {
	if status == "" {
		status = model.StatusReview
	}
	if !e.WS.Ontology.Has("item_status", status) {
		return nil, fmt.Errorf("core: unknown status %q", status)
	}
	it, err := e.itemForRun(runID)
	if err != nil {
		return nil, err
	}
	if err := e.appendEvent(eventlog.Event{ID: id.NewEvent(), Action: "run.finished", Object: it.ID, Project: it.Project, Run: runID, Status: status, Summary: summary, PR: pr}); err != nil {
		return nil, err
	}
	return e.UpdateItem(it.ID, func(i *model.Item) {
		i.Status = status
		i.RunID = ""
		i.ClaimedAt = ""
		i.LeaseUntil = ""
		if status == model.StatusDone {
			i.CompletedAt = now()
		}
	}, "", "finish run "+runID+" -> "+status)
}

// Block marks a run (and its item) blocked with a reason.
func (e *Engine) Block(runID, reason string) (*model.Item, error) {
	it, err := e.itemForRun(runID)
	if err != nil {
		return nil, err
	}
	if err := e.appendEvent(eventlog.Event{ID: id.NewEvent(), Action: "run.blocked", Object: it.ID, Project: it.Project, Run: runID, Reason: reason}); err != nil {
		return nil, err
	}
	return e.UpdateItem(it.ID, func(i *model.Item) {
		i.Status = model.StatusBlocked
	}, "", "block run "+runID+": "+reason)
}

// Heartbeat records progress for a long-running run.
func (e *Engine) Heartbeat(runID, message string) error {
	it, err := e.itemForRun(runID)
	if err != nil {
		return err
	}
	if err := e.appendEvent(eventlog.Event{ID: id.NewEvent(), Action: "run.heartbeat", Object: it.ID, Run: runID, Message: message}); err != nil {
		return err
	}
	_, err = e.commit("run(" + runID + "): heartbeat")
	return err
}

// itemForRun finds the item that a run belongs to, using events.
func (e *Engine) itemForRun(runID string) (*model.Item, error) {
	evs, err := eventlog.ReadAll(e.WS.Root)
	if err != nil {
		return nil, err
	}
	var itemID string
	for _, ev := range eventlog.ForRun(evs, runID) {
		if ev.Object != "" {
			itemID = ev.Object
			break
		}
	}
	if itemID == "" {
		return nil, fmt.Errorf("core: no item found for run %s", runID)
	}
	g, err := e.WS.Load()
	if err != nil {
		return nil, err
	}
	o := g.ByID(itemID)
	if o == nil {
		return nil, fmt.Errorf("core: item %s for run %s no longer exists", itemID, runID)
	}
	it, ok := o.(*model.Item)
	if !ok {
		return nil, fmt.Errorf("core: %s is not an item", itemID)
	}
	return it, nil
}

func countRounds(e *Engine, itemID string) int {
	evs, err := eventlog.ReadAll(e.WS.Root)
	if err != nil {
		return 0
	}
	seen := map[string]bool{}
	for _, ev := range evs {
		if ev.Object == itemID && ev.Action == "run.created" && ev.Run != "" {
			seen[ev.Run] = true
		}
	}
	return len(seen)
}

func orElse(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
