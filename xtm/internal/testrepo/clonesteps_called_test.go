package testrepo_test

import (
	"strings"
	"testing"

	"agile-suite/xtm/internal/testrepo"
)

// A step that calls another test has no action, data or expected result; its
// content is the called test's key. Cloning copied only those three fields, so
// the clone was an empty manual step (#487).
func TestCloneTestStepsKeepsACalledTestStep(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "source"},
		{Key: "QA-2", ID: "2", Summary: "target"},
		{Key: "QA-9", ID: "9", Summary: "shared login"},
	}); err != nil {
		t.Fatalf("seed tests: %v", err)
	}
	source := []testrepo.Step{
		{XrayID: "s1", Index: 1, CalledTestKey: "QA-9"},
		{XrayID: "s2", Index: 2, Action: "open cart", Data: "id=5", Expected: "cart"},
	}

	steps, err := repo.CloneTestSteps("p1", "QA-2", source)
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("steps after clone = %+v, want two", steps)
	}
	if steps[0].CalledTestKey != "QA-9" {
		t.Errorf("first cloned step = %+v, want a call to QA-9", steps[0])
	}
	if steps[1].Action != "open cart" || steps[1].Data != "id=5" || steps[1].Expected != "cart" {
		t.Errorf("second cloned step = %+v, want the manual step unchanged", steps[1])
	}

	// The queued add is what the commit sends, so it has to name the call too.
	changes, err := repo.ListPendingChanges("p1")
	if err != nil {
		t.Fatalf("read pending: %v", err)
	}
	calls := 0
	for _, c := range changes {
		if c.EntityType == "test_step_add" && strings.Contains(c.AfterVal, "QA-9") {
			calls++
		}
	}
	if calls != 1 {
		t.Errorf("queued step adds naming QA-9 = %d, want 1 (pending = %+v)", calls, changes)
	}
}
