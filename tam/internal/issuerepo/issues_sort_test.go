package issuerepo_test

import (
	"context"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// sortable lands three issues whose columns deliberately disagree with rank
// order, so a test can tell a real sort from the default one.
func sortable() []backend.Issue {
	return []backend.Issue{
		{Key: "PLAT-3", ID: "1", Project: "PLAT", Type: "task", Summary: "Alpha", Status: "Done", Assignee: "Zoe", SprintName: "Sprint 2", StoryPoints: pts(8), Rank: "0|a:"},
		{Key: "PLAT-1", ID: "2", Project: "PLAT", Type: "bug", Summary: "Charlie", Status: "To Do", Assignee: "adam", SprintName: "Sprint 1", StoryPoints: pts(2), Rank: "0|b:"},
		{Key: "PLAT-2", ID: "3", Project: "PLAT", Type: "story", Summary: "Bravo", Status: "In Progress", Rank: "0|c:"},
	}
}

func seedSortable(t *testing.T, r *issuerepo.Repository) {
	t.Helper()
	now := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)
	if err := r.UpsertPage(context.Background(), "p1", sortable(), now, true); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func listKeys(t *testing.T, r *issuerepo.Repository, q issuerepo.IssueQuery) []string {
	t.Helper()
	page, err := r.ListIssues(context.Background(), "p1", q)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	keys := make([]string, len(page.Issues))
	for i, iss := range page.Issues {
		keys[i] = iss.Key
	}
	return keys
}

func wantKeys(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestListIssuesSortsByColumn(t *testing.T) {
	r := newRepo(t)
	seedSortable(t, r)

	// No sort column is the rank order, which is deliberately neither key
	// nor summary order, so this also proves the cases below really sort.
	wantKeys(t, listKeys(t, r, issuerepo.IssueQuery{}), "PLAT-3", "PLAT-1", "PLAT-2")

	wantKeys(t, listKeys(t, r, issuerepo.IssueQuery{Sort: "key"}), "PLAT-1", "PLAT-2", "PLAT-3")
	wantKeys(t, listKeys(t, r, issuerepo.IssueQuery{Sort: "key", Desc: true}), "PLAT-3", "PLAT-2", "PLAT-1")
	wantKeys(t, listKeys(t, r, issuerepo.IssueQuery{Sort: "summary"}), "PLAT-3", "PLAT-2", "PLAT-1")
	wantKeys(t, listKeys(t, r, issuerepo.IssueQuery{Sort: "storyPoints"}), "PLAT-1", "PLAT-3", "PLAT-2")
}

// An unassigned or unestimated row must not lead the page just because its
// column is empty, in either direction.
func TestListIssuesSortsBlanksLast(t *testing.T) {
	r := newRepo(t)
	seedSortable(t, r)

	wantKeys(t, listKeys(t, r, issuerepo.IssueQuery{Sort: "assignee"}), "PLAT-1", "PLAT-3", "PLAT-2")
	wantKeys(t, listKeys(t, r, issuerepo.IssueQuery{Sort: "assignee", Desc: true}), "PLAT-3", "PLAT-1", "PLAT-2")
	wantKeys(t, listKeys(t, r, issuerepo.IssueQuery{Sort: "storyPoints", Desc: true}), "PLAT-3", "PLAT-1", "PLAT-2")
	wantKeys(t, listKeys(t, r, issuerepo.IssueQuery{Sort: "sprint", Desc: true}), "PLAT-3", "PLAT-1", "PLAT-2")
}

// Assignee "adam" sorts before "Zoe": a display name is prose, so the
// comparison is case-insensitive rather than by byte.
func TestListIssuesSortsTextCaseInsensitively(t *testing.T) {
	r := newRepo(t)
	seedSortable(t, r)
	wantKeys(t, listKeys(t, r, issuerepo.IssueQuery{Sort: "assignee"}), "PLAT-1", "PLAT-3", "PLAT-2")
}

// A sort key the store does not know falls back to rank order. The column
// name selects a whitelist entry and never reaches the query, so neither a
// plausible-but-absent name nor an injection attempt changes the SQL.
func TestListIssuesIgnoresUnknownSortColumn(t *testing.T) {
	r := newRepo(t)
	seedSortable(t, r)
	wantKeys(t, listKeys(t, r, issuerepo.IssueQuery{Sort: "rank"}), "PLAT-3", "PLAT-1", "PLAT-2")
	wantKeys(t, listKeys(t, r, issuerepo.IssueQuery{Sort: "key; DROP TABLE issue"}), "PLAT-3", "PLAT-1", "PLAT-2")
	// The table is still there, and still holds every row.
	wantKeys(t, listKeys(t, r, issuerepo.IssueQuery{Sort: "key"}), "PLAT-1", "PLAT-2", "PLAT-3")
}

// Drafts stay at the top under every sort: they are uncommitted local work,
// and a sort that buried them would hide it.
func TestListIssuesKeepsDraftsFirstUnderSort(t *testing.T) {
	r := newRepo(t)
	seedSortable(t, r)
	key, err := r.CreateDraft(context.Background(), "p1", "PLAT", backend.IssueDraft{
		Type: "task", Summary: "Zulu, last alphabetically",
	})
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if got := listKeys(t, r, issuerepo.IssueQuery{Sort: "summary"}); got[0] != key {
		t.Fatalf("draft should lead an ascending summary sort, got %v", got)
	}
	if got := listKeys(t, r, issuerepo.IssueQuery{Sort: "key", Desc: true}); got[0] != key {
		t.Fatalf("draft should lead a descending key sort, got %v", got)
	}
}

// SortColumns is what the frontend's header list agrees with, so it has to
// name every column the grid offers and nothing else.
func TestSortColumns(t *testing.T) {
	want := []string{"assignee", "key", "sprint", "status", "storyPoints", "summary", "type"}
	wantKeys(t, issuerepo.SortColumns(), want...)
}
