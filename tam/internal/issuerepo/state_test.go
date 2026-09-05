package issuerepo_test

import (
	"context"
	"testing"
	"time"

	"agile-suite/tam/internal/issuerepo"
)

func TestSyncStateRoundTripsAndCountsIssues(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	s, err := r.SyncState(ctx, "p1")
	if err != nil || s.LastSynced != "" || s.IssueCount != 0 {
		t.Fatalf("empty state = %+v, %v", s, err)
	}
	if err := r.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	want := issuerepo.SyncState{LastSynced: "2026-09-05T10:42:00Z", LastFull: "2026-09-01T08:00:00Z", LastError: ""}
	if err := r.SetSyncState(ctx, "p1", want); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := r.SetSyncState(ctx, "p1", issuerepo.SyncState{LastSynced: want.LastSynced, LastFull: want.LastFull, LastError: "page 3: timeout"}); err != nil {
		t.Fatalf("set again: %v", err)
	}
	s, err = r.SyncState(ctx, "p1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if s.LastSynced != want.LastSynced || s.LastFull != want.LastFull || s.LastError != "page 3: timeout" || s.IssueCount != 4 {
		t.Errorf("state = %+v", s)
	}
	other, _ := r.SyncState(ctx, "p2")
	if other.LastSynced != "" {
		t.Errorf("state leaked across profiles: %+v", other)
	}
}

func TestProfileSettingsAreScopedAndOverwritten(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if v, err := r.ProfileSetting(ctx, "p1", "requirement_issue_type"); err != nil || v != "" {
		t.Fatalf("unset = %q, %v", v, err)
	}
	if err := r.SetProfileSetting(ctx, "p1", "requirement_issue_type", "Requirement"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := r.SetProfileSetting(ctx, "p1", "requirement_issue_type", "Business Requirement"); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	if v, _ := r.ProfileSetting(ctx, "p1", "requirement_issue_type"); v != "Business Requirement" {
		t.Errorf("p1 = %q", v)
	}
	if v, _ := r.ProfileSetting(ctx, "p2", "requirement_issue_type"); v != "" {
		t.Errorf("p2 = %q, want unset", v)
	}
}
