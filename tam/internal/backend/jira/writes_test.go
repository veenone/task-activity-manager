package jira_test

import (
	"context"
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
	f.createKey = "PLAT-601"
	if _, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{
		Type: backend.TypeEpic, Summary: "Another epic", Extra: map[string]string{"customfield_10011": "Custom name"},
	}); err != nil {
		t.Fatal(err)
	}
	post = f.writes[len(f.writes)-1]
	if !strings.Contains(post, `"customfield_10011":"Custom name"`) {
		t.Errorf("Extra's Epic Name wins: %s", post)
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

func TestCreateFieldsKeepsOnlyRequiredUnknownFields(t *testing.T) {
	b, f := newBackend(t, twoFields)
	specs, err := b.CreateFields(context.Background(), "PLAT", backend.TypeBug)
	if err != nil {
		t.Fatalf("CreateFields: %v", err)
	}
	var seen []string
	for _, s := range specs {
		seen = append(seen, s.ID+":"+s.Type)
	}
	// Sorted by name: Component/s, Keywords, Release Note, Severity.
	if strings.Join(seen, ",") != "components:array,customfield_10071:array,customfield_10070:option,customfield_10050:option" {
		t.Errorf("specs = %v", seen)
	}
	if specs[3].Name != "Severity" || len(specs[3].AllowedValues) != 2 || specs[3].AllowedValues[1].Value != "Critical" {
		t.Errorf("severity = %+v", specs[3])
	}
	if specs[0].AllowedValues[0].Value != "Checkout" {
		t.Errorf("array options take name when value is empty: %+v", specs[0])
	}
	// A field whose create-meta lists no allowed values is what tells the form
	// to offer free text and the create to send a name rather than an id.
	if len(specs[1].AllowedValues) != 0 || len(specs[2].AllowedValues) != 0 {
		t.Errorf("fields without options must report none: %+v %+v", specs[1], specs[2])
	}
	found := false
	for _, s := range f.searches {
		if strings.HasPrefix(s, "createmeta ") && strings.Contains(s, "projectKeys=PLAT") && strings.Contains(s, "issuetypeNames=Bug") && strings.Contains(s, "expand=projects.issuetypes.fields") {
			found = true
		}
	}
	if !found {
		t.Errorf("createmeta query: %v", f.searches)
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
