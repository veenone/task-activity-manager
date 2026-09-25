package issuerepo_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
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

// numbered lands two projects whose keys cross the nine-to-ten boundary, so
// a text sort of the key is visibly wrong, plus the two keys that are not a
// prefix and a number at all. Every rank is empty and every type is the
// same, so the default order and the tie-break under another column both
// come down to the key.
func numbered() []backend.Issue {
	keys := []string{"PLAT-2", "OPS-10", "PLAT-100", "OPS-2", "PLAT-10", "PLAT-9", "OPS-1", "PLAT-1", "PLAT-7A", "NOHYPHEN"}
	out := make([]backend.Issue, len(keys))
	for i, k := range keys {
		project, _, _ := strings.Cut(k, "-")
		out[i] = backend.Issue{Key: k, ID: strconv.Itoa(i + 1), Project: project, Type: "task", Summary: k}
	}
	return out
}

func seedNumbered(t *testing.T, r *issuerepo.Repository) {
	t.Helper()
	now := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)
	if err := r.UpsertPage(context.Background(), "p1", numbered(), now, true); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// A key is a project prefix and a number, and the number sorts as a number:
// PLAT-10 belongs after PLAT-9, not between PLAT-1 and PLAT-2. The two
// projects do not interleave, and descending is the same order reversed. A
// key with no number to read counts as zero, which puts NOHYPHEN under its
// own prefix and PLAT-7A behind every numbered PLAT key rather than
// anywhere the next row happens to fall.
func TestListIssuesSortsKeysByNumber(t *testing.T) {
	r := newRepo(t)
	seedNumbered(t, r)

	asc := []string{"NOHYPHEN", "OPS-1", "OPS-2", "OPS-10", "PLAT-1", "PLAT-2", "PLAT-9", "PLAT-10", "PLAT-100", "PLAT-7A"}
	desc := make([]string, len(asc))
	for i, k := range asc {
		desc[len(asc)-1-i] = k
	}

	wantKeys(t, listKeys(t, r, issuerepo.IssueQuery{Sort: "key"}), asc...)
	wantKeys(t, listKeys(t, r, issuerepo.IssueQuery{Sort: "key", Desc: true}), desc...)
	// Every type is "task", so sorting by type falls through to the key.
	wantKeys(t, listKeys(t, r, issuerepo.IssueQuery{Sort: "type"}), asc...)
	// No sort column is the rank order, and every rank here is empty.
	wantKeys(t, listKeys(t, r, issuerepo.IssueQuery{}), asc...)
}

// A draft key carries a second hyphen, so its number is what follows the
// last one. Drafts stay pinned to the top and run 1, 2 ... 10 among
// themselves.
func TestListIssuesSortsDraftKeysByNumber(t *testing.T) {
	r := newRepo(t)
	seedNumbered(t, r)

	drafts := make([]backend.IssueDraft, 10)
	for i := range drafts {
		drafts[i] = backend.IssueDraft{Type: "task", Summary: fmt.Sprintf("draft %d", i+1)}
	}
	keys, err := r.CreateDrafts(context.Background(), "p1", "PLAT", drafts, "")
	if err != nil {
		t.Fatalf("drafts: %v", err)
	}
	got := listKeys(t, r, issuerepo.IssueQuery{Sort: "key"})
	if len(got) <= len(keys) {
		t.Fatalf("got %v, want the %d drafts and the seeded issues", got, len(keys))
	}
	wantKeys(t, got[:len(keys)], keys...)
}

// SortColumns is what the frontend's header list agrees with, so it has to
// name every column the grid offers and nothing else.
func TestSortColumns(t *testing.T) {
	want := []string{"assignee", "key", "sprint", "status", "storyPoints", "summary", "type"}
	wantKeys(t, issuerepo.SortColumns(), want...)
}
