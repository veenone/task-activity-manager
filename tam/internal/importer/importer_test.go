package importer_test

import (
	"context"
	"encoding/csv"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agile-suite/core/importfile"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/importer"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/tamstore"
)

func newRepo(t *testing.T) *issuerepo.Repository {
	t.Helper()
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := issuerepo.New(db.DB())
	rows := []backend.Issue{
		{Key: "PLAT-350", Type: backend.TypeEpic, Summary: "Promotions", Labels: []string{}, Updated: "2026-09-01T00:00:00Z"},
		{Key: "PLAT-412", Type: backend.TypeStory, Summary: "Apply promo code", Labels: []string{}, Updated: "2026-09-01T00:00:00Z"},
	}
	if err := repo.UpsertPage(context.Background(), "p1", rows, time.Now(), false); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestAutoMapMatchesHeadersLooselyAndBySynonym(t *testing.T) {
	m := importer.AutoMap([]string{"Issue Type", "Summary", "Description", "priority", "Labels", "Assignee", "Story_Points", "Epic Link", "Comment"})
	want := importer.Mapping{Type: "Issue Type", Summary: "Summary", Description: "Description", Priority: "priority", Labels: "Labels", Assignee: "Assignee", StoryPoints: "Story_Points", ParentKey: "Epic Link"}
	if m != want {
		t.Errorf("AutoMap = %+v, want %+v", m, want)
	}
	m = importer.AutoMap([]string{"Title", "Points", "Parent"})
	if m.Summary != "Title" || m.StoryPoints != "Points" || m.ParentKey != "Parent" || m.Type != "" {
		t.Errorf("synonyms: %+v", m)
	}
}

func records() [][]string {
	return [][]string{
		{"Issue Type", "Summary", "Description", "Priority", "Labels", "Assignee", "Points", "Epic Link"},
		{"Story", "Apply promo at payment", "As a shopper", "High", "checkout, promo", "ranand", "5", "PLAT-350"},
		{"", "Rotate keys", "", "", "security", "", "", ""},
		{"Bug", "", "no summary", "", "", "", "", ""},
		{"Epic", "Promo overhaul", "", "", "", "", "", ""},
		{"Task", "Bad points", "", "", "", "", "eight", ""},
		{"Task", "Unknown parent", "", "", "", "", "", "PLAT-999"},
		{"Task", "Story as parent", "", "", "", "", "", "PLAT-412"},
		{"Epic", "Epic with a parent", "", "", "", "", "", "PLAT-350"},
		{"Business Requirement", "Single-use promo codes", "", "", "promo", "", "", ""},
		{"", "", "", "", "", "", "", ""},
	}
}

func TestRunDryRunValidatesEveryRuleAndCreatesNothing(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	m := importer.AutoMap(records()[0])
	res, err := importer.Run(ctx, repo, "p1", "PLAT", "Business Requirement", records(), m, "backlog.csv", true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// The trailing blank row is not counted in Rows and produces no error.
	if res.Rows != 9 || len(res.Created) != 0 || len(res.Errors) != 5 {
		t.Fatalf("result: %+v", res)
	}
	got := map[int]string{}
	for _, e := range res.Errors {
		got[e.Row] = e.Message
	}
	for row, want := range map[int]string{
		4: "Summary is empty",
		6: `Story points "eight" is not a number`,
		7: "Parent PLAT-999 is not in the cache",
		8: "Parent PLAT-412 is not an epic",
		9: "An epic cannot have a parent",
	} {
		if !strings.Contains(got[row], want) {
			t.Errorf("row %d: %q lacks %q", row, got[row], want)
		}
	}
	if page, _ := repo.ListIssues(ctx, "p1", issuerepo.IssueQuery{}); page.Total != 2 {
		t.Errorf("dry run created rows: %d", page.Total)
	}
}

func TestRunImportsTheValidRowsAsDrafts(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	m := importer.AutoMap(records()[0])
	res, err := importer.Run(ctx, repo, "p1", "PLAT", "Business Requirement", records(), m, "backlog.csv", false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Join(res.Created, ",") != "TAM-NEW-1,TAM-NEW-2,TAM-NEW-3,TAM-NEW-4" || len(res.Errors) != 5 {
		t.Fatalf("result: %+v", res)
	}
	first, _ := repo.GetIssue(ctx, "p1", "TAM-NEW-1")
	if first.Type != backend.TypeStory || first.Summary != "Apply promo at payment" || first.Priority != "High" || strings.Join(first.Labels, "|") != "checkout|promo" || first.Assignee != "ranand" || *first.StoryPoints != 5 || first.ParentKey != "PLAT-350" {
		t.Errorf("first: %+v", first)
	}
	second, _ := repo.GetIssue(ctx, "p1", "TAM-NEW-2")
	if second.Type != backend.TypeTask || second.StoryPoints != nil {
		t.Errorf("blank type means task, blank points mean none: %+v", second)
	}
	third, _ := repo.GetIssue(ctx, "p1", "TAM-NEW-3")
	if third.Type != backend.TypeEpic || third.Summary != "Promo overhaul" {
		t.Errorf("an epic row imports as a draft of type epic: %+v", third)
	}
	fourth, _ := repo.GetIssue(ctx, "p1", "TAM-NEW-4")
	if fourth.Type != backend.TypeRequirement {
		t.Errorf("the profile's requirement type name maps to requirement: %+v", fourth)
	}
	d, _, _, _ := repo.ReadDetail(ctx, "p1", "TAM-NEW-1")
	if d.Description != "As a shopper" {
		t.Errorf("description: %+v", d)
	}
	act, _ := repo.ListActivity(ctx, "p1", "TAM-NEW-1", 0)
	if len(act) != 1 || act[0].Note != "imported from backlog.csv" {
		t.Errorf("audit note: %+v", act)
	}
}

func TestRunOnlyResolvesAnInFileEpicWhenItComesFirst(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	header := []string{"Type", "Summary", "Parent"}
	m := importer.AutoMap(header)

	childFirst := [][]string{
		header,
		{"Task", "Child of a new epic", "TAM-NEW-1"},
		{"Epic", "New team epic", ""},
	}
	res, err := importer.Run(ctx, repo, "p1", "PLAT", "", childFirst, m, "f.csv", false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Message, "TAM-NEW-1 is not in the cache") {
		t.Fatalf("a child before its new epic fails with the usual message: %+v", res.Errors)
	}
	if strings.Join(res.Created, ",") != "TAM-NEW-1" {
		t.Fatalf("the epic row still imports, under the key the child guessed too late: %+v", res)
	}

	epicFirst := [][]string{
		header,
		{"Epic", "Another new epic", ""},
		{"Task", "Second child", "TAM-NEW-2"},
	}
	res2, err := importer.Run(ctx, repo, "p1", "PLAT", "", epicFirst, m, "f.csv", false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Join(res2.Created, ",") != "TAM-NEW-2,TAM-NEW-3" || len(res2.Errors) != 0 {
		t.Fatalf("epic first creates both: %+v", res2)
	}
	child, _ := repo.GetIssue(ctx, "p1", "TAM-NEW-3")
	if child.ParentKey != "TAM-NEW-2" {
		t.Errorf("the child's parent is the epic's predicted key: %+v", child)
	}
}

func TestRunRefusesADraftAsAParent(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	draft, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeStory, Summary: "Not committed yet"})
	if err != nil {
		t.Fatal(err)
	}
	m := importer.AutoMap(records()[0])
	rows := [][]string{
		records()[0],
		{"Task", "Child of a draft", "", "", "", "", "", draft},
	}
	res, err := importer.Run(ctx, repo, "p1", "PLAT", "Business Requirement", rows, m, "backlog.csv", true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Message, draft+" is a draft; commit it first.") {
		t.Errorf("errors: %+v", res.Errors)
	}
}

func TestRunRefusesAMappingWithoutSummaryOrWithAMissingColumn(t *testing.T) {
	repo := newRepo(t)
	if _, err := importer.Run(context.Background(), repo, "p1", "PLAT", "", records(), importer.Mapping{Type: "Issue Type"}, "f.csv", true); err == nil || !strings.Contains(err.Error(), "Summary") {
		t.Errorf("no summary mapping: %v", err)
	}
	if _, err := importer.Run(context.Background(), repo, "p1", "PLAT", "", records(), importer.Mapping{Summary: "Nope"}, "f.csv", true); err == nil || !strings.Contains(err.Error(), `"Nope"`) {
		t.Errorf("missing column: %v", err)
	}
	if _, err := importer.Run(context.Background(), repo, "p1", "PLAT", "", [][]string{{"Summary"}}, importer.Mapping{Summary: "Summary"}, "f.csv", true); err == nil {
		t.Error("a file with only a header has nothing to import")
	}
}

func TestTemplateCSVRoundTripsThroughAutoMap(t *testing.T) {
	data := importer.TemplateCSV("Business Requirement")
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 6 || !strings.HasPrefix(lines[0], "Key,Type,Summary,Description,Priority,Labels,Assignee,Story Points,Parent") {
		t.Errorf("template: %q", string(data))
	}
	records, err := csv.NewReader(strings.NewReader(string(data))).ReadAll()
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	m := importer.AutoMap(records[0])
	if m.Key == "" || m.Type == "" || m.Summary == "" || m.StoryPoints == "" || m.ParentKey == "" {
		t.Errorf("template headers must auto-map: %+v", m)
	}
	// Key and Parent are checked against the profile's cache, so a filled-in
	// example would fail for everyone. Both stay empty in the shipped rows.
	for i, row := range records[1:] {
		if row[0] != "" || row[8] != "" {
			t.Errorf("row %d key and parent cells must be empty: %q, %q", i+2, row[0], row[8])
		}
	}
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := issuerepo.New(db.DB())
	res, err := importer.Run(context.Background(), repo, "p1", "PLAT", "Business Requirement", records, m, "template.csv", true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Rows != 5 || len(res.Errors) != 0 {
		t.Errorf("template dry run against a fresh repo: %+v", res)
	}
}

// The workbook has to be a workbook the importer itself can read back, or
// the round trip the template exists for does not close.
func TestTemplateXLSXParsesBackIntoTheSameRows(t *testing.T) {
	data, err := importer.TemplateXLSX("Business Requirement")
	if err != nil {
		t.Fatalf("TemplateXLSX: %v", err)
	}
	records, err := importfile.ParseRecords(data, true)
	if err != nil {
		t.Fatalf("parse workbook: %v", err)
	}
	if len(records) != 6 {
		t.Fatalf("want a header and 5 examples, got %d rows", len(records))
	}
	for i, h := range importer.TemplateHeaders {
		if records[0][i] != h {
			t.Errorf("column %d: got %q, want %q", i, records[0][i], h)
		}
	}
	m := importer.AutoMap(records[0])
	if m.Key == "" || m.Summary == "" || m.Assignee == "" {
		t.Errorf("workbook headers must auto-map: %+v", m)
	}
	// The requirement example uses the profile's own type name, so a project
	// that calls it something else gets a row it can actually import.
	if got := records[5][1]; got != "Business Requirement" {
		t.Errorf("requirement example type: %q", got)
	}
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	res, err := importer.Run(context.Background(), issuerepo.New(db.DB()), "p1", "PLAT", "Business Requirement", records, m, "template.xlsx", true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Rows != 5 || len(res.Errors) != 0 {
		t.Errorf("workbook dry run against a fresh repo: %+v", res)
	}
}

func TestRunSkipsRowsAlreadyDraftedOrRepeatedInTheFile(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	if _, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeStory, Summary: "Apply promo at payment"}); err != nil {
		t.Fatal(err)
	}
	rows := [][]string{
		records()[0],
		{"Story", "Apply promo at payment", "", "", "", "", "", ""}, // matches the existing draft
		{"Task", "Brand new row", "", "", "", "", "", ""},
		{"Task", "Brand new row", "", "", "", "", "", ""}, // repeats row 3
	}
	m := importer.AutoMap(rows[0])
	res, err := importer.Run(ctx, repo, "p1", "PLAT", "", rows, m, "f.csv", true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Rows != 3 || len(res.Errors) != 2 {
		t.Fatalf("result: %+v", res)
	}
	got := map[int]string{}
	for _, e := range res.Errors {
		got[e.Row] = e.Message
	}
	if got[2] != "Already a draft (TAM-NEW-1); commit or discard it first." {
		t.Errorf("row 2: %q", got[2])
	}
	if got[4] != "Duplicate of row 3." {
		t.Errorf("row 4: %q", got[4])
	}
}
