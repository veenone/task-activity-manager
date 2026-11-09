package main

import (
	"strings"
	"testing"

	"agile-suite/xtm/internal/testrepo"
)

func seedExecution(t *testing.T, a *App, profileID string) {
	t.Helper()
	if err := a.repo.UpsertTests(profileID, []testrepo.TestCase{{Key: "QA-1", ID: "1", Summary: "Login"}}); err != nil {
		t.Fatal(err)
	}
	if err := a.repo.UpsertContainers(profileID, []testrepo.Container{{Key: "QA-TE-1", Kind: "testexec", Summary: "Cycle 1"}}); err != nil {
		t.Fatal(err)
	}
	if err := a.repo.ReplaceAllContainerLinks(profileID, []testrepo.ContainerLink{{ContainerKey: "QA-TE-1", TestKey: "QA-1"}}); err != nil {
		t.Fatal(err)
	}
}

// A backend without JQL, and the demo profile, say so instead of returning
// every test or none (#446).
func TestContainerTestsMatchingSaysWhenThereIsNoJQL(t *testing.T) {
	a, demo := newTestAppWithDemoProfile(t)
	seedExecution(t, a, demo)
	if _, err := a.ContainerTestsMatching(demo, "QA-TE-1", "labels = smoke"); err == nil || !strings.Contains(err.Error(), "demo") {
		t.Fatalf("demo answered %v", err)
	}

	k, kiwi := newTestAppWithKiwiProfile(t)
	seedExecution(t, k, kiwi)
	if _, err := k.ContainerTestsMatching(kiwi, "QA-TE-1", "labels = smoke"); err == nil || !strings.Contains(err.Error(), "JQL") {
		t.Fatalf("kiwi answered %v", err)
	}

	if _, err := a.ContainerTestsMatching(demo, "QA-TE-1", "  "); err == nil {
		t.Fatal("an empty query was sent")
	}
}
