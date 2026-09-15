package jira_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
)

func TestGetIssueParsesLikeSearch(t *testing.T) {
	b, _ := newBackend(t, twoFields)
	iss, err := b.GetIssue(context.Background(), "PLAT-412")
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if iss.Key != "PLAT-412" || iss.StoryPoints == nil || *iss.StoryPoints != 5 {
		t.Errorf("issue = %+v", iss)
	}
}

func TestUpdateIssueMapsTheSixFields(t *testing.T) {
	b, f := newBackend(t, twoFields)
	err := b.UpdateIssue(context.Background(), "PLAT-412", map[string]string{
		"summary": "New title", "description": "Body", "priority": "High", "assignee": "jdoe",
		"labels": "checkout, promo", "storyPoints": "8",
	})
	if err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	if len(f.writes) != 1 || !strings.HasPrefix(f.writes[0], "PUT /rest/api/2/issue/PLAT-412 ") {
		t.Fatalf("writes = %v", f.writes)
	}
	body := f.writes[0]
	for _, want := range []string{
		`"summary":"New title"`, `"description":"Body"`, `"priority":{"name":"High"}`, `"assignee":{"name":"jdoe"}`,
		`"labels":["checkout","promo"]`, `"customfield_10016":8`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body lacks %s: %s", want, body)
		}
	}
	// Clearing assignee and points sends null; an unknown field is refused
	// before any request; points without the custom field are refused too.
	f.writes = nil
	if err := b.UpdateIssue(context.Background(), "PLAT-412", map[string]string{"assignee": "", "storyPoints": ""}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.writes[0], `"assignee":null`) || !strings.Contains(f.writes[0], `"customfield_10016":null`) {
		t.Errorf("clears: %s", f.writes[0])
	}
	if err := b.UpdateIssue(context.Background(), "PLAT-412", map[string]string{"status": "Done"}); err == nil {
		t.Error("status must be refused")
	}
	noPoints, f2 := newBackend(t, `[]`)
	if err := noPoints.UpdateIssue(context.Background(), "PLAT-412", map[string]string{"storyPoints": "3"}); err == nil || len(f2.writes) != 0 {
		t.Errorf("points without the field: err=%v writes=%v", err, f2.writes)
	}
}

func TestCreateIssuePostsTheDraftAndReturnsTheKey(t *testing.T) {
	b, f := newBackend(t, twoFields)
	f.createKey = "PLAT-501"
	key, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{
		Type: backend.TypeBug, Summary: "Promo field accepts spaces", Description: "Steps", Priority: "Low",
		Labels: []string{"promo"}, Assignee: "jdoe", StoryPoints: pts(1),
		Extra: map[string]string{"customfield_10050": "3", "components": "100", "customfield_10060": "free text"},
	})
	if err != nil || key != "PLAT-501" {
		t.Fatalf("CreateIssue: %q %v", key, err)
	}
	var post string
	for _, w := range f.writes {
		if strings.HasPrefix(w, "POST /rest/api/2/issue ") {
			post = w
		}
	}
	for _, want := range []string{
		`"project":{"key":"PLAT"}`, `"issuetype":{"name":"Bug"}`, `"summary":"Promo field accepts spaces"`,
		`"description":"Steps"`, `"priority":{"name":"Low"}`, `"labels":["promo"]`, `"assignee":{"name":"jdoe"}`,
		`"customfield_10016":1`, `"customfield_10050":{"id":"3"}`, `"components":[{"id":"100"}]`, `"customfield_10060":"free text"`,
	} {
		if !strings.Contains(post, want) {
			t.Errorf("POST lacks %s: %s", want, post)
		}
	}
	f.createFail = true
	if _, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{Type: backend.TypeBug, Summary: "x"}); err == nil || !strings.Contains(err.Error(), "Severity is required") {
		t.Errorf("Jira's message must surface: %v", err)
	}
}

// An extra value's JSON shape comes from its create-meta: an id when Jira
// listed allowed values, the typed text as a name when it did not, and a
// comma list split into the array Jira wants.
func TestCreateIssueShapesExtraFromCreateMeta(t *testing.T) {
	b, f := newBackend(t, twoFields)
	f.createKey = "PLAT-502"
	if _, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{
		Type: backend.TypeBug, Summary: "Promo field accepts spaces",
		Extra: map[string]string{
			"components":        "100,101",
			"customfield_10070": "Needs docs",
			"customfield_10071": "alpha, beta",
			"customfield_10050": "3",
		},
	}); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	var post string
	for _, w := range f.writes {
		if strings.HasPrefix(w, "POST /rest/api/2/issue ") {
			post = w
		}
	}
	for _, want := range []string{
		// An array with options takes every id chosen, not only the first.
		`"components":[{"id":"100"},{"id":"101"}]`,
		// An option with no listed values goes by name; {"id": <typed text>}
		// was never a valid id.
		`"customfield_10070":{"value":"Needs docs"}`,
		// A free-string array is a plain list, not a list of ids.
		`"customfield_10071":["alpha","beta"]`,
		`"customfield_10050":{"id":"3"}`,
	} {
		if !strings.Contains(post, want) {
			t.Errorf("POST lacks %s: %s", want, post)
		}
	}
}

