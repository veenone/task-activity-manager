package issuerepo_test

import (
	"context"
	"testing"
	"time"
)

// TestUpsertPageStoresAssigneeEmail round-trips the assignee's email through
// the insert, the ON CONFLICT update and the row read.
func TestUpsertPageStoresAssigneeEmail(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 5, 10, 42, 0, 0, time.UTC)
	page := sample()
	page[0].AssigneeEmail = "r.anand@example.com"
	if err := r.UpsertPage(ctx, "p1", page, now, false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if got, _ := r.GetIssue(ctx, "p1", "PLAT-412"); got.AssigneeEmail != "r.anand@example.com" {
		t.Fatalf("assignee email after insert = %q", got.AssigneeEmail)
	}
	page[0].AssigneeEmail = "m.ortiz@example.com"
	if err := r.UpsertPage(ctx, "p1", page[:1], now.Add(time.Minute), false); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if got, _ := r.GetIssue(ctx, "p1", "PLAT-412"); got.AssigneeEmail != "m.ortiz@example.com" {
		t.Fatalf("assignee email after the second sync = %q", got.AssigneeEmail)
	}
}
