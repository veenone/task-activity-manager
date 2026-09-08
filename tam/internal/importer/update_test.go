package importer_test

import (
	"context"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/importer"
	"agile-suite/tam/internal/issuerepo"
)

// updateRecords is a file in the shape the reference kanban sheet is in: a
// key column that holds either a bare key or the browse URL, alongside the
// ordinary field columns.
func updateRecords() [][]string {
	return [][]string{
		{"JIRA Task", "Type", "Summary", "Description", "Priority", "Labels", "Assignee", "Points", "Epic Link"},
		{"PLAT-412", "Story", "Apply promo code at payment step", "As a shopper", "High", "checkout, promo", "ranand", "5", "PLAT-350"},
		{"https://jira.example.com/browse/PLAT-350", "Epic", "Promotions and discounts", "", "", "", "", "", ""},
	}
}

func run(t *testing.T, repo *issuerepo.Repository, recs [][]string, dryRun bool) importer.Result {
	t.Helper()
	m := importer.AutoMap(recs[0])
	res, err := importer.Run(context.Background(), repo, "p1", "PLAT", "Business Requirement", recs, m, "kanban.xlsx", dryRun)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return res
}

func detail(t *testing.T, repo *issuerepo.Repository, key string) backend.Issue {
	t.Helper()
	iss, err := repo.GetIssue(context.Background(), "p1", key)
	if err != nil {
		t.Fatalf("GetIssue %s: %v", key, err)
	}
	return iss
}

// A keyed row edits the issue it names. Nothing is created, and the browse
// URL form resolves to the same issue as the bare key.
func TestRunUpdatesKeyedRowsInsteadOfCreating(t *testing.T) {
	repo := newRepo(t)
	res := run(t, repo, updateRecords(), false)

	if len(res.Created) != 0 {
		t.Errorf("keyed rows must not create: %v", res.Created)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("errors: %+v", res.Errors)
	}
	if len(res.Updated) != 2 || res.Updated[0] != "PLAT-412" || res.Updated[1] != "PLAT-350" {
		t.Fatalf("Updated = %v, want [PLAT-412 PLAT-350]", res.Updated)
	}

	story := detail(t, repo, "PLAT-412")
	if story.Summary != "Apply promo code at payment step" || story.Priority != "High" || story.Assignee != "ranand" {
		t.Errorf("story fields: %+v", story)
	}
	if story.StoryPoints == nil || *story.StoryPoints != 5 {
		t.Errorf("story points: %+v", story.StoryPoints)
	}
	if story.ParentKey != "PLAT-350" {
		t.Errorf("parent: %q", story.ParentKey)
	}
	if strings.Join(story.Labels, ", ") != "checkout, promo" {
		t.Errorf("labels: %v", story.Labels)
	}
	if detail(t, repo, "PLAT-350").Summary != "Promotions and discounts" {
		t.Errorf("epic summary was not updated")
	}
}

// A second pass over the same file records nothing: every value already
// matches, so there is no pending change to commit.
func TestRunSkipsKeyedRowsThatChangeNothing(t *testing.T) {
	repo := newRepo(t)
	run(t, repo, updateRecords(), false)
	res := run(t, repo, updateRecords(), false)
	if len(res.Updated) != 0 {
		t.Errorf("re-import recorded %v", res.Updated)
	}
	if res.Rows != 2 || len(res.Errors) != 0 {
		t.Errorf("re-import: %+v", res)
	}
}

