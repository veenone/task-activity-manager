package jira_test

import (
	"context"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
)

// The sub-task type is whatever the project calls it, so it is read off the
// project rather than assumed to be "Sub-task".
func TestSubtaskTypeNameComesFromTheProject(t *testing.T) {
	b, f := newBackend(t, twoFields)
	name, err := b.SubtaskTypeName(context.Background(), "PLAT")
	if err != nil {
		t.Fatalf("SubtaskTypeName: %v", err)
	}
	if name != "Technical task" {
		t.Fatalf("name = %q, want %q", name, "Technical task")
	}
	// Cached: a second ask costs no request, so a sync's every page does not
	// re-read the project.
	before := len(f.searches)
	if _, err := b.SubtaskTypeName(context.Background(), "PLAT"); err != nil {
		t.Fatal(err)
	}
	if len(f.searches) != before {
		t.Errorf("the second lookup made a request: %v", f.searches[before:])
	}
}

// A sub-task hangs off its parent through Jira's own parent field. The Epic
// Link is a different relationship and must not carry it.
func TestCreateSubtaskSendsTheParentField(t *testing.T) {
	b, f := newBackend(t, threeFields)
	f.createKey = "PLAT-700"
	if _, _, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{
		Type: backend.TypeSubtask, Summary: "Wire the promo input", ParentKey: "PLAT-412",
	}); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	var post string
	for _, w := range f.writes {
		if strings.HasPrefix(w, "POST /rest/api/2/issue ") {
			post = w
		}
	}
	if !strings.Contains(post, `"parent":{"key":"PLAT-412"}`) {
		t.Errorf("parent field: %s", post)
	}
	if !strings.Contains(post, `"issuetype":{"name":"Technical task"}`) {
		t.Errorf("the project's own sub-task name: %s", post)
	}
	if strings.Contains(post, "customfield_10014") {
		t.Errorf("a sub-task must not carry an Epic Link: %s", post)
	}
}

// Without a parent it is not a sub-task, and the failure belongs here rather
// than in a Jira 400 at Commit.
func TestCreateSubtaskRefusesAMissingParent(t *testing.T) {
	b, f := newBackend(t, threeFields)
	_, _, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{
		Type: backend.TypeSubtask, Summary: "Orphan",
	})
	if err == nil || !strings.Contains(err.Error(), "needs a parent") {
		t.Fatalf("err = %v", err)
	}
	for _, w := range f.writes {
		if strings.HasPrefix(w, "POST /rest/api/2/issue ") {
			t.Errorf("nothing may be posted: %s", w)
		}
	}
}

// The plain task level is read off the project too: this instance calls it
// "Todo", and asking for "Task" would have found nothing.
func TestScopeUsesTheProjectsOwnTaskName(t *testing.T) {
	b, f := newBackend(t, twoFields)
	if _, _, err := b.SearchIssuesPage(context.Background(), "TODOP", "", "", []string{backend.TypeTask}, 0, 50); err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(f.searches) == 0 || !strings.Contains(f.searches[len(f.searches)-1], `issuetype in ("Todo")`) {
		t.Errorf("scope = %v", f.searches)
	}
}

// Item 3 of the ticket, first half: the duplicate Parent input put a plain
// string into Extra["parent"], which overwrote the {"key": ...} object and
// Jira answered "parent: data was not an object".
func TestCreateSubtaskKeepsTheParentObjectWhateverExtraSays(t *testing.T) {
	b, f := newBackend(t, threeFields)
	f.createKey = "TKT-9"
	_, _, err := b.CreateIssue(context.Background(), "TKT", backend.IssueDraft{
		Type: backend.TypeSubtask, Summary: "Wire the input", ParentKey: "TKT-7",
		Extra:        map[string]string{"parent": "TKT-7", "customfield_10300": "Given a promo"},
		ScreenFields: []string{"customfield_10300"},
	})
	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	post := f.writes[len(f.writes)-1]
	if !strings.Contains(post, `"parent":{"key":"TKT-7"}`) || strings.Contains(post, `"parent":"TKT-7"`) {
		t.Errorf("parent must stay the object the create set: %s", post)
	}
	if !strings.Contains(post, `"customfield_10300":"Given a promo"`) {
		t.Errorf("an extra on the screen is still sent: %s", post)
	}
}
