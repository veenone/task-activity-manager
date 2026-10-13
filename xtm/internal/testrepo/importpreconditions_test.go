package testrepo_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"agile-suite/xtm/internal/testrepo"
)

var precondMapping = testrepo.PreconditionImportMapping{
	Summary: "Summary", Type: "Type", Condition: "Condition", Description: "Description", Labels: "Labels",
}

func precondRecords(rows ...[]string) [][]string {
	return append([][]string{{"Summary", "Type", "Condition", "Description", "Labels"}}, rows...)
}

func seedPrecondition(t *testing.T, repo *testrepo.Repository, summary string) {
	t.Helper()
	if _, err := repo.CreatePrecondition("p1", "QA", summary, "Manual", "", nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func pendingOfType(t *testing.T, repo *testrepo.Repository, entityType string) []testrepo.PendingChange {
	t.Helper()
	all, err := repo.ListPendingChanges("p1")
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	var out []testrepo.PendingChange
	for _, c := range all {
		if c.EntityType == entityType {
			out = append(out, c)
		}
	}
	return out
}

func statuses(res testrepo.PreconditionImportResult) []string {
	out := []string{}
	for _, r := range res.Rows {
		out = append(out, r.Status)
	}
	return out
}

func TestValidateReportsNewAndExistingWithoutWriting(t *testing.T) {
	repo := newRepo(t)
	seedPrecondition(t, repo, "Network is up")

	res, err := repo.ImportPreconditions("p1", "QA", precondRecords(
		[]string{"network  IS up", "", "", "", ""},
		[]string{"Card inserted", "generic", "", "", ""},
		[]string{"Card inserted", "", "", "", ""},
	), precondMapping, true, nil)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if want := []string{"existing", "new", "existing"}; !reflect.DeepEqual(statuses(res), want) {
		t.Fatalf("statuses %v, want %v", statuses(res), want)
	}
	if res.Rows[1].Type != "Generic" {
		t.Fatalf("type not normalised: %q", res.Rows[1].Type)
	}
	if res.Created != 0 || res.NewCount != 1 || res.ExistingCount != 2 {
		t.Fatalf("counts %+v", res)
	}
	if got := pendingOfType(t, repo, "precondition_add"); len(got) != 1 {
		t.Fatalf("validate wrote %d creates", len(got)-1)
	}
}

func TestImportQueuesCreatesWithConditionLabelsAndFields(t *testing.T) {
	repo := newRepo(t)
	team := map[string]any{"customfield_20000": map[string]any{"id": "3"}}

	res, err := repo.ImportPreconditions("p1", "QA", precondRecords(
		[]string{"Card inserted", "Cucumber", "Given a card", "A note", "smoke card"},
	), precondMapping, false, team)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.Created != 1 {
		t.Fatalf("created %d: %+v", res.Created, res)
	}
	adds := pendingOfType(t, repo, "precondition_add")
	if len(adds) != 1 {
		t.Fatalf("adds %d", len(adds))
	}
	var payload struct {
		Type   string         `json:"type"`
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal([]byte(adds[0].AfterVal), &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if payload.Type != "Cucumber" {
		t.Fatalf("type %q", payload.Type)
	}
	if !reflect.DeepEqual(payload.Fields["labels"], []any{"smoke", "card"}) || payload.Fields["customfield_20000"] == nil {
		t.Fatalf("fields %v", payload.Fields)
	}
	edits := pendingOfType(t, repo, "precondition_edit")
	if len(edits) != 1 || edits[0].Field != "condition" || edits[0].AfterVal != "Given a card" {
		t.Fatalf("condition edit %+v", edits)
	}
}

func TestReimportingTheSameFileCreatesNothing(t *testing.T) {
	repo := newRepo(t)
	file := precondRecords([]string{"Card inserted", "", "", "", ""})
	if _, err := repo.ImportPreconditions("p1", "QA", file, precondMapping, false, nil); err != nil {
		t.Fatalf("first: %v", err)
	}
	res, err := repo.ImportPreconditions("p1", "QA", file, precondMapping, false, nil)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if res.Created != 0 || len(pendingOfType(t, repo, "precondition_add")) != 1 {
		t.Fatalf("re-import created %d", res.Created)
	}
}

func TestImportStopsOnAnUnknownTypeAndWritesNothing(t *testing.T) {
	repo := newRepo(t)

	res, err := repo.ImportPreconditions("p1", "QA", precondRecords(
		[]string{"Card inserted", "Manual", "", "", ""},
		[]string{"Reader on", "Automated", "", "", ""},
		[]string{"", "", "", "orphan", ""},
	), precondMapping, false, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.Created != 0 || len(pendingOfType(t, repo, "precondition_add")) != 0 {
		t.Fatalf("partial import: created %d", res.Created)
	}
	if !reflect.DeepEqual(res.UnknownTypes, []string{"Automated"}) {
		t.Fatalf("unknown types %v", res.UnknownTypes)
	}
	want := []testrepo.ImportError{
		{Row: 3, Message: `Type "Automated" is not Manual, Generic or Cucumber.`},
		{Row: 4, Message: "Summary is empty."},
	}
	if !reflect.DeepEqual(res.Errors, want) {
		t.Fatalf("errors %+v", res.Errors)
	}
}

func TestPreconditionTemplateRoundTrips(t *testing.T) {
	repo := newRepo(t)
	records, err := testrepo.ParseRecords([]byte(testrepo.PreconditionImportTemplateCSV()), false)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if strings.Join(records[0], ",") != "Summary,Type,Condition,Description,Labels" {
		t.Fatalf("header %v", records[0])
	}
	res, err := repo.ImportPreconditions("p1", "QA", records, precondMapping, true, nil)
	if err != nil || len(res.Errors) != 0 || res.NewCount != len(records)-1 {
		t.Fatalf("template does not validate cleanly: %+v, %v", res, err)
	}
}
