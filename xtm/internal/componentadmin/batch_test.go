package componentadmin

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/testrepo"
)

func problems(rows []BatchRow) []string {
	out := []string{}
	for _, r := range rows {
		out = append(out, r.Problem)
	}
	return out
}

func TestCheckFlagsBlankDuplicateAndExistingNames(t *testing.T) {
	b := &fakeBackend{list: []backend.Component{{ID: "1", Name: "Core"}}}
	rows, err := newSvc(&fakeRepo{}, b).Check(context.Background(), []backend.ComponentInput{
		{Name: " API "},
		{Name: "  "},
		{Name: "core"},
		{Name: "api"},
		{Name: "UI", AssigneeType: "Somebody"},
	})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	want := []string{
		"",
		"A component needs a name.",
		`The project already has a component named "Core".`,
		`Another row also creates "API".`,
		"Default assignee must be Project default, Component lead, Project lead or Unassigned.",
	}
	if got := problems(rows); !reflect.DeepEqual(got, want) {
		t.Fatalf("problems\n got %q\nwant %q", got, want)
	}
	if rows[0].Input.Name != "API" || rows[0].Input.AssigneeType != "PROJECT_DEFAULT" {
		t.Fatalf("row 0 not cleaned: %+v", rows[0].Input)
	}
}

func TestCreateManyCreatesValidRowsPastFailures(t *testing.T) {
	repo := &fakeRepo{}
	b := &fakeBackend{
		list:   []backend.Component{{ID: "1", Name: "Core"}},
		failOn: map[string]error{"Bad": errors.New("lead does not exist")},
	}
	res, err := newSvc(repo, b).CreateMany(context.Background(), []backend.ComponentInput{
		{Name: "API", Description: "Public API", LeadUserName: "jdoe", AssigneeType: "COMPONENT_LEAD"},
		{Name: "Bad"},
		{Name: "CORE"},
		{Name: "UI"},
	})
	if err != nil {
		t.Fatalf("create many: %v", err)
	}
	var created []string
	for _, c := range res.Created {
		created = append(created, c.Name)
	}
	if !reflect.DeepEqual(created, []string{"API", "UI"}) {
		t.Fatalf("created %v", created)
	}
	wantFailed := []BatchFailure{
		{Row: 2, Name: "Bad", Error: "lead does not exist"},
		{Row: 3, Name: "CORE", Error: `The project already has a component named "Core".`},
	}
	if !reflect.DeepEqual(res.Failed, wantFailed) {
		t.Fatalf("failed %+v", res.Failed)
	}
	if b.created[0] != (backend.ComponentInput{Name: "API", Description: "Public API", LeadUserName: "jdoe", AssigneeType: "COMPONENT_LEAD"}) {
		t.Fatalf("sent %+v", b.created[0])
	}
	if !reflect.DeepEqual(repo.options, []string{"Core", "API", "UI"}) {
		t.Fatalf("cache %v", repo.options)
	}
}

func TestWorkbookRoundTripsTheTemplate(t *testing.T) {
	data, err := TemplateWorkbook()
	if err != nil {
		t.Fatalf("template: %v", err)
	}
	rows, err := ParseWorkbook(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rows) == 0 || rows[0].Name == "" {
		t.Fatalf("template rows %+v", rows)
	}
}

func TestParseWorkbookMapsColumnsAndAssigneeLabels(t *testing.T) {
	data, err := testrepo.WriteXLSXSheets([]testrepo.NamedRows{{
		Name:   "Components",
		Header: []string{"default assignee", "Name", "Lead Username", "Description"},
		Rows: [][]string{
			{"Component lead", "API", "jdoe", "Public API"},
			{"", "", "", ""},
			{"UNASSIGNED", "UI", "", ""},
			{"", "", "", "orphan description"},
		},
	}})
	if err != nil {
		t.Fatalf("workbook: %v", err)
	}
	rows, err := ParseWorkbook(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []backend.ComponentInput{
		{Name: "API", Description: "Public API", LeadUserName: "jdoe", AssigneeType: "COMPONENT_LEAD"},
		{Name: "UI", AssigneeType: "UNASSIGNED"},
		{Description: "orphan description"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows\n got %+v\nwant %+v", rows, want)
	}
}

func TestParseWorkbookNeedsANameColumn(t *testing.T) {
	data, _ := testrepo.WriteXLSXSheets([]testrepo.NamedRows{{
		Name: "Components", Header: []string{"Title"}, Rows: [][]string{{"API"}},
	}})
	_, err := ParseWorkbook(data)
	if err == nil || err.Error() != "The workbook has no Name column. Start from the template." {
		t.Fatalf("err %v", err)
	}
}
