package main

import (
	"reflect"
	"strings"
	"testing"
)

func newTestAppWithDemo2Profile(t *testing.T) (*App, string) {
	t.Helper()
	a := newTestApp(t)
	p, err := a.CreateProfile("Demo2", "demo", "DEMO2", "", "", "", "", "tok", "", false, "xray")
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	return a, p.ID
}

func TestGetParameterizedRunBuildsStableResolvedDemo2Iterations(t *testing.T) {
	a, profileID := newTestAppWithDemo2Profile(t)
	first, err := a.GetParameterizedRun(profileID, "DEMO2-STE-1", "DEMO2-1", false)
	if err != nil {
		t.Fatalf("GetParameterizedRun: %v", err)
	}
	if len(first.Iterations) != 6 {
		t.Fatalf("iterations = %d, want 6", len(first.Iterations))
	}
	statuses := map[string]bool{}
	for _, iteration := range first.Iterations {
		statuses[iteration.Status] = true
		if iteration.ID == "" || len(iteration.Parameters) != 3 || len(iteration.Steps) == 0 {
			t.Fatalf("incomplete iteration: %+v", iteration)
		}
		for _, step := range iteration.Steps {
			resolved := step.Action + step.Data + step.Expected
			if strings.Contains(resolved, "${") {
				t.Errorf("step still contains a macro: %+v", step)
			}
		}
	}
	for _, want := range []string{"PASS", "FAIL", "TO" + "DO"} {
		if !statuses[want] {
			t.Errorf("statuses = %v, missing %s", statuses, want)
		}
	}
	again, err := a.GetParameterizedRun(profileID, "DEMO2-STE-1", "DEMO2-1", true)
	if err != nil || !reflect.DeepEqual(again, first) {
		t.Fatalf("repeated source read changed: err=%v\nfirst=%+v\nagain=%+v", err, first, again)
	}
}

func TestParameterizedRunCacheAndForceRefresh(t *testing.T) {
	a, profileID := newTestAppWithDemo2Profile(t)
	run, err := a.GetParameterizedRun(profileID, "DEMO2-STE-1", "DEMO2-1", false)
	if err != nil {
		t.Fatalf("prime: %v", err)
	}
	iterationID := run.Iterations[0].ID
	if err := a.SetIterationStatus(profileID, "DEMO2-STE-1", "DEMO2-1", iterationID, "BLOCKED"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	cached, err := a.GetParameterizedRun(profileID, "DEMO2-STE-1", "DEMO2-1", false)
	if err != nil || cached.Iterations[0].Status != "BLOCKED" {
		t.Fatalf("cached status = %q, err=%v", cached.Iterations[0].Status, err)
	}
	if !cached.Iterations[0].Uncommitted {
		t.Error("an edited result is not marked uncommitted, so it reads as a value the backend holds")
	}

	// This assertion is the reverse of what it was. It used to require that a
	// force refresh discarded the local edit, which is the bug #364 describes:
	// results are journalled and pushed on Commit (#366), so a refresh brings
	// the backend's view of everything else and leaves an uncommitted result
	// alone. Discarding it here is how a step result silently vanished.
	refreshed, err := a.GetParameterizedRun(profileID, "DEMO2-STE-1", "DEMO2-1", true)
	if err != nil {
		t.Fatalf("force refresh: %v", err)
	}
	if refreshed.Iterations[0].Status != "BLOCKED" {
		t.Errorf("force-refreshed status = %q, want the uncommitted BLOCKED to survive", refreshed.Iterations[0].Status)
	}
	if !refreshed.Iterations[0].Uncommitted {
		t.Error("the surviving result is not marked uncommitted after a refresh")
	}
}

// Picking a result journals it rather than sending it, so it shows up in
// Pending changes like every other edit (#366).
func TestSetIterationStatusJournalsForCommit(t *testing.T) {
	a, profileID := newTestAppWithDemo2Profile(t)
	run, err := a.GetParameterizedRun(profileID, "DEMO2-STE-1", "DEMO2-1", false)
	if err != nil {
		t.Fatalf("prime: %v", err)
	}
	iterationID := run.Iterations[0].ID
	if err := a.SetIterationStatus(profileID, "DEMO2-STE-1", "DEMO2-1", iterationID, "BLOCKED"); err != nil {
		t.Fatalf("edit: %v", err)
	}

	pending, err := a.ListPendingChanges(profileID)
	if err != nil {
		t.Fatalf("list pending changes: %v", err)
	}
	want := "DEMO2-STE-1:DEMO2-1:" + iterationID
	for _, change := range pending {
		if change.EntityType == "test_run_iteration" && change.EntityKey == want {
			if change.AfterVal != "BLOCKED" {
				t.Errorf("journalled %q, want BLOCKED", change.AfterVal)
			}
			return
		}
	}
	t.Fatalf("no pending change for %s; a pick that journals nothing cannot be committed or discarded", want)
}

func TestParameterizedRunBindingsRejectIncompleteIdentity(t *testing.T) {
	a, profileID := newTestAppWithDemo2Profile(t)
	for _, tc := range []struct{ profile, exec, test string }{
		{"", "DEMO2-STE-1", "DEMO2-1"},
		{profileID, "", "DEMO2-1"},
		{profileID, "DEMO2-STE-1", ""},
	} {
		if _, err := a.GetParameterizedRun(tc.profile, tc.exec, tc.test, false); err == nil {
			t.Errorf("GetParameterizedRun(%q, %q, %q) succeeded", tc.profile, tc.exec, tc.test)
		}
	}
	if err := a.SetIterationStepStatus(profileID, "DEMO2-STE-1", "DEMO2-1", "", "step-1", "PASS"); err == nil {
		t.Fatal("SetIterationStepStatus accepted a blank iteration id")
	}
}
