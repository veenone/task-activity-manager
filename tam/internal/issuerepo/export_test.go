package issuerepo_test

import (
	"context"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// seedForExport puts more rows in the store than a page holds, so a test
// can tell "every row the filter matched" from "the page on screen".
func seedForExport(t *testing.T) *issuerepo.Repository {
	t.Helper()
	r := newRepo(t)
	rows := make([]backend.Issue, 0, 40)
	for i := 1; i <= 40; i++ {
		kind := backend.TypeTask
		if i%2 == 0 {
			kind = backend.TypeBug
		}
		rows = append(rows, backend.Issue{
			Key: key(i), ID: id(i), Project: "PLAT", Type: kind,
			Summary: "Row " + id(i), Status: "To Do", Updated: "2026-09-01T00:00:00Z",
		})
	}
	if err := r.UpsertPage(context.Background(), "p1", rows, time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return r
}

func key(i int) string { return "PLAT-" + id(i) }
func id(i int) string {
	if i < 10 {
		return "0" + string(rune('0'+i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}

// An export is of the filter, not of the page: the grid shows 25 rows and
// the file has to hold every row the filter matched.
func TestExportReadsEveryMatchingRowRatherThanThePage(t *testing.T) {
	r := seedForExport(t)
	rows, err := r.ListIssuesForExport(context.Background(), "p1", issuerepo.IssueQuery{Limit: 25})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(rows) != 40 {
		t.Errorf("rows = %d, want all 40 whatever the page size says", len(rows))
	}
}

// The filter bar's state reaches the file: what is on screen is what is
// exported, narrowed the same way.
func TestExportHonoursTheFilterAndTheSort(t *testing.T) {
	r := seedForExport(t)
	ctx := context.Background()
	bugs, err := r.ListIssuesForExport(ctx, "p1", issuerepo.IssueQuery{Types: []string{backend.TypeBug}})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(bugs) != 20 {
		t.Fatalf("bugs = %d, want the twenty the filter matches", len(bugs))
	}
	for _, iss := range bugs {
		if iss.Type != backend.TypeBug {
			t.Fatalf("exported a %s row under a bug filter", iss.Type)
		}
	}
	desc, err := r.ListIssuesForExport(ctx, "p1", issuerepo.IssueQuery{Sort: "key", Desc: true})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if desc[0].Key != "PLAT-40" {
		t.Errorf("first row under a descending key sort = %s", desc[0].Key)
	}
}

// A filter matching nothing exports nothing, which the caller turns into a
// file with its header row. It is not an error: a backlog with no bugs in
// it is an answer.
func TestExportOfAFilterThatMatchesNothingIsEmptyRatherThanAnError(t *testing.T) {
	r := seedForExport(t)
	rows, err := r.ListIssuesForExport(context.Background(), "p1", issuerepo.IssueQuery{Text: "nothing matches this"})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("rows = %d, want none", len(rows))
	}
}
