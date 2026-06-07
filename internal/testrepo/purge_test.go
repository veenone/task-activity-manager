package testrepo_test

import (
	"testing"

	"xray-test-manager/internal/testrepo"
)

func TestPurgeProfileRemovesOnlyThatProfilesData(t *testing.T) {
	repo := newRepo(t)
	for _, p := range []string{"p1", "p2"} {
		if err := repo.UpsertTests(p, []testrepo.TestCase{
			{Key: "QA-1", ID: "1", Summary: "t"},
		}); err != nil {
			t.Fatalf("seed %s: %v", p, err)
		}
		if err := repo.UpsertContainers(p, []testrepo.Container{
			{Key: "QA-TP-1", Kind: "testplan", Summary: "plan"},
		}); err != nil {
			t.Fatalf("seed container %s: %v", p, err)
		}
		if err := repo.SetTestReview(p, "QA-1", "approved", "x", ""); err != nil {
			t.Fatalf("seed review %s: %v", p, err)
		}
	}

	if err := repo.PurgeProfile("p1"); err != nil {
		t.Fatalf("purge: %v", err)
	}

	p1, _ := repo.ListTests("p1", testrepo.Query{})
	if p1.Total != 0 {
		t.Errorf("p1 tests = %d after purge, want 0", p1.Total)
	}
	if c, _ := repo.ListContainers("p1", "testplan"); len(c) != 0 {
		t.Errorf("p1 containers remain after purge: %v", c)
	}
	if rv, _ := repo.GetTestReview("p1", "QA-1"); rv.Verdict != "" {
		t.Errorf("p1 review remains after purge: %+v", rv)
	}
	if ch, _ := repo.ListPendingChanges("p1"); len(ch) != 0 {
		t.Errorf("p1 pending changes remain after purge: %v", ch)
	}

	// p2 must be untouched.
	p2, _ := repo.ListTests("p2", testrepo.Query{})
	if p2.Total != 1 {
		t.Errorf("p2 tests = %d after purging p1, want 1", p2.Total)
	}
}
