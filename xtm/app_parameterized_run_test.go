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
	refreshed, err := a.GetParameterizedRun(profileID, "DEMO2-STE-1", "DEMO2-1", true)
	if err != nil || refreshed.Iterations[0].Status == "BLOCKED" {
		t.Fatalf("force-refreshed status = %q, err=%v", refreshed.Iterations[0].Status, err)
	}
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
