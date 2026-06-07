package testrepo_test

import (
	"testing"

	"xray-test-manager/internal/testrepo"
)

func seedReviewRepo(t *testing.T) *testrepo.Repository {
	t.Helper()
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{{Key: "QA-1", ID: "1"}}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return repo
}

func TestSetTestReviewRecordsVerdictAndQueues(t *testing.T) {
	repo := seedReviewRepo(t)

	if err := repo.SetTestReview("p1", "QA-1", "approved", "Alice", "looks good"); err != nil {
		t.Fatalf("set review: %v", err)
	}

	rv, _ := repo.GetTestReview("p1", "QA-1")
	if rv.Verdict != "approved" || rv.Reviewer != "Alice" || rv.Note != "looks good" {
		t.Errorf("review = %+v, want approved/Alice/looks good", rv)
	}
	if rv.ReviewedAt == "" {
		t.Error("reviewedAt should be stamped")
	}

	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 1 || changes[0].EntityType != "test_review" {
		t.Fatalf("pending = %+v, want one test_review", changes)
	}
}

func TestSetTestReviewRejectsUnknownVerdict(t *testing.T) {
	repo := seedReviewRepo(t)
	if err := repo.SetTestReview("p1", "QA-1", "maybe", "", ""); err == nil {
		t.Error("unknown verdict should error")
	}
}

func TestClearTestReviewRemovesIt(t *testing.T) {
	repo := seedReviewRepo(t)
	if err := repo.SetTestReview("p1", "QA-1", "rejected", "Bob", "needs steps"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := repo.SetTestReview("p1", "QA-1", "", "", ""); err != nil {
		t.Fatalf("clear: %v", err)
	}
	rv, _ := repo.GetTestReview("p1", "QA-1")
	if rv.Verdict != "" {
		t.Errorf("verdict = %q after clear, want empty", rv.Verdict)
	}
}

func TestDiscardReviewRestoresPrevious(t *testing.T) {
	repo := seedReviewRepo(t)
	// Establish a committed-ish baseline by setting then committing locally is
	// not available here, so simulate: first review, discard should remove it.
	if err := repo.SetTestReview("p1", "QA-1", "approved", "Alice", ""); err != nil {
		t.Fatalf("set: %v", err)
	}
	changes, _ := repo.ListPendingChanges("p1")
	if err := repo.DiscardPendingChange("p1", changes[0].ID); err != nil {
		t.Fatalf("discard: %v", err)
	}
	rv, _ := repo.GetTestReview("p1", "QA-1")
	if rv.Verdict != "" {
		t.Errorf("verdict = %q after discarding the first review, want empty", rv.Verdict)
	}
}
