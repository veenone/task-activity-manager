package importer_test

import (
	"context"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
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
	res, err := importer.Run(context.Background(), repo, "p1", "PLAT", "Business Requirement", nil, recs, m, "kanban.xlsx", dryRun)
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

// A keyed row's Sprint cell is read by nothing on purpose: a sprint is a
// board write EditFields cannot carry, so the row edits its other fields
// and leaves the issue's sprint alone even when the cell names a real open
// sprint, rather than failing the row or moving it.
func TestRunIgnoresTheSprintCellOnAKeyedRowOnPurpose(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	open := []boardrepo.SprintChoice{{ID: 12, Name: "Sprint 12", BoardName: "PLAT Scrum", State: "active"}}
	recs := [][]string{
		{"Key", "Summary", "Priority", "Sprint"},
		{"PLAT-412", "Apply promo code, revised", "Low", "Sprint 12"},
	}
	m := importer.AutoMap(recs[0])
	res, err := importer.Run(ctx, repo, "p1", "PLAT", "", open, recs, m, "plan.csv", false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Errors) != 0 || len(res.Updated) != 1 || res.Updated[0] != "PLAT-412" {
		t.Fatalf("a real open sprint in the Sprint cell must not fail the row: %+v", res)
	}
	story := detail(t, repo, "PLAT-412")
	if story.Summary != "Apply promo code, revised" || story.Priority != "Low" {
		t.Errorf("the row's other fields still land: %+v", story)
	}
	if story.SprintID != "" || story.SprintName != "" {
		t.Errorf("the Sprint cell must not move the issue: %+v", story)
	}
	if res.SprintCellsIgnored != 1 {
		t.Errorf("SprintCellsIgnored = %d, want 1", res.SprintCellsIgnored)
	}
}

// A keyed row whose Sprint column is mapped but whose cell is blank counts
// nothing: an empty cell means "leave the sprint alone", which is not a
// request the import declined.
func TestRunCountsNothingForABlankSprintCellOnAKeyedRow(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	open := []boardrepo.SprintChoice{{ID: 12, Name: "Sprint 12", BoardName: "PLAT Scrum", State: "active"}}
	recs := [][]string{
		{"Key", "Summary", "Priority", "Sprint"},
		{"PLAT-412", "Apply promo code, revised", "Low", ""},
	}
	m := importer.AutoMap(recs[0])
	res, err := importer.Run(ctx, repo, "p1", "PLAT", "", open, recs, m, "plan.csv", false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Errors) != 0 || len(res.Updated) != 1 {
		t.Fatalf("Run: %+v", res)
	}
	if res.SprintCellsIgnored != 0 {
		t.Errorf("SprintCellsIgnored = %d, want 0 for a blank cell", res.SprintCellsIgnored)
	}
}

// A create row's Sprint cell counts nothing, since a create honours it: the
// counter is only for cells a keyed row's update could not act on.
func TestRunCountsNothingForACreateRowsSprintCell(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	open := []boardrepo.SprintChoice{{ID: 12, Name: "Sprint 12", BoardName: "PLAT Scrum", State: "active"}}
	recs := [][]string{
		{"Type", "Summary", "Sprint"},
		{"Task", "Rotate the payment gateway keys", "Sprint 12"},
	}
	m := importer.AutoMap(recs[0])
	res, err := importer.Run(ctx, repo, "p1", "PLAT", "", open, recs, m, "plan.csv", false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Errors) != 0 || len(res.Created) != 1 {
		t.Fatalf("Run: %+v", res)
	}
	if res.SprintCellsIgnored != 0 {
		t.Errorf("SprintCellsIgnored = %d, want 0 for a create row", res.SprintCellsIgnored)
	}
	created := detail(t, repo, res.Created[0])
	if created.SprintID != "12" || created.SprintName != "Sprint 12" {
		t.Errorf("the create row must still land in its sprint: %+v", created)
	}
}

// The count survives a dry run, since the preflight is where the user should
// learn about it, before Import rather than after.
func TestRunCountsIgnoredSprintCellsOnADryRun(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	open := []boardrepo.SprintChoice{{ID: 12, Name: "Sprint 12", BoardName: "PLAT Scrum", State: "active"}}
	recs := [][]string{
		{"Key", "Summary", "Priority", "Sprint"},
		{"PLAT-412", "Apply promo code, revised", "Low", "Sprint 12"},
	}
	m := importer.AutoMap(recs[0])
	res, err := importer.Run(ctx, repo, "p1", "PLAT", "", open, recs, m, "plan.csv", true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Errors) != 0 || res.SprintCellsIgnored != 1 {
		t.Fatalf("dry run: %+v", res)
	}
	if len(res.Updated) != 0 {
		t.Error("a dry run must write nothing")
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
