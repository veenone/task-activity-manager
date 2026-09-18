package jira_test

import (
	"context"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
)

// threeFieldsNamed is threeFields plus the two fields these errors name, so
// the instance can say what "customfield_10253" is called.
const threeFieldsNamed = `[{"id":"customfield_10020","name":"Sprint","custom":true},
	{"id":"customfield_10016","name":"Story Points","custom":true},
	{"id":"customfield_10014","name":"Epic Link","custom":true},
	{"id":"customfield_10253","name":"Team","custom":true},
	{"id":"customfield_10050","name":"Severity","custom":true}]`

// The user's report: a raw id and a raw Jira sentence, with nothing saying
// which field in their dialog it was. The instance names the field, so TAM
// can too.
func TestCreateIssueNamesTheFieldJiraRefused(t *testing.T) {
	b, f := newBackend(t, threeFieldsNamed)
	f.writeFail = `{"errorMessages":[],"errors":{"customfield_10253":"Field 'customfield_10253' cannot be set. It is not on the appropriate screen, or unknown."}}`
	_, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{Type: backend.TypeBug, Summary: "Promo input"})
	if err == nil {
		t.Fatal("want an error")
	}
	want := "Jira refused the field Team (customfield_10253): Field 'customfield_10253' cannot be set. " +
		"It is not on the appropriate screen, or unknown. TAM sent it because Jira's create metadata for this " +
		"issue type lists it; if it is not on the create screen, a Jira administrator has to put it there."
	if err.Error() != want {
		t.Errorf("err  = %q\nwant = %q", err.Error(), want)
	}
	// The REST prefix is noise in front of a sentence written for a person,
	// and this string is what the Commit failure line shows verbatim.
	if strings.Contains(err.Error(), "POST /rest/api/2/issue") {
		t.Errorf("the humanized message carries no transport detail: %q", err)
	}
}

// Jira answers with a map, so one refusal can name several fields. Dropping
// all but the first would hide half of what has to be fixed.
func TestCreateIssueNamesEveryFieldJiraRefused(t *testing.T) {
	b, f := newBackend(t, threeFieldsNamed)
	f.writeFail = `{"errorMessages":[],"errors":{` +
		`"customfield_10253":"Field 'customfield_10253' cannot be set. It is not on the appropriate screen, or unknown.",` +
		`"customfield_10050":"Severity is required.",` +
		`"customfield_10777":"Field 'customfield_10777' cannot be set. It is not on the appropriate screen, or unknown."}}`
	_, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{Type: backend.TypeBug, Summary: "Promo input"})
	if err == nil {
		t.Fatal("want an error")
	}
	want := "Jira refused 3 fields. " +
		"Severity (customfield_10050): Severity is required. " +
		"Team (customfield_10253): Field 'customfield_10253' cannot be set. It is not on the appropriate screen, or unknown. " +
		"customfield_10777: Field 'customfield_10777' cannot be set. It is not on the appropriate screen, or unknown. " +
		"TAM sent them because Jira's create metadata for this issue type lists them; if they are not on the create " +
		"screen, a Jira administrator has to put them there."
	if err.Error() != want {
		t.Errorf("err  = %q\nwant = %q", err.Error(), want)
	}
}

// A field the instance does not name keeps its id. Inventing a name would
// send the user looking for a field that is not called that.
func TestCreateIssueShowsTheIdWhenTheInstanceHasNoName(t *testing.T) {
	b, f := newBackend(t, `[]`)
	f.writeFail = `{"errorMessages":[],"errors":{"customfield_10253":"Severity is required."}}`
	_, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{Type: backend.TypeBug, Summary: "x"})
	// Exact, not a prefix: a refusal that is not about the create screen
	// gets the naming and nothing else. The advice would be wrong here.
	if err == nil || err.Error() != "Jira refused the field customfield_10253: Severity is required." {
		t.Errorf("err = %v", err)
	}
}

// Jira's free-standing errorMessages are not about a field and are carried
// through as they are, beside the fields that are.
func TestCreateIssueKeepsTheNonFieldPartOfAJiraError(t *testing.T) {
	b, f := newBackend(t, threeFieldsNamed)
	f.writeFail = `{"errorMessages":["You do not have permission to create issues in this project."],"errors":{"customfield_10253":"Severity is required."}}`
	_, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{Type: backend.TypeBug, Summary: "x"})
	if err == nil {
		t.Fatal("want an error")
	}
	want := "You do not have permission to create issues in this project. " +
		"Jira refused the field Team (customfield_10253): Severity is required."
	if err.Error() != want {
		t.Errorf("err = %q, want %q", err.Error(), want)
	}
}

// A refusal that names no field is left exactly as it was: the transport
// detail is the only thing there is to go on.
func TestCreateIssueLeavesAFieldlessErrorAlone(t *testing.T) {
	b, f := newBackend(t, threeFieldsNamed)
	f.writeFail = `{"errorMessages":["Issue type is not valid."],"errors":{}}`
	_, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{Type: backend.TypeBug, Summary: "x"})
	if err == nil || !strings.Contains(err.Error(), "jira: POST /rest/api/2/issue") || !strings.Contains(err.Error(), "Issue type is not valid.") {
		t.Errorf("err = %v", err)
	}
}

// The edit path carries field ids too, and a required field refused there
// reads as badly as one refused on a create. Its advice is not the create's:
// nothing about a create screen applies.
func TestUpdateIssueNamesTheFieldJiraRefused(t *testing.T) {
	b, f := newBackend(t, threeFieldsNamed)
	f.writeFail = `{"errorMessages":[],"errors":{"customfield_10050":"Severity is required."}}`
	err := b.UpdateIssue(context.Background(), "PLAT-412", map[string]string{"summary": "New title"})
	if err == nil {
		t.Fatal("want an error")
	}
	want := "Jira refused the field Severity (customfield_10050): Severity is required."
	if err.Error() != want {
		t.Errorf("err  = %q\nwant = %q", err.Error(), want)
	}
}
