package issuerepo_test

import (
	"context"
	"testing"

	"agile-suite/tam/internal/backend"
)

func TestDraftIndexMapsTypeAndSummaryToKey(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	k1, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "Rotate keys"})
	if err != nil {
		t.Fatal(err)
	}
	k2, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeBug, Summary: "Trim input"})
	if err != nil {
		t.Fatal(err)
	}
	index, err := repo.DraftIndex(ctx, "p1")
	if err != nil {
		t.Fatalf("DraftIndex: %v", err)
	}
	if len(index) != 2 || index["task|rotate keys"] != k1 || index["bug|trim input"] != k2 {
		t.Errorf("index: %+v", index)
	}
	if other, err := repo.DraftIndex(ctx, "p2"); err != nil || len(other) != 0 {
		t.Errorf("profiles are isolated: %v %v", other, err)
	}
}
