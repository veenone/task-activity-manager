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
