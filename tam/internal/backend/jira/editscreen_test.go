package jira_test

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestEditableFieldsNamesOnlyWhatTheScreenCarries(t *testing.T) {
	b, f := newBackend(t, threeFields)
	f.editMeta = sixFieldScreen
	got, err := b.EditableFields(context.Background(), "PLAT-412")
	if err != nil {
		t.Fatalf("EditableFields: %v", err)
	}
	// Reporter is on that screen and TAM does not offer it, so it is not a
	// name here; Story Points and the Epic Link are TAM's and are not on it.
	want := []string{"assignee", "description", "labels", "priority", "summary"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("editable = %v, want %v", got, want)
	}
}

func TestEditableFieldsNamesTheDiscoveredCustomFields(t *testing.T) {
	b, f := newBackend(t, threeFields)
	f.editMeta = fullScreen
	got, err := b.EditableFields(context.Background(), "PLAT-412")
	if err != nil {
		t.Fatalf("EditableFields: %v", err)
	}
	want := []string{"assignee", "description", "labels", "parentKey", "priority", "storyPoints", "summary"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("editable = %v, want %v", got, want)
	}
}

// A field on the screen that Jira will not let a write set is not editable,
// whatever else it says. The instance behind #52 does not do this today, but
// editmeta is a server response and the operations array is the half of it
// that says what a write may do (I1).
func TestEditableFieldsSkipsAFieldThatTakesNoSet(t *testing.T) {
	b, f := newBackend(t, threeFields)
	f.editMeta = unsettableScreen
	got, err := b.EditableFields(context.Background(), "PLAT-412")
	if err != nil {
		t.Fatalf("EditableFields: %v", err)
	}
	// Story Points takes no operation at all and the Epic Link takes add and
	// remove but not set, so neither is editable. Summary carries no
	// operations array at all, which is an older payload saying nothing
	// rather than saying no, so it stays.
	want := []string{"assignee", "description", "labels", "priority", "summary"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("editable = %v, want %v", got, want)
	}
}

func TestUpdateIssueRefusesAFieldThatTakesNoSet(t *testing.T) {
	b, f := newBackend(t, threeFields)
	f.editMeta = unsettableScreen
	err := b.UpdateIssue(context.Background(), "PLAT-412", map[string]string{"storyPoints": "8"})
	if err == nil {
		t.Fatal("a field Jira will not let a write set must be refused before the request")
	}
	if !strings.Contains(err.Error(), "Story points") {
		t.Errorf("refusal = %q", err)
	}
	for _, w := range f.writes {
		if strings.HasPrefix(w, "PUT ") {
			t.Fatalf("nothing may be sent: %s", w)
		}
	}
}

func TestUpdateIssueRefusesAFieldTheEditScreenDoesNotCarry(t *testing.T) {
	b, f := newBackend(t, threeFields)
	f.editMeta = sixFieldScreen
	err := b.UpdateIssue(context.Background(), "PLAT-412", map[string]string{
		"summary": "New title", "storyPoints": "8",
	})
	if err == nil {
		t.Fatal("an estimate the edit screen does not carry must be refused before the request")
	}
	if !strings.Contains(err.Error(), "Story points") || !strings.Contains(err.Error(), "edit screen") {
		t.Errorf("refusal = %q, want it to name the field and say why", err)
	}
	for _, w := range f.writes {
		if strings.HasPrefix(w, "PUT ") {
			t.Fatalf("nothing may be sent when one field is refused: %s", w)
		}
	}
}

func TestUpdateIssueSendsWhatTheEditScreenCarries(t *testing.T) {
	b, f := newBackend(t, threeFields)
	f.editMeta = fullScreen
	if err := b.UpdateIssue(context.Background(), "PLAT-412", map[string]string{"storyPoints": "8"}); err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	if len(f.writes) != 1 || !strings.Contains(f.writes[0], `"customfield_10016":8`) {
		t.Fatalf("writes = %v", f.writes)
	}
}

func TestUpdateIssueStillSendsWhenTheEditScreenCannotBeRead(t *testing.T) {
	b, f := newBackend(t, threeFields)
	f.editMetaFail = true
	if err := b.UpdateIssue(context.Background(), "PLAT-412", map[string]string{"storyPoints": "8"}); err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	// Nothing read the screen, so nothing may claim the field is off it:
	// Jira's own refusal stays the backstop.
	if len(f.writes) != 1 || !strings.Contains(f.writes[0], `"customfield_10016":8`) {
		t.Fatalf("writes = %v", f.writes)
	}
}

// An id TAM could not work out is not a field Jira lacks, and reporting one
// as the other is this branch's own thesis broken a layer down: the panel
// would disable a control that is on the screen and blame an administrator
// for it.
func TestEditableFieldsKeepsAFieldWhoseIdIsAmbiguous(t *testing.T) {
	// Two fields called Story Points identify neither, so discovery leaves
	// the id empty. That says nothing about the screen.
	b, f := newBackend(t, duplicatePointsFields)
	f.editMeta = sixFieldScreen
	got, err := b.EditableFields(context.Background(), "PLAT-412")
	if err != nil {
		t.Fatalf("EditableFields: %v", err)
	}
	if !slices.Contains(got, "storyPoints") {
		t.Errorf("editable = %v, want storyPoints kept: TAM not knowing the id is not Jira saying the field is off the screen", got)
	}
}

func TestEditableFieldsKeepsAFieldTheInstanceNeverNamed(t *testing.T) {
	// twoFields has no Epic Link at all, so ids.EpicLink is empty and no id
	// can be matched against the screen. The name stays, and the accurate
	// refusal ("this Jira has no Epic Link field") is the one a push gives.
	b, f := newBackend(t, twoFields)
	f.editMeta = sixFieldScreen
	got, err := b.EditableFields(context.Background(), "PLAT-412")
	if err != nil {
		t.Fatalf("EditableFields: %v", err)
	}
	if !slices.Contains(got, "parentKey") {
		t.Errorf("editable = %v, want parentKey kept", got)
	}
	// And the push says the true thing rather than blaming the screen.
	err = b.UpdateIssue(context.Background(), "PLAT-412", map[string]string{"parentKey": "PLAT-350"})
	if err == nil || !strings.Contains(err.Error(), "no Epic Link field") {
		t.Errorf("refusal = %v, want the instance's missing field named", err)
	}
}

// A screen carrying none of TAM's own names is not an instruction to refuse
// every field. Nothing downstream can tell that apart from an unread screen,
// and the three surfaces reading it disagreed about which it was.
func TestUpdateIssueTreatsAnEmptyScreenAsUnknown(t *testing.T) {
	// Both custom ids resolve, so nothing is kept on the "TAM cannot tell"
	// route above and the mapped answer really is empty.
	b, f := newBackend(t, threeFields)
	f.editMeta = `{"fields":{"reporter":{"required":true,"name":"Reporter","operations":["set"],"schema":{"type":"user","system":"reporter"}}}}`
	if err := b.UpdateIssue(context.Background(), "PLAT-412", map[string]string{"summary": "New title"}); err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	if len(f.writes) != 1 || !strings.Contains(f.writes[0], `"summary":"New title"`) {
		t.Fatalf("writes = %v, want the edit sent rather than refused wholesale", f.writes)
	}
}
