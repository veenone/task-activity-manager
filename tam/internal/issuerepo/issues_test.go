package issuerepo_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/tamstore"
)

func newRepo(t *testing.T) *issuerepo.Repository {
	t.Helper()
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return issuerepo.New(db.DB())
}

func pts(v float64) *float64 { return &v }

func sample() []backend.Issue {
	return []backend.Issue{
		{Key: "PLAT-412", ID: "1", Project: "PLAT", Type: "story", Summary: "Checkout: apply promo code", Status: "In Progress", Assignee: "R. Anand", Labels: []string{"checkout", "promo"}, SprintID: "12", SprintName: "Sprint 12", StoryPoints: pts(5), Rank: "0|i0002:", Updated: "2026-09-05T09:58:00Z"},
		{Key: "PLAT-409", ID: "2", Project: "PLAT", Type: "task", Summary: "Rotate payment gateway API keys", Status: "To Do", SprintID: "12", SprintName: "Sprint 12", StoryPoints: pts(2), Rank: "0|i0001:", Updated: "2026-09-04T10:00:00Z"},
		{Key: "PLAT-350", ID: "3", Project: "PLAT", Type: "epic", Summary: "Promotions and discounts", Status: "In Progress", Assignee: "PO", Labels: []string{"promo"}, StoryPoints: pts(21), Rank: "", Updated: "2026-09-01T10:00:00Z"},
		{Key: "PLAT-347", ID: "4", Project: "PLAT", Type: "task", Summary: "Write retro notes template", Status: "To Do", Assignee: "S. Kim", SprintID: "13", SprintName: "Sprint 13", Rank: "0|i0003:", Updated: "2026-09-03T10:00:00Z"},
	}
}

func TestUpsertPageInsertsThenUpdatesWithoutDuplicates(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 5, 10, 42, 0, 0, time.UTC)
	if err := r.UpsertPage(ctx, "p1", sample(), now, false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	changed := sample()[:1]
	changed[0].Summary = "Checkout: apply promo code at payment step"
	if err := r.UpsertPage(ctx, "p1", changed, now.Add(time.Minute), false); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	n, err := r.CountIssues(ctx, "p1")
	if err != nil || n != 4 {
		t.Fatalf("count = %d, %v; want 4", n, err)
	}
	got, err := r.GetIssue(ctx, "p1", "PLAT-412")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Summary != "Checkout: apply promo code at payment step" || got.Labels[1] != "promo" || *got.StoryPoints != 5 {
		t.Errorf("row = %+v", got)
	}
	if _, err := r.GetIssue(ctx, "p1", "PLAT-1"); !errors.Is(err, issuerepo.ErrNotFound) {
		t.Errorf("missing key err = %v", err)
	}
	if _, err := r.GetIssue(ctx, "other", "PLAT-412"); !errors.Is(err, issuerepo.ErrNotFound) {
		t.Errorf("rows are scoped by profile: err = %v", err)
	}
}

func TestListIssuesOrdersByRankThenKeyAndPages(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	page, err := r.ListIssues(ctx, "p1", issuerepo.IssueQuery{Limit: 3})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if page.Total != 4 || len(page.Issues) != 3 {
		t.Fatalf("page = total %d, %d rows", page.Total, len(page.Issues))
	}
	want := []string{"PLAT-409", "PLAT-412", "PLAT-347"}
	for i, k := range want {
		if page.Issues[i].Key != k {
			t.Errorf("row %d = %s, want %s", i, page.Issues[i].Key, k)
		}
	}
	page, err = r.ListIssues(ctx, "p1", issuerepo.IssueQuery{Offset: 3, Limit: 3})
	if err != nil || len(page.Issues) != 1 || page.Issues[0].Key != "PLAT-350" {
		t.Errorf("last page = %+v, %v (the empty rank sorts last)", page.Issues, err)
	}
}

func TestListIssuesFilters(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	cases := []struct {
		name string
		q    issuerepo.IssueQuery
		want []string
	}{
		{"by type", issuerepo.IssueQuery{Types: []string{"task"}}, []string{"PLAT-409", "PLAT-347"}},
		{"by two types", issuerepo.IssueQuery{Types: []string{"epic", "story"}}, []string{"PLAT-412", "PLAT-350"}},
		{"by sprint", issuerepo.IssueQuery{SprintID: "12"}, []string{"PLAT-409", "PLAT-412"}},
		{"by key text", issuerepo.IssueQuery{Text: "plat-35"}, []string{"PLAT-350"}},
		{"by summary text", issuerepo.IssueQuery{Text: "retro"}, []string{"PLAT-347"}},
		{"by label text", issuerepo.IssueQuery{Text: "promo"}, []string{"PLAT-412", "PLAT-350"}},
		{"type and sprint", issuerepo.IssueQuery{Types: []string{"task"}, SprintID: "13"}, []string{"PLAT-347"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			page, err := r.ListIssues(ctx, "p1", c.q)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if page.Total != len(c.want) {
				t.Fatalf("total = %d, want %d", page.Total, len(c.want))
			}
			for i, k := range c.want {
				if page.Issues[i].Key != k {
					t.Errorf("row %d = %s, want %s", i, page.Issues[i].Key, k)
				}
			}
		})
	}
}

func TestClearFirstReplacesTheProfileOnly(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatalf("seed p1: %v", err)
	}
	if err := r.UpsertPage(ctx, "p2", sample()[:1], time.Now(), false); err != nil {
		t.Fatalf("seed p2: %v", err)
	}
	if err := r.UpsertPage(ctx, "p1", sample()[3:], time.Now(), true); err != nil {
		t.Fatalf("replace: %v", err)
	}
	n, _ := r.CountIssues(ctx, "p1")
	if n != 1 {
		t.Errorf("p1 count after clear = %d, want 1", n)
	}
	n, _ = r.CountIssues(ctx, "p2")
	if n != 1 {
		t.Errorf("p2 count = %d, want 1 (untouched)", n)
	}
}

func TestListSprintsIsDistinctAndSorted(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	sprints, err := r.ListSprints(ctx, "p1")
	if err != nil {
		t.Fatalf("sprints: %v", err)
	}
	if len(sprints) != 2 || sprints[0].ID != "12" || sprints[0].Name != "Sprint 12" || sprints[1].ID != "13" {
		t.Errorf("sprints = %+v", sprints)
	}
}
