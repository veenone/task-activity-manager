package importer_test

import (
	"context"
	"encoding/csv"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	rows := []backend.Issue{{Key: "PLAT-350", Type: backend.TypeEpic, Summary: "Promotions", Labels: []string{}, Updated: "2026-09-01T00:00:00Z"}}
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
		{"Epic", "Not creatable", "", "", "", "", "", ""},
		{"Task", "Bad points", "", "", "", "", "eight", ""},
		{"Task", "Unknown parent", "", "", "", "", "", "PLAT-999"},
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
	if res.Rows != 7 || len(res.Created) != 0 || len(res.Errors) != 4 {
		t.Fatalf("result: %+v", res)
	}
	got := map[int]string{}
	for _, e := range res.Errors {
		got[e.Row] = e.Message
	}
	for row, want := range map[int]string{4: "Summary is empty", 5: `Type "Epic" cannot be created`, 6: `Story points "eight" is not a number`, 7: "Parent PLAT-999 is not in the cache"} {
		if !strings.Contains(got[row], want) {
			t.Errorf("row %d: %q lacks %q", row, got[row], want)
		}
	}
	if page, _ := repo.ListIssues(ctx, "p1", issuerepo.IssueQuery{}); page.Total != 1 {
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
	if strings.Join(res.Created, ",") != "TAM-NEW-1,TAM-NEW-2,TAM-NEW-3" || len(res.Errors) != 4 {
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
	if third.Type != backend.TypeRequirement {
		t.Errorf("the profile's requirement type name maps to requirement: %+v", third)
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
	data := importer.TemplateCSV()
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 5 || !strings.HasPrefix(lines[0], "Type,Summary,Description,Priority,Labels,Assignee,Story Points,Parent") {
		t.Errorf("template: %q", string(data))
	}
	records, err := csv.NewReader(strings.NewReader(string(data))).ReadAll()
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	m := importer.AutoMap(records[0])
	if m.Type == "" || m.Summary == "" || m.StoryPoints == "" || m.ParentKey == "" {
		t.Errorf("template headers must auto-map: %+v", m)
	}
	for i, row := range records[1:] {
		if row[7] != "" {
			t.Errorf("row %d parent cell must be empty: %q", i+2, row[7])
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
	if res.Rows != 4 || len(res.Errors) != 0 {
		t.Errorf("template dry run against a fresh repo: %+v", res)
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
