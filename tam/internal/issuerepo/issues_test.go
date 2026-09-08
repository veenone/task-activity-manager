package issuerepo_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/tamstore"
)

func newRepo(t *testing.T) *issuerepo.Repository {
	t.Helper()
	r, _ := newRepoWithDB(t)
	return r
}

// newRepoWithDB is newRepo with the handle beside it, for the reads that
// take the querier they run on: the board composes itself inside one read
// transaction and hands that in, and a test reading on its own hands in
// the handle.
func newRepoWithDB(t *testing.T) (*issuerepo.Repository, *sql.DB) {
	t.Helper()
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return issuerepo.New(db.DB()), db.DB()
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

func TestUpsertKeepsTheStatusID(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 5, 10, 42, 0, 0, time.UTC)
	page := sample()
	page[0].StatusID = "3"
	page[1].StatusID = "1"
	if err := r.UpsertPage(ctx, "p1", page, now, false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := r.GetIssue(ctx, "p1", "PLAT-412")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.StatusID != "3" || got.Status != "In Progress" {
		t.Errorf("row = %q / %q, want In Progress / 3", got.Status, got.StatusID)
	}
	// A second sync of the same issue in a new status carries the new id.
	page[0].Status, page[0].StatusID = "Done", "5"
	if err := r.UpsertPage(ctx, "p1", page[:1], now.Add(time.Minute), false); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if got, _ := r.GetIssue(ctx, "p1", "PLAT-412"); got.StatusID != "5" {
		t.Errorf("status id after the second sync = %q, want 5", got.StatusID)
	}
	// An issue synced without one reads back empty rather than failing.
	if got, _ := r.GetIssue(ctx, "p1", "PLAT-350"); got.StatusID != "" {
		t.Errorf("status id = %q, want empty", got.StatusID)
	}
}

func TestIssuesByKeysReturnsTheCallersOrderAndSkipsWhatIsNotCached(t *testing.T) {
	r, db := newRepoWithDB(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 5, 10, 42, 0, 0, time.UTC)
	if err := r.UpsertPage(ctx, "p1", sample(), now, false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// A board's order is neither the key order nor the rank order, and the
	// missing key is one a board filter reached outside the project.
	keys := []string{"PLAT-347", "OPS-9", "PLAT-412", "PLAT-350"}
	got, err := r.IssuesByKeys(ctx, db, "p1", keys)
	if err != nil {
		t.Fatalf("by keys: %v", err)
	}
	want := []string{"PLAT-347", "PLAT-412", "PLAT-350"}
	if len(got) != len(want) {
		t.Fatalf("got %d issues, want %d", len(got), len(want))
	}
	for i, k := range want {
		if got[i].Key != k {
			t.Errorf("row %d = %s, want %s", i, got[i].Key, k)
		}
	}
	if got[0].Summary == "" || got[0].Labels == nil {
		t.Errorf("rows come back whole: %+v", got[0])
	}
	// Another profile's cache is not readable through the same keys.
	other, err := r.IssuesByKeys(ctx, db, "p2", keys)
	if err != nil || len(other) != 0 {
		t.Errorf("other profile = %+v, %v; want empty", other, err)
	}
}

func TestIssuesByKeysWithNoKeys(t *testing.T) {
	r, db := newRepoWithDB(t)
	got, err := r.IssuesByKeys(context.Background(), db, "p1", nil)
	if err != nil {
		t.Fatalf("nil keys: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("nil keys = %+v, want an empty slice", got)
	}
	if got, err = r.IssuesByKeys(context.Background(), db, "p1", []string{}); err != nil || len(got) != 0 {
		t.Errorf("empty keys = %+v, %v", got, err)
	}
}

func TestDraftIssuesReadsTheProfilesDraftsInKeyOrder(t *testing.T) {
	r, db := newRepoWithDB(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 5, 10, 42, 0, 0, time.UTC)
	if err := r.UpsertPage(ctx, "p1", sample(), now, false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	first, err := r.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "Draft one"})
	if err != nil {
		t.Fatalf("first draft: %v", err)
	}
	second, err := r.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeStory, Summary: "Draft two"})
	if err != nil {
		t.Fatalf("second draft: %v", err)
	}
	if _, err := r.CreateDraft(ctx, "p2", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "Another profile"}); err != nil {
		t.Fatalf("other profile draft: %v", err)
	}

	got, err := r.DraftIssues(ctx, db, "p1")
	if err != nil {
		t.Fatalf("drafts: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("drafts = %+v, want the two of this profile and no synced row", got)
	}
	if got[0].Key != first || got[1].Key != second {
		t.Errorf("drafts = %s, %s; want %s, %s in key order", got[0].Key, got[1].Key, first, second)
	}
	for _, iss := range got {
		if !iss.Draft {
			t.Errorf("%s came back without the Draft flag", iss.Key)
		}
		if iss.StatusID != "" {
			t.Errorf("%s has status id %q; a draft Jira has never seen has none", iss.Key, iss.StatusID)
		}
		if iss.Labels == nil {
			t.Errorf("%s came back with nil labels; the rows are whole", iss.Key)
		}
	}
	if got[0].Summary != "Draft one" {
		t.Errorf("first draft = %+v", got[0])
	}
}

func TestDraftIssuesWithNoDrafts(t *testing.T) {
	r, db := newRepoWithDB(t)
	got, err := r.DraftIssues(context.Background(), db, "p1")
	if err != nil {
		t.Fatalf("drafts: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("drafts = %+v, want an empty slice", got)
	}
}

func TestIssuesByKeysReadsPastTheChunkBoundary(t *testing.T) {
	r, db := newRepoWithDB(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 5, 10, 42, 0, 0, time.UTC)
	// One more than a chunk, so the read spans two statements.
	const n = 501
	page := make([]backend.Issue, 0, n)
	keys := make([]string, 0, n)
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("PLAT-%d", 1000+i)
		page = append(page, backend.Issue{Key: key, ID: key, Project: "PLAT", Type: "task", Summary: key, Status: "To Do", StatusID: "1"})
		keys = append(keys, key)
	}
	if err := r.UpsertPage(ctx, "p1", page, now, false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// Reversed, so a chunk that quietly reordered its rows would show.
	for i, j := 0, len(keys)-1; i < j; i, j = i+1, j-1 {
		keys[i], keys[j] = keys[j], keys[i]
	}
	got, err := r.IssuesByKeys(ctx, db, "p1", keys)
	if err != nil {
		t.Fatalf("by keys: %v", err)
	}
	if len(got) != n {
		t.Fatalf("got %d issues, want %d", len(got), n)
	}
	for i, k := range keys {
		if got[i].Key != k {
			t.Fatalf("row %d = %s, want %s", i, got[i].Key, k)
		}
	}
}
