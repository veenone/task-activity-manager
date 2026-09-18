package jira_test

import (
	"context"
	"reflect"
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
