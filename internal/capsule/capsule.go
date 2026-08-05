// Package capsule generates the task-scoped launch contract copied into a
// target repository at .workgraph/runs/RUN-.../.
//
// A capsule is a contract, not a memory dump. It carries only the task, a
// compact project summary, active constraints, relevant links, and the finish
// protocol. It never copies the target repo's own CLAUDE.md/AGENTS.md — those
// tell the agent *how* to work there; the capsule tells it *what* and *why*.
package capsule

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moul/workgraph/internal/model"
	"github.com/moul/workgraph/internal/redact"
)

// Run is the machine contract written as RUN.json.
type Run struct {
	RunID         string `json:"run_id"`
	ItemID        string `json:"item_id"`
	Round         int    `json:"round,omitempty"`
	Worker        string `json:"worker,omitempty"`
	Agent         string `json:"agent,omitempty"`
	ControlRepo   string `json:"control_repo,omitempty"`
	ControlRef    string `json:"control_ref,omitempty"`
	ObjectVersion string `json:"object_version,omitempty"`
	TargetRepo    string `json:"target_repo,omitempty"`
	TargetRef     string `json:"target_ref,omitempty"`
	TargetPath    string `json:"target_path,omitempty"`
	FinishCommand string `json:"finish_command"`
	BlockCommand  string `json:"block_command"`
}

// Data is everything needed to render a capsule.
type Data struct {
	Run       Run
	Item      *model.Item
	Project   *model.Project
	Decisions []*model.Decision // accepted decisions only
	DependsOn []*model.Item
	Redact    bool
}

// Budgets (in lines) keep capsules small enough to read directly.
const (
	promptBudget  = 200
	taskBudget    = 300
	projectBudget = 150
	linksBudget   = 100
)

// Dir returns the capsule directory within a target repo for a run id.
func Dir(targetRepo, runID string) string {
	return filepath.Join(targetRepo, ".workgraph", "runs", runID)
}

// Generate writes the capsule into targetRepo and returns the capsule dir.
func Generate(targetRepo string, d Data) (string, error) {
	dir := Dir(targetRepo, d.Run.RunID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("capsule: mkdir: %w", err)
	}
	files := map[string]string{
		"RUN.json":   mustJSON(d.Run),
		"PROMPT.md":  capLines(renderPrompt(d), promptBudget),
		"TASK.md":    capLines(renderTask(d), taskBudget),
		"PROJECT.md": capLines(renderProject(d), projectBudget),
		"LINKS.md":   capLines(renderLinks(d), linksBudget),
		"RULES.md":   renderRules(d),
		"RESULT.md":  renderResult(d),
	}
	for name, content := range files {
		// Redaction scrubs secret-shaped values before the capsule lands in a
		// target repo, which may be a different (even public) repository. It is
		// the default when the target is outside the control repo.
		if d.Redact && name != "RUN.json" {
			content, _ = redact.Redact(content)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			return "", fmt.Errorf("capsule: write %s: %w", name, err)
		}
	}
	return dir, nil
}

func mustJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b) + "\n"
}

func renderPrompt(d Data) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Run %s\n\n", d.Run.RunID)
	fmt.Fprintf(&b, "You are working on item **%s** — %s\n\n", d.Item.ID, d.Item.Title)
	b.WriteString("Read the files in this capsule directory, then complete the task:\n\n")
	b.WriteString("- `TASK.md` — the goal, context, acceptance criteria, and constraints.\n")
	b.WriteString("- `PROJECT.md` — compact project state.\n")
	b.WriteString("- `LINKS.md` — dependencies and accepted decisions.\n")
	b.WriteString("- `RULES.md` — how to report back and safety rules.\n\n")
	b.WriteString("Preserve this repository's own CLAUDE.md / AGENTS.md rules — they tell you\n")
	b.WriteString("*how* to work here. This capsule tells you *what* and *why*.\n\n")
	b.WriteString("## When you finish\n\n")
	fmt.Fprintf(&b, "Write your outcome to `RESULT.md`, then run:\n\n```bash\n%s\n```\n\n", d.Run.FinishCommand)
	fmt.Fprintf(&b, "If you are blocked:\n\n```bash\n%s\n```\n", d.Run.BlockCommand)
	return b.String()
}

