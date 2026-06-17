package testrepo_test

import (
	"testing"

	"xray-test-manager/internal/testrepo"
)

func TestCreateBugForTestQueuesAndDiscardRestores(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{{Key: "QA-1", ID: "1", Summary: "Login"}}); err != nil {
		t.Fatalf("seed test: %v", err)
	}

	key, err := repo.CreateBugForTest("p1", "QA-1", "QA-TE-1", testrepo.BugDraft{
		ProjectKey: "QA", Summary: "Login crashes", Description: "...", Priority: "High",
		Labels: []string{"regression"},
	})
	if err != nil {
		t.Fatalf("create bug: %v", err)
	}
	if len(key) < 8 || key[:8] != "NEW-BUG-" {
		t.Fatalf("temp key = %q, want NEW-BUG-*", key)
	}

	bugs, _ := repo.GetTestBugs("p1", "QA-1")
	if len(bugs) != 1 || bugs[0].Key != key || bugs[0].Summary != "Login crashes" {
		t.Fatalf("GetTestBugs = %+v, want one new bug", bugs)
	}
	changes, _ := repo.ListPendingChanges("p1")
	var id int64
	var n int
	for _, c := range changes {
		if c.EntityType == "bug_create" && c.EntityKey == key {
			n++
			id = c.ID
		}
	}
	if n != 1 {
		t.Fatalf("bug_create rows = %d, want 1", n)
	}

	if err := repo.DiscardPendingChange("p1", id); err != nil {
		t.Fatalf("discard: %v", err)
	}
	if bugs, _ := repo.GetTestBugs("p1", "QA-1"); len(bugs) != 0 {
		t.Errorf("after discard GetTestBugs = %+v, want none", bugs)
	}
}

func TestRenameBugRepointsCacheAndLinks(t *testing.T) {
	repo := newRepo(t)
	_ = repo.UpsertTests("p1", []testrepo.TestCase{{Key: "QA-1", ID: "1"}})
	key, _ := repo.CreateBugForTest("p1", "QA-1", "QA-TE-1", testrepo.BugDraft{ProjectKey: "QA", Summary: "x"})

	if err := repo.RenameBug("p1", key, "QA-500"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	bugs, _ := repo.GetTestBugs("p1", "QA-1")
	if len(bugs) != 1 || bugs[0].Key != "QA-500" {
		t.Errorf("after rename = %+v, want QA-500", bugs)
	}
}
