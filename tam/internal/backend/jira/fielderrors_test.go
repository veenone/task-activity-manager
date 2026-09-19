package jira_test

import (
	"context"
	"encoding/json"
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
	_, _, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{Type: backend.TypeBug, Summary: "Promo input"})
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
	_, _, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{Type: backend.TypeBug, Summary: "Promo input"})
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
	_, _, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{Type: backend.TypeBug, Summary: "x"})
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
	_, _, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{Type: backend.TypeBug, Summary: "x"})
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
	_, _, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{Type: backend.TypeBug, Summary: "x"})
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

// The user's stuck draft, as their journal row holds it: an extra chosen in
// an older session, against a create screen TAM can no longer read. Their
// instance answers the create-metadata call with an error, which used to
// skip both screen checks and send every extra as text, and Jira refused the
// create every time. Nothing confirms the field now, so it is left out and
// named, and the create goes through.
func TestCreateIssueLeavesOutExtrasItCannotConfirm(t *testing.T) {
	for _, tc := range []struct {
		name  string
		after string
	}{
		{
			// A row from before screenFields existed at all.
			name:  "a draft with no screen fields",
			after: `{"type":"story","summary":"Promo input","description":"","priority":"","labels":[],"assignee":"","storyPoints":null,"parentKey":"","statusId":"","sprintId":"","sprintName":"","extra":{"customfield_10253":"Platform"}}`,
		},
		{
			// A row whose screen was read when the dialog still could, so it
			// vouches for a field this instance now refuses. A stale vouching
			// is not a confirmation.
			name:  "a draft whose screen fields vouch for it",
			after: `{"type":"story","summary":"Promo input","labels":[],"extra":{"customfield_10253":"Platform"},"screenFields":["customfield_10253"]}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var d backend.IssueDraft
			if err := json.Unmarshal([]byte(tc.after), &d); err != nil {
				t.Fatalf("decode the stored draft: %v", err)
			}
			b, f := newBackend(t, threeFieldsNamed)
			f.createKey = "DOWN-1"
			// DOWN is the project whose type list cannot be read, which is
			// what makes createMeta fail the way their instance does.
			key, leftOut, err := b.CreateIssue(context.Background(), "DOWN", d)
			if err != nil || key != "DOWN-1" {
				t.Fatalf("the create still goes through: %q %v", key, err)
			}
			post := f.writes[len(f.writes)-1]
			if strings.Contains(post, "customfield_10253") {
				t.Errorf("an unconfirmable extra must stay out of the payload: %s", post)
			}
			if !strings.Contains(post, `"summary":"Promo input"`) {
				t.Errorf("the form's own fields still go: %s", post)
			}
			if len(leftOut) != 1 || leftOut[0] != "Team (customfield_10253)" {
				t.Errorf("leftOut = %+v, want the field named so Commit can say what it dropped", leftOut)
			}
		})
	}
}

// A readable screen is still trusted: this is not a licence to drop every
// extra, only the ones nothing can vouch for.
func TestCreateIssueStillSendsAConfirmedExtra(t *testing.T) {
	b, f := newBackend(t, threeFieldsNamed)
	f.createKey = "TKT-40"
	key, leftOut, err := b.CreateIssue(context.Background(), "TKT", backend.IssueDraft{
		Type: backend.TypeStory, Summary: "Promo input",
		Extra:        map[string]string{"customfield_10300": "Given a promo"},
		ScreenFields: []string{"customfield_10300"},
	})
	if err != nil || key != "TKT-40" {
		t.Fatalf("CreateIssue: %q %v", key, err)
	}
	if len(leftOut) != 0 {
		t.Errorf("nothing was left out: %+v", leftOut)
	}
	if !strings.Contains(f.writes[len(f.writes)-1], `"customfield_10300":"Given a promo"`) {
		t.Errorf("a confirmed extra is still sent: %s", f.writes[len(f.writes)-1])
	}
}
