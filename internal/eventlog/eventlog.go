// Package eventlog implements the append-only semantic event stream stored as
// JSON Lines under events/YYYY-MM.jsonl.
//
// Git is the forensic log; events are the operational log. Events are immutable
// facts and explain *how* the repo reached its current state. If an event and
// object frontmatter disagree, frontmatter wins for current state and
// validation reports the drift — events are never replayed blindly.
package eventlog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Dir is the events directory within a workspace.
const Dir = "events"

// Event is one immutable fact. Most fields are optional and depend on action.
type Event struct {
	ID       string `json:"id"`
	At       string `json:"at"`
	Actor    string `json:"actor"`
	Action   string `json:"action"`
	Object   string `json:"object,omitempty"`
	Project  string `json:"project,omitempty"`
	Run      string `json:"run,omitempty"`
	Worker   string `json:"worker,omitempty"`
	Round    int    `json:"round,omitempty"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	Status   string `json:"status,omitempty"`
	PR       string `json:"pr,omitempty"`
	Summary  string `json:"summary,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Message  string `json:"message,omitempty"`
	TokenID  string `json:"token_id,omitempty"`
	Artifact string `json:"artifact,omitempty"`
}

// FileFor returns the repo-relative JSONL path for an event timestamp.
func FileFor(root string, at time.Time) string {
	return filepath.Join(root, Dir, at.UTC().Format("2006-01")+".jsonl")
}

// Append writes ev to the monthly JSONL file, creating it if needed. The file
// is opened in append mode so concurrent appenders on the same host serialize
// at the OS level; cross-host coordination is handled by the git preflight.
func Append(root string, ev Event) error {
	if ev.At == "" {
		ev.At = time.Now().Format(time.RFC3339)
	}
	at, err := time.Parse(time.RFC3339, ev.At)
	if err != nil {
		at = time.Now()
	}
	path := FileFor(root, at)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("eventlog: mkdir: %w", err)
	}
	line, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("eventlog: marshal: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("eventlog: open: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("eventlog: write: %w", err)
	}
	return nil
}

// ReadAll returns every event across all monthly files, sorted by (At, ID).
func ReadAll(root string) ([]Event, error) {
	dir := filepath.Join(root, Dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("eventlog: readdir: %w", err)
	}
	var out []Event
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		evs, err := readFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, evs...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].At != out[j].At {
			return out[i].At < out[j].At
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func readFile(path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("eventlog: open %s: %w", path, err)
	}
	defer f.Close()
	var out []Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			return nil, fmt.Errorf("eventlog: %s:%d: %w", path, n, err)
		}
		out = append(out, ev)
	}
	return out, sc.Err()
}

// ForObject filters events referencing an object id (as Object).
func ForObject(evs []Event, objectID string) []Event {
	var out []Event
	for _, e := range evs {
		if e.Object == objectID {
			out = append(out, e)
		}
	}
	return out
}

// ForRun filters events referencing a run id.
func ForRun(evs []Event, runID string) []Event {
	var out []Event
	for _, e := range evs {
		if e.Run == runID {
			out = append(out, e)
		}
	}
	return out
}
