package jira_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
)

// postOf is the body of the POST that created an issue, and "" when none was
// sent.
func postOf(f *fakeJira) string {
	for _, w := range f.writes {
		if strings.HasPrefix(w, "POST /rest/api/2/issue ") {
			return w
		}
	}
	return ""
}

// TKT's Story create screen is the per-type answer the fake gives: it lists
// the Epic Link and does not list Story Points. That is the same shape the
// edit screen behind #52 has, on the other half of the write path.
func TestCreateIssueLeavesOutAnEstimateTheCreateScreenLacks(t *testing.T) {
	b, f := newBackend(t, fourFields)
	f.createKey = "TKT-77"
	key, leftOut, err := b.CreateIssue(context.Background(), "TKT", backend.IssueDraft{
		Type: backend.TypeStory, Summary: "Promo code at payment", StoryPoints: pts(5), ParentKey: "TKT-10",
	})
	if err != nil || key != "TKT-77" {
		t.Fatalf("CreateIssue: %q %v", key, err)
	}
	post := postOf(f)
	if post == "" {
		t.Fatal("the create must still go: a field off the screen is not a reason to lose the issue")
	}
	if strings.Contains(post, "customfield_10016") {
		t.Errorf("the estimate is not on the create screen and must not be sent: %s", post)
	}
	// The Epic Link is on that screen, so the parent still goes.
	if !strings.Contains(post, `"customfield_10014":"TKT-10"`) {
		t.Errorf("POST lacks the epic: %s", post)
	}
	// What did not go is named, rather than dropped in silence.
	if !reflect.DeepEqual(leftOut, []string{"Story Points (customfield_10016)"}) {
		t.Errorf("leftOut = %v", leftOut)
	}
}

func TestCreateIssueNamesAnEpicTheCreateScreenWillNotTake(t *testing.T) {
	b, f := newBackend(t, fourFields)
	f.createKey = "TKT-78"
	f.perTypeStory = `{"startAt":0,"maxResults":50,"total":2,"isLast":true,"values":[
		{"fieldId":"summary","name":"Summary","required":true,"schema":{"type":"string","system":"summary"}},
		{"fieldId":"customfield_10016","name":"Story Points","required":false,"schema":{"type":"number"}}
	]}`
	_, leftOut, err := b.CreateIssue(context.Background(), "TKT", backend.IssueDraft{
		Type: backend.TypeStory, Summary: "Promo code at payment", StoryPoints: pts(5), ParentKey: "TKT-10",
	})
	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	post := postOf(f)
	if strings.Contains(post, "customfield_10014") {
		t.Errorf("the Epic Link is not on this screen and must not be sent: %s", post)
	}
	if !strings.Contains(post, `"customfield_10016":5`) {
		t.Errorf("the estimate is on this screen and must be sent: %s", post)
	}
	if !reflect.DeepEqual(leftOut, []string{"Epic Link (customfield_10014)"}) {
		t.Errorf("leftOut = %v", leftOut)
	}
}

func TestCreateIssueLeavesOutAFieldTheCreateScreenWillNotLetItSet(t *testing.T) {
	b, f := newBackend(t, fourFields)
	f.createKey = "TKT-79"
	// On the screen, but a create may only add to and remove from it, which
	// is not what setting an estimate does.
	f.perTypeStory = `{"startAt":0,"maxResults":50,"total":2,"isLast":true,"values":[
		{"fieldId":"summary","name":"Summary","required":true,"operations":["set"],"schema":{"type":"string","system":"summary"}},
		{"fieldId":"customfield_10016","name":"Story Points","required":false,"operations":["add","remove"],"schema":{"type":"number"}}
	]}`
	_, leftOut, err := b.CreateIssue(context.Background(), "TKT", backend.IssueDraft{
		Type: backend.TypeStory, Summary: "Promo code at payment", StoryPoints: pts(5),
	})
	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	if strings.Contains(postOf(f), "customfield_10016") {
		t.Errorf("a field a create may not set must not be sent: %s", postOf(f))
	}
	if !reflect.DeepEqual(leftOut, []string{"Story Points (customfield_10016)"}) {
		t.Errorf("leftOut = %v", leftOut)
	}
}

// The one that matters most: an instance whose create metadata cannot be
// read at all must create exactly as it did before. Not knowing a field is
// absent is not knowing it is, and reading silence as absence would quietly
// strip the estimate off every create on such an instance.
func TestCreateIssueSendsItsOwnFieldsWhenTheCreateScreenCannotBeRead(t *testing.T) {
	b, f := newBackend(t, fourFields)
	f.createKey = "DOWN-3"
	// DOWN's issue type list answers 503, so createMeta fails outright.
	_, leftOut, err := b.CreateIssue(context.Background(), "DOWN", backend.IssueDraft{
		Type: backend.TypeStory, Summary: "Promo code at payment", StoryPoints: pts(5), ParentKey: "DOWN-1",
	})
	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	post := postOf(f)
	if !strings.Contains(post, `"customfield_10016":5`) || !strings.Contains(post, `"customfield_10014":"DOWN-1"`) {
		t.Errorf("a screen nobody could read must change nothing: %s", post)
	}
	if len(leftOut) != 0 {
		t.Errorf("nothing was left out, so nothing may be named: %v", leftOut)
	}
}

// An epic's own name is TAM's, not the user's: it is the summary they typed,
// which goes as the summary regardless. A screen without the field drops it
// without naming it, the same rule applyExtras already states for a field
// TAM sets itself.
func TestCreateIssueDropsAnEpicNameTheScreenLacksWithoutNamingIt(t *testing.T) {
	b, f := newBackend(t, fourFields)
	f.createKey = "TKT-80"
	f.perTypeEpicID = "10005"
	f.perTypeEpic = `{"startAt":0,"maxResults":50,"total":1,"isLast":true,"values":[
		{"fieldId":"summary","name":"Summary","required":true,"schema":{"type":"string","system":"summary"}}
	]}`
	_, leftOut, err := b.CreateIssue(context.Background(), "TKT", backend.IssueDraft{
		Type: backend.TypeEpic, Summary: "Promotions and discounts",
	})
	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	post := postOf(f)
	if strings.Contains(post, "customfield_10011") {
		t.Errorf("an Epic Name off the screen must not be sent: %s", post)
	}
	if !strings.Contains(post, `"summary":"Promotions and discounts"`) {
		t.Errorf("the summary the user typed still goes: %s", post)
	}
	if len(leftOut) != 0 {
		t.Errorf("nothing the user typed was lost, so nothing is named: %v", leftOut)
	}
}

// Reading the screen costs a request, and a create that carries nothing the
// screen could refuse must not pay it: an import creates issues by the
// hundred, and a request per row is what that notices.
func TestCreateIssueAsksForNoCreateScreenWhenNothingCouldBeRefused(t *testing.T) {
	b, f := newBackend(t, fourFields)
	f.createKey = "TKT-81"
	_, leftOut, err := b.CreateIssue(context.Background(), "TKT", backend.IssueDraft{
		Type: backend.TypeStory, Summary: "Promo code at payment", Description: "Steps", Priority: "Low",
	})
	if err != nil || len(leftOut) != 0 {
		t.Fatalf("CreateIssue: %v %v", leftOut, err)
	}
	for _, req := range f.searches {
		if strings.HasPrefix(req, "createmeta") {
			t.Errorf("no create metadata may be read for this draft: %v", f.searches)
		}
	}
	if postOf(f) == "" {
		t.Error("the create still has to go")
	}
}