// An empty cell on a keyed row leaves the field alone rather than clearing
// it, so a sheet carrying only the columns someone filled in is safe.
func TestRunLeavesUnfilledCellsAloneOnKeyedRows(t *testing.T) {
	repo := newRepo(t)
	run(t, repo, updateRecords(), false)
	res := run(t, repo, [][]string{
		{"Key", "Summary", "Priority", "Labels", "Assignee", "Points"},
		{"PLAT-412", "", "Low", "", "", ""},
	}, false)
	if len(res.Updated) != 1 {
		t.Fatalf("Updated = %v", res.Updated)
	}
	story := detail(t, repo, "PLAT-412")
	if story.Priority != "Low" {
		t.Errorf("priority: %q", story.Priority)
	}
	if story.Summary != "Apply promo code at payment step" || story.Assignee != "ranand" {
		t.Errorf("blank cells cleared a field: %+v", story)
	}
	if story.StoryPoints == nil || *story.StoryPoints != 5 {
		t.Errorf("blank cell cleared the estimate: %+v", story.StoryPoints)
	}
}

// A dry run of keyed rows validates and writes nothing, the same as a dry
// run of creates.
func TestRunDryRunWritesNoUpdates(t *testing.T) {
	repo := newRepo(t)
	res := run(t, repo, updateRecords(), true)
	if len(res.Updated) != 0 || res.Rows != 2 || len(res.Errors) != 0 {
		t.Errorf("dry run: %+v", res)
	}
	if detail(t, repo, "PLAT-412").Summary != "Apply promo code" {
		t.Error("dry run changed the cache")
	}
}

// Every way a keyed row can be wrong is reported against its own file row,
// and the rest of the file still lands.
func TestRunReportsBadKeyedRows(t *testing.T) {
	repo := newRepo(t)
	key, err := repo.CreateDraft(context.Background(), "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "A draft"})
	if err != nil {
		t.Fatal(err)
	}
	res := run(t, repo, [][]string{
		{"Key", "Summary", "Points", "Epic Link"},
		{"not a key", "x", "", ""},
		{"PLAT-999", "x", "", ""},
		{key, "x", "", ""},
		{"PLAT-412", "x", "eight", ""},
		{"PLAT-412", "x", "", "PLAT-412"},
		{"PLAT-350", "x", "", "PLAT-350"},
		{"PLAT-350", "A good row", "", ""},
		{"PLAT-350", "Again", "", ""},
	}, false)

	// Rows 5 and 6 name the same issue and both fail for their own reason,
	// so neither reads as a duplicate of the other: only row 9, which
	// repeats the key row 8 actually used, does.
	want := []struct {
		row  int
		text string
	}{
		{2, "is not an issue key"},
		{3, "is not in the cache"},
		{4, "is a draft"},
		{5, "is not a number"},
		{6, "is not an epic"},
		{7, "An epic cannot have a parent"},
		{9, "Duplicate of row 8"},
	}
	if len(res.Errors) != len(want) {
		t.Fatalf("errors: %+v", res.Errors)
	}
	for i, w := range want {
		got := res.Errors[i]
		if got.Row != w.row || !strings.Contains(got.Message, w.text) {
			t.Errorf("error %d: got %+v, want row %d containing %q", i, got, w.row, w.text)
		}
	}
	if len(res.Updated) != 1 || res.Updated[0] != "PLAT-350" {
		t.Errorf("the good row must still land: %v", res.Updated)
	}
	if detail(t, repo, "PLAT-412").Summary != "Apply promo code" {
		t.Error("a rejected row edited the issue anyway")
	}
}

// One file can create and update at once, which is what a planning sheet
// half-filled from Jira actually looks like.
func TestRunMixesCreatesAndUpdatesInOneFile(t *testing.T) {
	repo := newRepo(t)
	res := run(t, repo, [][]string{
		{"Key", "Type", "Summary", "Epic Link"},
		{"PLAT-412", "Story", "Apply promo code at payment step", ""},
		{"", "Task", "Rotate the payment gateway keys", "PLAT-350"},
	}, false)
	if len(res.Errors) != 0 {
		t.Fatalf("errors: %+v", res.Errors)
	}
	if len(res.Created) != 1 || len(res.Updated) != 1 {
		t.Fatalf("want one create and one update, got %+v", res)
	}
	if detail(t, repo, res.Created[0]).ParentKey != "PLAT-350" {
		t.Error("the created draft lost its parent")
	}
}
