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

func TestResetSyncCursorClearsOnlyLastSynced(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	seed := issuerepo.SyncState{LastSynced: "2026-09-05T10:42:00Z", LastFull: "2026-09-01T08:00:00Z", LastError: "page 3: timeout"}
	if err := r.SetSyncState(ctx, "p1", seed); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := r.ResetSyncCursor(ctx, "p1"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	s, err := r.SyncState(ctx, "p1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if s.LastSynced != "" || s.LastFull != seed.LastFull || s.LastError != seed.LastError {
		t.Errorf("state = %+v, want only last_synced cleared", s)
	}
	if err := r.ResetSyncCursor(ctx, "p2"); err != nil {
		t.Fatalf("reset a profile with no state: %v", err)
	}
	if s, _ := r.SyncState(ctx, "p2"); s.LastSynced != "" {
		t.Errorf("p2 = %+v", s)
	}
}

func TestPurgeProfileDropsOnlyThatProfile(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	for _, id := range []string{"p1", "p2"} {
		if err := r.UpsertPage(ctx, id, sample(), time.Now(), false); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
		if err := r.SetSyncState(ctx, id, issuerepo.SyncState{LastSynced: "2026-09-05T10:42:00Z"}); err != nil {
			t.Fatalf("seed state %s: %v", id, err)
		}
		if err := r.SetProfileSetting(ctx, id, "requirement_issue_type", "Business Requirement"); err != nil {
			t.Fatalf("seed setting %s: %v", id, err)
		}
	}
	if err := r.PurgeProfile(ctx, "p1"); err != nil {
		t.Fatalf("purge: %v", err)
	}
	if n, err := r.CountIssues(ctx, "p1"); err != nil || n != 0 {
		t.Errorf("p1 issues = %d, %v; want 0", n, err)
	}
	if s, _ := r.SyncState(ctx, "p1"); s.LastSynced != "" || s.LastFull != "" || s.LastError != "" {
		t.Errorf("p1 state = %+v, want empty", s)
	}
	if v, _ := r.ProfileSetting(ctx, "p1", "requirement_issue_type"); v != "" {
		t.Errorf("p1 setting = %q, want empty", v)
	}
	if n, err := r.CountIssues(ctx, "p2"); err != nil || n != 4 {
		t.Errorf("p2 issues = %d, %v; want 4", n, err)
	}
	if s, _ := r.SyncState(ctx, "p2"); s.LastSynced != "2026-09-05T10:42:00Z" {
		t.Errorf("p2 state = %+v", s)
	}
	if v, _ := r.ProfileSetting(ctx, "p2", "requirement_issue_type"); v != "Business Requirement" {
		t.Errorf("p2 setting = %q", v)
	}
}