// Item 3 of the ticket, second half: customfield_10253 came from a classic
// createmeta answer that listed a field the Story create screen does not
// carry, and Jira answered "Field cannot be set. It is not on the
// appropriate screen". A draft carries the ids its dialog offered, so the
// field stays out of the payload with no network call; and when the live
// per-type answer does not list a field either, it stays out too.
func TestCreateIssueSendsNoFieldThatIsNotOnTheScreen(t *testing.T) {
	b, f := newBackend(t, threeFields)
	f.createKey = "TKT-10"
	if _, err := b.CreateIssue(context.Background(), "TKT", backend.IssueDraft{
		Type: backend.TypeStory, Summary: "Promo input",
		Extra:        map[string]string{"customfield_10253": "Platform", "customfield_10050": "3"},
		ScreenFields: []string{"customfield_10050"},
	}); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	post := f.writes[len(f.writes)-1]
	if strings.Contains(post, "customfield_10253") {
		t.Errorf("a field off the drafted screen must not be sent: %s", post)
	}
	if !strings.Contains(post, `"customfield_10050":{"id":"3"}`) {
		t.Errorf("the screen's own field is shaped and sent: %s", post)
	}

	// A draft from before the set existed: the live per-type answer decides.
	f.createKey = "TKT-11"
	if _, err := b.CreateIssue(context.Background(), "TKT", backend.IssueDraft{
		Type: backend.TypeStory, Summary: "Legacy draft",
		Extra: map[string]string{"customfield_10253": "Platform"},
	}); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	if post := f.writes[len(f.writes)-1]; strings.Contains(post, "customfield_10253") {
		t.Errorf("the per-type answer does not list the field, so it is not sent: %s", post)
	}
}

func TestCreateIssueNeverLetsAnExtraOverwriteABaseField(t *testing.T) {
	b, f := newBackend(t, threeFields)
	f.createKey = "TKT-12"
	if _, err := b.CreateIssue(context.Background(), "TKT", backend.IssueDraft{
		Type: backend.TypeStory, Summary: "Real summary", ParentKey: "TKT-2",
		Extra: map[string]string{"summary": "Fake summary", "customfield_10014": "TKT-99", "customfield_10016": "40"},
	}); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	post := f.writes[len(f.writes)-1]
	for _, bad := range []string{"Fake summary", "TKT-99", `"customfield_10016":40`} {
		if strings.Contains(post, bad) {
			t.Errorf("an extra overwrote a base field (%s): %s", bad, post)
		}
	}
	if !strings.Contains(post, `"summary":"Real summary"`) || !strings.Contains(post, `"customfield_10014":"TKT-2"`) {
		t.Errorf("the form's own values stand: %s", post)
	}
}

func TestUpdateIssuePushesTheEpicLink(t *testing.T) {
	b, f := newBackend(t, threeFields)
	if err := b.UpdateIssue(context.Background(), "PLAT-412", map[string]string{"parentKey": "PLAT-320"}); err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	if !strings.Contains(f.writes[0], `"customfield_10014":"PLAT-320"`) {
		t.Errorf("epic link set: %s", f.writes[0])
	}
	f.writes = nil
	if err := b.UpdateIssue(context.Background(), "PLAT-412", map[string]string{"parentKey": ""}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.writes[0], `"customfield_10014":null`) {
		t.Errorf("clear: %s", f.writes[0])
	}
	noEpic, _ := newBackend(t, twoFields)
	if err := noEpic.UpdateIssue(context.Background(), "PLAT-412", map[string]string{"parentKey": "PLAT-320"}); err == nil {
		t.Error("no Epic Link field must be refused")
	}
}

func TestCreateEpicDefaultsEpicNameAndSendsNoEpicLink(t *testing.T) {
	b, f := newBackend(t, fourFields)
	f.createKey = "PLAT-600"
	if _, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{Type: backend.TypeEpic, Summary: "New epic", ParentKey: "PLAT-350"}); err != nil {
		t.Fatal(err)
	}
	post := f.writes[len(f.writes)-1]
	if strings.Contains(post, "customfield_10014") {
		t.Errorf("an epic sends no Epic Link: %s", post)
	}
	if !strings.Contains(post, `"customfield_10011":"New epic"`) {
		t.Errorf("Epic Name defaults to the summary: %s", post)
	}
	// Epic Name is one of TAM's own fields: an extra naming it is ignored
	// and the summary is what Jira gets, the same as with no extra at all.
	f.createKey = "PLAT-601"
	if _, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{
		Type: backend.TypeEpic, Summary: "Another epic", Extra: map[string]string{"customfield_10011": "Custom name"},
	}); err != nil {
		t.Fatal(err)
	}
	post = f.writes[len(f.writes)-1]
	if !strings.Contains(post, `"customfield_10011":"Another epic"`) || strings.Contains(post, "Custom name") {
		t.Errorf("Epic Name is the summary, whatever Extra says: %s", post)
	}
}

