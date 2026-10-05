package importer_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/importer"
)

// improvementRows are a sheet carrying a type the project has and TAM does
// not model, which is the shape issue #137 is about.
func improvementRows() [][]string {
	return [][]string{
		{"Issue Type", "Summary"},
		{"Improvement", "Trim the checkout step"},
		{"Task", "Rotate the keys"},
	}
}

// The import takes the same types the New issue dialog offers. Before
// this, a sheet with Improvement in it failed row by row while the dialog
// offered that very type.
func TestImportTakesATypeTheProjectOffers(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	if err := repo.PutProjectTypes(ctx, "p1", []backend.IssueType{
		{ID: "1", Name: "Task", Logical: backend.TypeTask},
		{ID: "4", Name: "Improvement"},
	}); err != nil {
		t.Fatalf("record the types: %v", err)
	}
	rows := improvementRows()
	res, err := importer.Run(ctx, repo, "p1", "PLAT", "", nil, rows, importer.AutoMap(rows[0]), "backlog.csv", false)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("errors = %+v, want none", res.Errors)
	}
	if len(res.Created) != 2 {
		t.Fatalf("created = %v, want both rows", res.Created)
	}
	iss, err := repo.GetIssue(ctx, "p1", res.Created[0])
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if iss.Type != "Improvement" {
		t.Errorf("type = %q, want the project's own name", iss.Type)
	}
}

// A type nobody offers still fails its row, and the message names what
// this project takes rather than only TAM's own list.
func TestImportStillRefusesATypeTheProjectDoesNotOffer(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	if err := repo.PutProjectTypes(ctx, "p1", []backend.IssueType{
		{ID: "1", Name: "Task", Logical: backend.TypeTask},
		{ID: "4", Name: "Improvement"},
	}); err != nil {
		t.Fatalf("record the types: %v", err)
	}
	rows := [][]string{{"Issue Type", "Summary"}, {"Spike", "Try the other gateway"}}
	res, err := importer.Run(ctx, repo, "p1", "PLAT", "", nil, rows, importer.AutoMap(rows[0]), "backlog.csv", false)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(res.Errors) != 1 {
		t.Fatalf("errors = %+v, want the one row", res.Errors)
	}
	if !strings.Contains(res.Errors[0].Message, "Improvement") {
		t.Errorf("message = %q, want it to name what the project offers", res.Errors[0].Message)
	}
}

// A profile whose types have never been synced keeps the old list, which
// is what the dialog falls back to as well.
func TestImportFallsBackToTheModelledTypesWithNoSyncedList(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	rows := [][]string{{"Issue Type", "Summary"}, {"Story", "Guest checkout"}, {"Improvement", "Trim it"}}
	res, err := importer.Run(ctx, repo, "p1", "PLAT", "", nil, rows, importer.AutoMap(rows[0]), "backlog.csv", false)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(res.Created) != 1 {
		t.Errorf("created = %v, want the story alone", res.Created)
	}
	if len(res.Errors) != 1 {
		t.Errorf("errors = %+v, want the unmodelled row refused", res.Errors)
	}
}

// The template's Type column is a dropdown, and Excel refuses anything
// off it before TAM ever sees the sheet. So the list is the project's own
// types when there are any, which is what the import now takes.
func TestTheTemplateOffersTheProjectsOwnTypes(t *testing.T) {
	data, err := importer.TemplateXLSX("Business Requirement", nil, []backend.IssueType{
		{ID: "1", Name: "Task", Logical: backend.TypeTask},
		{ID: "4", Name: "Improvement"},
		{ID: "5", Name: "Technical task", Subtask: true, Logical: backend.TypeSubtask},
	})
	if err != nil {
		t.Fatalf("TemplateXLSX: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open the workbook: %v", err)
	}
	defer f.Close()
	validations, err := f.GetDataValidations("Issues")
	if err != nil {
		t.Fatalf("read the validations: %v", err)
	}
	var list string
	for _, v := range validations {
		if strings.HasPrefix(v.Sqref, "B") && v.Formula1 != "" {
			list = v.Formula1
		}
	}
	if !strings.Contains(list, "Improvement") {
		t.Errorf("type dropdown = %q, want the project's own types", list)
	}
	// A sub-task cannot be drafted at the top level, so it is not offered
	// here either.
	if strings.Contains(list, "Technical task") {
		t.Errorf("type dropdown = %q, want no sub-task level in it", list)
	}
}
