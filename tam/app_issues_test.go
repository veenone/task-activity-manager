package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
)

// detailBackend answers GetIssueDetail with one canned detail, or fails, and
// counts the calls: what these tests are about is whether Jira is asked at
// all.
type detailBackend struct {
	stubIssueBackend
	detail backend.IssueDetail
	fail   bool
	calls  int
}

func (b *detailBackend) GetIssueDetail(context.Context, string) (backend.IssueDetail, error) {
	b.calls++
	if b.fail {
		return backend.IssueDetail{}, errors.New("dial tcp: no route to host")
	}
	return b.detail, nil
}

// seedCachedDetail puts one issue and one cached detail in the store, fetched
// at the moment given.
func seedCachedDetail(t *testing.T, a *App, profileID string, at time.Time) {
	t.Helper()
	ctx := context.Background()
	issues := []backend.Issue{{Key: "PLAT-412", ID: "1", Project: "PLAT", Type: "story", Summary: "Promo code", Status: "In Progress", Updated: "2026-09-05T09:58:00Z"}}
	if err := a.repo.UpsertPage(ctx, profileID, issues, at, false); err != nil {
		t.Fatalf("seed issues: %v", err)
	}
	cached := backend.IssueDetail{
		Description:  "As a shopper",
		Comments:     []backend.Comment{{ID: "1", AuthorName: "R. Anand", Body: "Blocked on the gateway sandbox."}},
		CommentTotal: 1,
	}
	if err := a.repo.WriteDetail(ctx, profileID, "PLAT-412", cached, at); err != nil {
		t.Fatalf("seed detail: %v", err)
	}
}

func TestGetIssueDetailServesTheCacheWhenTheBackendFails(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	seedCachedDetail(t, a, p.ID, time.Now().Add(-24*time.Hour))
	b := &detailBackend{fail: true}
	a.backends[p.ID] = b

	d, err := a.GetIssueDetail(p.ID, "PLAT-412")
	if err != nil {
		t.Fatalf("detail = %v, an offline panel keeps what it had", err)
	}
	if b.calls != 1 {
		t.Errorf("backend calls = %d, want the stale cache to have been refreshed first", b.calls)
	}
	if d.Description != "As a shopper" || len(d.Comments) != 1 {
		t.Errorf("detail = %+v, want the cached one", d)
	}
	if d.FetchedAt == "" {
		t.Error("fetchedAt is empty, so the panel cannot say how old this is")
	}
}

func TestGetIssueDetailFailsWhenThereIsNothingCached(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	seedCachedDetail(t, a, p.ID, time.Now())
	if err := a.repo.ClearDetails(context.Background(), p.ID); err != nil {
		t.Fatalf("clear: %v", err)
	}
	a.backends[p.ID] = &detailBackend{fail: true}
	if _, err := a.GetIssueDetail(p.ID, "PLAT-412"); err == nil {
		t.Error("a failed read with no cache behind it must be an error")
	}
}

func TestDetailCacheMinutesOfZeroNeverExpires(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	seedCachedDetail(t, a, p.ID, time.Now().Add(-24*time.Hour))
	b := &detailBackend{detail: backend.IssueDetail{Description: "from Jira"}}
	a.backends[p.ID] = b

	// The default is ten minutes, so a day-old detail is refetched.
	if _, err := a.GetIssueDetail(p.ID, "PLAT-412"); err != nil {
		t.Fatalf("detail: %v", err)
	}
	if b.calls != 1 {
		t.Fatalf("backend calls = %d, want the default freshness to have expired", b.calls)
	}
	if err := a.SetProfileSetting(p.ID, settingDetailCacheMinutes, "0"); err != nil {
		t.Fatalf("set the setting: %v", err)
	}
	// SetProfileSetting drops the cached backend, so put the counter back,
	// and age the cached detail again: what is being tested is a detail the
	// default would have thrown away.
	seedCachedDetail(t, a, p.ID, time.Now().Add(-24*time.Hour))
	a.backends[p.ID] = b
	d, err := a.GetIssueDetail(p.ID, "PLAT-412")
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if b.calls != 1 {
		t.Errorf("backend calls = %d, zero minutes means the cache never expires", b.calls)
	}
	if d.Description != "As a shopper" {
		t.Errorf("detail = %+v, want the day-old cached one", d)
	}
}
