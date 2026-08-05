package store

import "testing"

func TestPlanAtCurrent(t *testing.T) {
	steps, err := Plan(CurrentSchema)
	if err != nil || len(steps) != 0 {
		t.Fatalf("Plan(current) = %v, %v; want 0 steps, nil", steps, err)
	}
	if s, _ := Plan(""); len(s) != 0 {
		t.Errorf("Plan(empty) should treat as current")
	}
}

func TestPlanUnknownVersion(t *testing.T) {
	if _, err := Plan("0.0"); err == nil {
		t.Fatal("expected an error for a version with no migration path")
	}
}
