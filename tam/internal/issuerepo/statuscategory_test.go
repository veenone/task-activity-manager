package issuerepo_test

import (
	"context"
	"testing"
	"time"
)

// The status category rides the row so the chip colours from Jira's own
// bucket rather than from the status name. It has to refresh on a later
// sync like every other column, the ON CONFLICT trap version 14's
// assignee_name pins, and a row synced before the column existed reads back
// empty so the frontend falls back to guessing from the name.
func TestUpsertKeepsTheStatusCategoryAndRefreshesItOnASecondSync(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 5, 10, 42, 0, 0, time.UTC)
	page := sample()
	page[0].Status, page[0].StatusCategory = "En cours", "indeterminate"
	if err := r.UpsertPage(ctx, "p1", page, now, false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := r.GetIssue(ctx, "p1", "PLAT-412")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.StatusCategory != "indeterminate" {
		t.Errorf("status category = %q, want indeterminate", got.StatusCategory)
	}
	page[0].Status, page[0].StatusCategory = "Erledigt", "done"
	if err := r.UpsertPage(ctx, "p1", page[:1], now.Add(time.Minute), false); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if got, _ := r.GetIssue(ctx, "p1", "PLAT-412"); got.StatusCategory != "done" {
		t.Errorf("status category after the second sync = %q, want done", got.StatusCategory)
	}
	if got, _ := r.GetIssue(ctx, "p1", "PLAT-350"); got.StatusCategory != "" {
		t.Errorf("status category = %q, want empty for a row synced without one", got.StatusCategory)
	}
}
