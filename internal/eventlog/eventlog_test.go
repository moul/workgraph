package eventlog

import (
	"testing"
	"time"
)

func TestAppendAndReadAll(t *testing.T) {
	root := t.TempDir()
	evs := []Event{
		{ID: "EVT-2", At: "2026-08-04T21:00:00Z", Actor: "human:moul", Action: "item.created", Object: "ITM-1"},
		{ID: "EVT-1", At: "2026-08-04T20:00:00Z", Actor: "human:moul", Action: "project.created", Object: "PRJ-1"},
		{ID: "EVT-3", At: "2026-09-01T10:00:00Z", Actor: "agent:claude", Action: "run.created", Object: "ITM-1", Run: "RUN-1"},
	}
	for _, e := range evs {
		if err := Append(root, e); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ReadAll(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d events, want 3", len(got))
	}
	// Sorted by At.
	if got[0].ID != "EVT-1" || got[1].ID != "EVT-2" || got[2].ID != "EVT-3" {
		t.Errorf("order = %s,%s,%s", got[0].ID, got[1].ID, got[2].ID)
	}
}

func TestForObjectAndRun(t *testing.T) {
	evs := []Event{
		{ID: "1", Object: "ITM-1"},
		{ID: "2", Object: "ITM-2"},
		{ID: "3", Object: "ITM-1", Run: "RUN-9"},
	}
	if got := ForObject(evs, "ITM-1"); len(got) != 2 {
		t.Errorf("ForObject = %d, want 2", len(got))
	}
	if got := ForRun(evs, "RUN-9"); len(got) != 1 {
		t.Errorf("ForRun = %d, want 1", len(got))
	}
}

func TestFileForMonthlyBucket(t *testing.T) {
	root := "/x"
	at := time.Date(2026, 8, 4, 21, 0, 0, 0, time.UTC)
	if got := FileFor(root, at); got != "/x/events/2026-08.jsonl" {
		t.Errorf("FileFor = %q", got)
	}
}

func TestAppendDefaultsTimestamp(t *testing.T) {
	root := t.TempDir()
	if err := Append(root, Event{ID: "EVT-x", Action: "item.created"}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadAll(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].At == "" {
		t.Fatalf("expected defaulted timestamp, got %+v", got)
	}
}
