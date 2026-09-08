package issuerepo_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

func seedTwo(t *testing.T, repo *issuerepo.Repository) {
	t.Helper()
	rows := []backend.Issue{
		{Key: "PLAT-1", ID: "1", Project: "PLAT", Type: backend.TypeTask, Summary: "one", Priority: "Medium", Labels: []string{"a"}, StoryPoints: pts(3), Updated: v1},
		{Key: "PLAT-2", ID: "2", Project: "PLAT", Type: backend.TypeTask, Summary: "two", Labels: []string{}, Updated: v1},
	}
	if err := repo.UpsertPage(context.Background(), "p1", rows, time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// The batch journals every change the way one EditField would, and reports
// each key it touched once, in the order it was given.
func TestEditFieldsJournalsEveryChangeAndNamesTheKeysItTouched(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedTwo(t, repo)

	touched, err := repo.EditFields(ctx, "p1", []issuerepo.Edit{
		{Key: "PLAT-2", Field: "summary", Value: "zwei"},
		{Key: "PLAT-2", Field: "priority", Value: "High"},
		{Key: "PLAT-1", Field: "summary", Value: "uno"},
	}, "imported from f.xlsx")
	if err != nil {
		t.Fatalf("EditFields: %v", err)
	}
	if len(touched) != 2 || touched[0] != "PLAT-2" || touched[1] != "PLAT-1" {
		t.Fatalf("touched = %v, want [PLAT-2 PLAT-1]", touched)
	}

	pend, _ := repo.ListPendingChanges(ctx, "p1")
	if len(pend) != 3 {
		t.Fatalf("journal: %+v", pend)
	}
	for _, p := range pend {
		if p.BaseVersion != v1 || p.EntityType != issuerepo.EntityIssue {
			t.Errorf("pending row: %+v", p)
		}
	}
	act, _ := repo.ListActivity(ctx, "p1", "PLAT-1", 0)
	if len(act) != 1 || act[0].Action != "edit" || act[0].Note != "imported from f.xlsx" {
		t.Errorf("audit: %+v", act)
	}
	if iss, _ := repo.GetIssue(ctx, "p1", "PLAT-2"); iss.Summary != "zwei" || iss.Priority != "High" || !iss.Pending {
		t.Errorf("row after batch: %+v", iss)
	}
}

// A value that already matches is not a change, so it is neither written nor
// reported. A batch of nothing but no-ops leaves the journal empty.
func TestEditFieldsSkipsValuesThatAlreadyMatch(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedTwo(t, repo)

	touched, err := repo.EditFields(ctx, "p1", []issuerepo.Edit{
		{Key: "PLAT-1", Field: "summary", Value: "one"},
		{Key: "PLAT-1", Field: "labels", Value: "a"},
		{Key: "PLAT-1", Field: "storyPoints", Value: "3"},
	}, "note")
	if err != nil {
		t.Fatalf("EditFields: %v", err)
	}
	if len(touched) != 0 {
		t.Errorf("touched = %v", touched)
	}
	if pend, _ := repo.ListPendingChanges(ctx, "p1"); len(pend) != 0 {
		t.Errorf("journal: %+v", pend)
	}
}

// One bad edit fails the whole batch: an import that half-applied would
// leave the user reconciling a spreadsheet against a journal by hand.
func TestEditFieldsIsAllOrNothing(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	seedTwo(t, repo)

	cases := []struct {
		name string
		edit issuerepo.Edit
		want string
	}{
		{"unknown issue", issuerepo.Edit{Key: "PLAT-9", Field: "summary", Value: "x"}, "PLAT-9"},
		{"unknown field", issuerepo.Edit{Key: "PLAT-2", Field: "status", Value: "Done"}, "cannot be edited"},
		{"bad points", issuerepo.Edit{Key: "PLAT-2", Field: "storyPoints", Value: "eight"}, "storyPoints"},
		{"parent is not an epic", issuerepo.Edit{Key: "PLAT-2", Field: "parentKey", Value: "PLAT-1"}, "PLAT-2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := repo.EditFields(ctx, "p1", []issuerepo.Edit{
				{Key: "PLAT-1", Field: "summary", Value: "uno"},
				tc.edit,
			}, "note")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want one naming %q", err, tc.want)
			}
			if iss, _ := repo.GetIssue(ctx, "p1", "PLAT-1"); iss.Summary != "one" {
				t.Errorf("the good edit landed anyway: %q", iss.Summary)
			}
			if pend, _ := repo.ListPendingChanges(ctx, "p1"); len(pend) != 0 {
				t.Errorf("journal: %+v", pend)
			}
		})
	}
}

func TestEditFieldsOnNoEditsIsANoop(t *testing.T) {
	repo := newRepo(t)
	touched, err := repo.EditFields(context.Background(), "p1", nil, "note")
	if err != nil || len(touched) != 0 {
		t.Errorf("EditFields(nil) = %v, %v", touched, err)
	}
}
