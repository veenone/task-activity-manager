package main

import (
	"testing"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/testrepo"
)

func TestApplyJUnitImportDoesNotFallbackWhenIterationWritesAreUnsupported(t *testing.T) {
	a, profileID := newTestAppWithKiwiProfile(t)
	match := testrepo.JUnitMatch{
		TestKey: "42", Result: "FAIL", TargetKind: testrepo.JUnitTargetIteration,
		IterationID: "iteration-1", Parameters: []backend.IterationParameter{{Name: "USER", Value: "alice"}},
	}
	if _, err := a.ApplyJUnitImport(profileID, "execution-1", []testrepo.JUnitMatch{match}); err == nil {
		t.Fatal("iteration import fell back to an overall run write")
	}
	if pending, _ := a.repo.ListPendingChanges(profileID); len(pending) != 0 {
		t.Fatalf("unsupported import created pending changes: %+v", pending)
	}
}