func TestCreateFieldsHidesEpicName(t *testing.T) {
	b, _ := newBackend(t, fourFields)
	specs, err := b.CreateFields(context.Background(), "PLAT", backend.TypeEpic)
	if err != nil {
		t.Fatalf("CreateFields: %v", err)
	}
	if len(specs) != 0 {
		t.Errorf("Epic Name is hidden: %+v", specs)
	}
}

// The classic answer this fake gives for a Bug carries one optional field,
// Environment, which the dialog now offers under More fields.
func TestCreateFieldsOffersRequiredAndOptionalFieldsBeyondTheForm(t *testing.T) {
	b, f := newBackend(t, twoFields)
	specs, err := b.CreateFields(context.Background(), "PLAT", backend.TypeBug)
	if err != nil {
		t.Fatalf("CreateFields: %v", err)
	}
	var seen []string
	for _, s := range specs {
		seen = append(seen, fmt.Sprintf("%s:%s:%v", s.ID, s.Type, s.Required))
	}
	// Sorted by name: Component/s, Environment, Keywords, Release Note,
	// Severity. Story Points and Summary are the form's own.
	want := "components:array:true,environment:string:false,customfield_10071:array:true,customfield_10070:option:true,customfield_10050:option:true"
	if strings.Join(seen, ",") != want {
		t.Errorf("specs = %v", seen)
	}
	if specs[4].Name != "Severity" || len(specs[4].AllowedValues) != 2 || specs[4].AllowedValues[1].Value != "Critical" {
		t.Errorf("severity = %+v", specs[4])
	}
	if specs[0].AllowedValues[0].Value != "Checkout" {
		t.Errorf("array options take name when value is empty: %+v", specs[0])
	}
	found := false
	for _, s := range f.searches {
		if strings.HasPrefix(s, "createmeta ") && strings.Contains(s, "projectKeys=PLAT") && strings.Contains(s, "issuetypeNames=Bug") && strings.Contains(s, "expand=projects.issuetypes.fields") {
			found = true
		}
	}
	if !found {
		t.Errorf("a type with no id in the project list is read through the classic call by name: %v", f.searches)
	}
}

// A Story on TKT is read through the per-type endpoint, which does not list
// customfield_10253, so the dialog never offers it. Epic Link and Sprint are
// the form's own, found by id or by their greenhopper type, and an optional
// attachment is nothing a text form can fill.
func TestCreateFieldsReadsTheScreenAndLeavesOutBaseAndUnfillableFields(t *testing.T) {
	b, f := newBackend(t, threeFields)
	specs, err := b.CreateFields(context.Background(), "TKT", backend.TypeStory)
	if err != nil {
		t.Fatalf("CreateFields: %v", err)
	}
	var seen []string
	for _, s := range specs {
		seen = append(seen, fmt.Sprintf("%s:%s:%v", s.ID, s.Type, s.Required))
	}
	if strings.Join(seen, ",") != "customfield_10300:textarea:false,customfield_10050:option:true" {
		t.Errorf("specs = %v", seen)
	}
	for _, s := range f.searches {
		if strings.HasPrefix(s, "createmeta ") {
			t.Errorf("the per-type endpoint answered, so the classic call is never made: %v", f.searches)
		}
	}
}

// Item 2 of the ticket: a technical task drafted from a story was asked for
// its parent a second time, because createmeta lists parent as required.
func TestCreateFieldsNeverOffersTheParentOfASubtask(t *testing.T) {
	b, _ := newBackend(t, threeFields)
	specs, err := b.CreateFields(context.Background(), "TKT", backend.TypeSubtask)
	if err != nil {
		t.Fatalf("CreateFields: %v", err)
	}
	if len(specs) != 1 || specs[0].ID != "customfield_10300" {
		t.Errorf("specs = %+v, want only Acceptance criteria", specs)
	}
}

func TestCreateIssueSendsTheParentThroughEpicLinkWhenItExists(t *testing.T) {
	b, f := newBackend(t, threeFields)
	f.createKey = "PLAT-502"
	if _, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{Type: backend.TypeStory, Summary: "Under an epic", ParentKey: "PLAT-350"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.writes[len(f.writes)-1], `"customfield_10014":"PLAT-350"`) {
		t.Errorf("epic link missing: %s", f.writes[len(f.writes)-1])
	}
	noEpic, f2 := newBackend(t, twoFields)
	f2.createKey = "PLAT-503"
	if _, err := noEpic.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{Type: backend.TypeStory, Summary: "No field", ParentKey: "PLAT-350"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(f2.writes[len(f2.writes)-1], "PLAT-350") {
		t.Errorf("the parent must be dropped when the field is missing: %s", f2.writes[len(f2.writes)-1])
	}
}

func pts(v float64) *float64 { return &v }
