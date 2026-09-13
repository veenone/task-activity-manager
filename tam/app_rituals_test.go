package main

import (
	"testing"

	"agile-suite/core/profile"
	"agile-suite/tam/internal/ritualrepo"
)

// newTestAppWithRituals builds on the shared newTestApp/newTestProfile
// helpers (app_boards_test.go) and wires the ritual repository the way
// initStore does in production. Neither existing helper does this on its
// own: newTestApp replicates initStore's field assignments by hand and
// predates the ritual store, so a test exercising it has to finish that
// wiring itself rather than trust a helper that cannot know about a field
// it was written before.
func newTestAppWithRituals(t *testing.T) (*App, profile.Profile) {
	t.Helper()
	a := newTestApp(t)
	p := newTestProfile(t, a)
	a.rituals = ritualrepo.New(a.local.DB())
	return a, p
}

func TestSavingARitualDraftStoresItAgainstTheSprint(t *testing.T) {
	a, p := newTestAppWithRituals(t)

	issues, err := ritualrepo.EncodeIssues([]ritualrepo.Issue{{Key: "PLAT-14", Remark: "demoed"}})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := a.SaveRitualDraft(p.ID, ritualrepo.Draft{
		BoardID: 1, SprintID: 12, RitualType: "review",
		Title: "Sprint 12 Review", Remark: "short sprint", IssuesJSON: issues,
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := a.GetRitualDraft(p.ID, 1, 12, "review")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Remark != "short sprint" {
		t.Fatalf("remark = %q", got.Remark)
	}
	if got.Status != "draft" {
		t.Fatalf("status = %q, want draft; saving must never publish", got.Status)
	}
	if got.UpdatedAt == "" {
		t.Fatal("updatedAt was not stamped")
	}
}

func TestARitualDraftNeedsAKnownRitualType(t *testing.T) {
	a, p := newTestAppWithRituals(t)

	err := a.SaveRitualDraft(p.ID, ritualrepo.Draft{BoardID: 1, SprintID: 12, RitualType: "party"})
	if err == nil {
		t.Fatal("expected an unknown ritual type to be refused")
	}
}

// TestSavingADraftCannotForgePublicationState is the proof the first test
// cannot give: a brand-new draft lands as "draft" whether or not the
// carry-from-stored-row logic exists at all, because there is nothing
// stored yet to compare against. This test seeds a real published row
// directly through the repository, then calls SaveRitualDraft with a
// caller-supplied draft that claims different values for every publication
// field, and checks the stored row afterwards still carries what was
// there before the call, not what the caller sent.
func TestSavingADraftCannotForgePublicationState(t *testing.T) {
	a, p := newTestAppWithRituals(t)

	published := ritualrepo.Draft{
		ProfileID: p.ID, BoardID: 1, SprintID: 12, RitualType: "review",
		Title: "Sprint 12 Review", Remark: "already published",
		ConfluencePageID: "42", ConfluenceVersion: 3, Body: "<p>real body</p>",
		Status: "published", UpdatedAt: "2025-06-01T00:00:00Z", PublishedAt: "2025-06-01T00:00:00Z",
	}
	if err := a.rituals.Upsert(a.ctx, published); err != nil {
		t.Fatalf("seed published row: %v", err)
	}

	forged := ritualrepo.Draft{
		BoardID: 1, SprintID: 12, RitualType: "review",
		Title: "Sprint 12 Review", Remark: "trying to forge",
		ConfluencePageID: "999", ConfluenceVersion: 7, Body: "forged",
		Status: "published", PublishedAt: "2026-01-01T00:00:00Z",
	}
	if err := a.SaveRitualDraft(p.ID, forged); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := a.GetRitualDraft(p.ID, 1, 12, "review")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != "draft" {
		t.Fatalf("status = %q, want draft; a save must never publish", got.Status)
	}
	if got.ConfluencePageID != "42" {
		t.Fatalf("confluencePageId = %q, want the stored 42, not the forged 999", got.ConfluencePageID)
	}
	if got.ConfluenceVersion != 3 {
		t.Fatalf("confluenceVersion = %d, want the stored 3, not the forged 7", got.ConfluenceVersion)
	}
	if got.Body != "<p>real body</p>" {
		t.Fatalf("body = %q, want the stored body, not the forged one", got.Body)
	}
	if got.PublishedAt != "2025-06-01T00:00:00Z" {
		t.Fatalf("publishedAt = %q, want the stored timestamp, not the forged one", got.PublishedAt)
	}
}

// TestSavingARitualDraftMatchesAnExistingRowRegardlessOfCase guards the case
// mismatch that used to look up the stored row by the caller's raw-case
// ritual type. ritual_type has no COLLATE NOCASE, so "Review" against a
// stored "review" row used to match nothing, and the save then overwrote a
// real publication field with the zero value.
func TestSavingARitualDraftMatchesAnExistingRowRegardlessOfCase(t *testing.T) {
	a, p := newTestAppWithRituals(t)

	if err := a.SaveRitualDraft(p.ID, ritualrepo.Draft{
		BoardID: 1, SprintID: 12, RitualType: "review",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	stored, err := a.rituals.Get(a.ctx, p.ID, 1, 12, "review")
	if err != nil {
		t.Fatalf("get stored: %v", err)
	}
	stored.ConfluencePageID = "42"
	if err := a.rituals.Upsert(a.ctx, stored); err != nil {
		t.Fatalf("seed page id: %v", err)
	}

	if err := a.SaveRitualDraft(p.ID, ritualrepo.Draft{
		BoardID: 1, SprintID: 12, RitualType: "Review",
		Remark: "case mismatch",
	}); err != nil {
		t.Fatalf("save with different case: %v", err)
	}

	got, err := a.GetRitualDraft(p.ID, 1, 12, "review")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ConfluencePageID != "42" {
		t.Fatalf("confluencePageId = %q, want the stored 42 to survive a differently-cased save", got.ConfluencePageID)
	}
}

func TestDeletingARitualDraftLeavesItsSiblings(t *testing.T) {
	a, p := newTestAppWithRituals(t)

	for _, ritualType := range []string{"planning", "review"} {
		if err := a.SaveRitualDraft(p.ID, ritualrepo.Draft{
			BoardID: 1, SprintID: 12, RitualType: ritualType,
		}); err != nil {
			t.Fatalf("save %s: %v", ritualType, err)
		}
	}
	if err := a.DeleteRitualDraft(p.ID, 1, 12, "review"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	drafts, err := a.ListRitualDrafts(p.ID, 1, 12)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(drafts) != 1 || drafts[0].RitualType != "planning" {
		t.Fatalf("drafts = %#v, want only planning", drafts)
	}
}
