package jira_test

import (
	"context"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
)

// twoStoryPoints is the shape a name lookup cannot answer: two custom fields
// both called Story Points. Keying by name picked whichever Jira listed last
// and wrote every estimate to it, with nothing to say it had chosen.
const twoStoryPoints = `[{"id":"customfield_10020","name":"Sprint","custom":true},
	{"id":"customfield_10016","name":"Story Points","custom":true},
	{"id":"customfield_10253","name":"Story Points","custom":true},
	{"id":"customfield_10014","name":"Epic Link","custom":true}]`

// With a name that means two fields, TAM has nothing to go on. Picking one
// would be a silent wrong write; dropping the value would lose a number the
// user typed. It says so instead, and the journal row stays pending because
// the edit failed rather than succeeded against the wrong field.
func TestPointsRefusedWhenTheNameMeansTwoFields(t *testing.T) {
	b, f := newBackend(t, twoStoryPoints)
	err := b.UpdateIssue(context.Background(), "PLAT-412", map[string]string{"storyPoints": "8"})
	if err == nil {
		t.Fatal("want an error rather than a guess")
	}
	want := "TAM cannot tell which field PLAT estimates story points in: customfield_10016 and customfield_10253 " +
		"are both called Story Points. Ask a Jira administrator which field this project uses, or set the points in Jira."
	if err.Error() != want {
		t.Errorf("err = %q, want %q", err.Error(), want)
	}
	for _, w := range f.writes {
		if strings.Contains(w, "customfield_10253") || strings.Contains(w, "customfield_10016") {
			t.Errorf("nothing may be written to a field TAM guessed: %s", w)
		}
	}
}

// The same refusal on the create path. This used to drop the estimate
// without a word: CreateIssue only wrote points when the discovered id was
// non-empty, and said nothing when it was not.
func TestCreateRefusesRatherThanDropPointsItCannotPlace(t *testing.T) {
	b, f := newBackend(t, twoStoryPoints)
	f.createKey = "PLAT-701"
	_, _, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{
		Type: backend.TypeBug, Summary: "Promo input", StoryPoints: pts(5),
	})
	if err == nil {
		t.Fatal("want an error rather than a silently dropped estimate")
	}
	if !strings.Contains(err.Error(), "cannot tell which field PLAT estimates story points in") {
		t.Errorf("err = %v", err)
	}
	for _, w := range f.writes {
		if strings.HasPrefix(w, "POST /rest/api/2/issue ") {
			t.Errorf("nothing is created with the estimate missing: %s", w)
		}
	}
}

// An unresolvable points field only matters to a write that carries one. A
// draft with no estimate is not held up by it.
func TestCreateWithoutPointsIgnoresTheAmbiguity(t *testing.T) {
	b, f := newBackend(t, twoStoryPoints)
	f.createKey = "PLAT-702"
	if _, _, err := b.CreateIssue(context.Background(), "PLAT", backend.IssueDraft{Type: backend.TypeBug, Summary: "No estimate"}); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	if !strings.Contains(f.writes[len(f.writes)-1], `"summary":"No estimate"`) {
		t.Errorf("the create still goes: %s", f.writes[len(f.writes)-1])
	}
}

// An edit that never mentions points is not held up by an unresolvable
// points field either.
func TestEditWithoutPointsIgnoresTheAmbiguity(t *testing.T) {
	b, f := newBackend(t, twoStoryPoints)
	if err := b.UpdateIssue(context.Background(), "PLAT-412", map[string]string{"summary": "New title"}); err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	if !strings.Contains(f.writes[len(f.writes)-1], `"summary":"New title"`) {
		t.Errorf("the edit still goes: %s", f.writes[len(f.writes)-1])
	}
}