func renderTask(d Data) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", d.Item.Title)
	fmt.Fprintf(&b, "- id: %s\n- kind: %s\n- status: %s\n- priority: %s\n", d.Item.ID, d.Item.Kind, d.Item.Status, d.Item.Priority)
	if d.Item.TargetRepo != "" {
		fmt.Fprintf(&b, "- target_repo: %s (%s)\n", d.Item.TargetRepo, orDefault(d.Item.TargetRef, "main"))
	}
	b.WriteString("\n")
	// Prefer explicit body sections; fall back to whole body.
	body := d.Item.Body()
	if secs := extractSections(body, "Goal", "Outcome", "Context", "Acceptance criteria", "Acceptance Criteria", "Constraints", "Handoff"); secs != "" {
		b.WriteString(secs)
	} else if strings.TrimSpace(body) != "" {
		b.WriteString(body)
	}
	return b.String()
}

func renderProject(d Data) string {
	if d.Project == nil {
		return "# Project\n\n(No project attached.)\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", d.Project.Title)
	fmt.Fprintf(&b, "- id: %s\n- status: %s\n- health: %s\n\n", d.Project.ID, d.Project.Status, orDefault(d.Project.Health, "unknown"))
	if secs := extractSections(d.Project.Body(), "Purpose", "Current outcome", "Current state", "Success criteria", "Constraints"); secs != "" {
		b.WriteString(secs)
	}
	return b.String()
}

func renderLinks(d Data) string {
	var b strings.Builder
	b.WriteString("# Links\n\n")
	if len(d.DependsOn) > 0 {
		b.WriteString("## Depends on\n\n")
		for _, dep := range d.DependsOn {
			fmt.Fprintf(&b, "- %s [%s] %s\n", dep.ID, dep.Status, dep.Title)
		}
		b.WriteString("\n")
	}
	if len(d.Decisions) > 0 {
		b.WriteString("## Accepted decisions\n\n")
		for _, dec := range d.Decisions {
			fmt.Fprintf(&b, "- %s %s\n", dec.ID, dec.Title)
		}
		b.WriteString("\n")
	}
	if len(d.DependsOn) == 0 && len(d.Decisions) == 0 {
		b.WriteString("(No blocking dependencies or decisions.)\n")
	}
	return b.String()
}

func renderRules(Data) string {
	return strings.TrimLeft(`
# Rules

## Reporting
- When done, write RESULT.md and run the finish command in RUN.json.
- If blocked, run the block command with a clear reason.
- Report outcomes, blockers, commands of record, test results, and PR links —
  never chain-of-thought.

## Safety
- Preserve this repository's own CLAUDE.md / AGENTS.md conventions.
- Do not commit secrets. Secret names and env var names are fine; values are not.
- Prefer small, reviewable commits.
- If the task conflicts with a repository rule, stop and report a blocker.
`, "\n")
}

func renderResult(d Data) string {
	return fmt.Sprintf("# Result for %s\n\n<!-- The worker writes the outcome here. -->\n\n## Summary\n\nNot produced yet.\n\n## Evidence\n\n- \n", d.Run.RunID)
}

// extractSections returns the named "## Heading" sections found in body, in the
// order requested, de-duplicated by heading text.
func extractSections(body string, headings ...string) string {
	var out strings.Builder
	seen := map[string]bool{}
	for _, h := range headings {
		key := strings.ToLower(h)
		if seen[key] {
			continue
		}
		seen[key] = true
		if sec := oneSection(body, h); sec != "" {
			out.WriteString(sec)
			out.WriteString("\n")
		}
	}
	return out.String()
}

func oneSection(body, heading string) string {
	lines := strings.Split(body, "\n")
	want := "## " + heading
	start := -1
	for i, ln := range lines {
		if strings.EqualFold(strings.TrimSpace(ln), want) {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## " + heading + "\n")
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			break
		}
		b.WriteString(lines[i])
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func capLines(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	kept := lines[:maxLines]
	return strings.Join(kept, "\n") + fmt.Sprintf("\n\n<!-- truncated: %d more lines; see the control repo for full context -->\n", len(lines)-maxLines)
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
