package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// screenBackend answers EditableFields with a fixed screen, or with an error
// once offline is set, which is what an unreachable instance looks like from
// here.
type screenBackend struct {
	stubIssueBackend
	fields  []string
	offline bool
	calls   int
}

func (b *screenBackend) EditableFields(context.Context, string) ([]string, error) {
	b.calls++
	if b.offline {
		return nil, errors.New("dial tcp: no route to host")
	}
	return b.fields, nil
}

func seedStory(t *testing.T, a *App, profileID string) {
	t.Helper()
	err := a.repo.UpsertPage(a.ctx, profileID, []backend.Issue{{
		Key: "PLAT-412", ID: "1", Project: "PLAT", Type: "story", Summary: "Checkout: apply promo code",
		Updated: "2026-09-05T09:58:00Z",
	}}, time.Now(), false)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestGetEditableFieldsCachesTheAnswerAndServesItOffline(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	seedStory(t, a, p.ID)
	b := &screenBackend{fields: []string{"assignee", "description", "labels", "priority", "summary"}}
	a.backends[p.ID] = b

	got, err := a.GetEditableFields(p.ID, "PLAT-412")
	if err != nil {
		t.Fatalf("GetEditableFields: %v", err)
	}
	if !reflect.DeepEqual(got, b.fields) {
		t.Fatalf("fields = %v", got)
	}

	// Jira has gone away. The answer it gave is still the answer.
	b.offline = true
	got, err = a.GetEditableFields(p.ID, "PLAT-412")
	if err != nil {
		t.Fatalf("offline: %v", err)
	}
	if !reflect.DeepEqual(got, b.fields) {
		t.Fatalf("offline fields = %v, want the cached answer", got)
	}
}

func TestGetEditableFieldsAnswersUnknownWhenNothingWasEverCached(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	seedStory(t, a, p.ID)
	a.backends[p.ID] = &screenBackend{offline: true}

	// A profile that has never reached Jira has no screen to draw from, and
	// an empty list is how the panel is told to fall back to its own fixed
	// one rather than refusing to edit anything.
	got, err := a.GetEditableFields(p.ID, "PLAT-412")
	if err != nil {
		t.Fatalf("GetEditableFields: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("fields = %v, want nothing known", got)
	}
}

func TestGetEditableFieldsNeverAsksJiraAboutADraft(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	b := &screenBackend{fields: []string{"summary"}}
	a.backends[p.ID] = b
	key, err := a.CreateIssue(p.ID, backend.IssueDraft{Type: backend.TypeStory, Summary: "Draft"})
	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}

	got, err := a.GetEditableFields(p.ID, key)
	if err != nil || len(got) != 0 {
		t.Fatalf("draft = %v, %v", got, err)
	}
	if b.calls != 0 {
		t.Errorf("a draft has no edit screen in Jira, so nothing may be asked: %d calls", b.calls)
	}
}

func TestListUnpushableEditsNamesTheRowAndKeepsIt(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	seedStory(t, a, p.ID)
	a.backends[p.ID] = &screenBackend{fields: []string{"summary", "description", "priority", "labels", "assignee"}}
	if _, err := a.GetEditableFields(p.ID, "PLAT-412"); err != nil {
		t.Fatalf("read the screen: %v", err)
	}
	if err := a.EditIssue(p.ID, "PLAT-412", "storyPoints", "8"); err != nil {
		t.Fatalf("EditIssue: %v", err)
	}

	rows, err := a.ListUnpushableEdits(p.ID)
	if err != nil {
		t.Fatalf("ListUnpushableEdits: %v", err)
	}
	// Naming it is not discarding it: the value stays in the journal until
	// the user says otherwise, and the row named is the journal row itself.
	pending, err := a.ListPendingChanges(p.ID)
	if err != nil || len(pending) != 1 || pending[0].AfterVal != "8" {
		t.Fatalf("journal = %+v, %v", pending, err)
	}
	want := []issuerepo.UnpushableEdit{{ID: pending[0].ID, Key: "PLAT-412", Field: "storyPoints"}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %+v, want %+v", rows, want)
	}
}

// An empty answer is one of the ways of not knowing, so it must not become
// the cached truth: cached, UnpushableEdits would then read every pending
// edit on that project and type as doomed.
func TestGetEditableFieldsNeverCachesAnEmptyAnswer(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	seedStory(t, a, p.ID)
	good := []string{"summary", "description", "priority", "labels", "assignee"}
	b := &screenBackend{fields: good}
	a.backends[p.ID] = b
	if _, err := a.GetEditableFields(p.ID, "PLAT-412"); err != nil {
		t.Fatalf("first read: %v", err)
	}

	// The instance now answers with nothing TAM edits. The good answer it
	// gave a moment ago is the one that stands.
	b.fields = []string{}
	got, err := a.GetEditableFields(p.ID, "PLAT-412")
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	if !reflect.DeepEqual(got, good) {
		t.Fatalf("fields = %v, want the cached answer rather than the empty one", got)
	}
	cached, ok, err := a.repo.EditScreen(a.ctx, p.ID, "PLAT", "story")
	if err != nil || !ok || !reflect.DeepEqual(cached, good) {
		t.Fatalf("cache = %v (ok %v), %v: an empty answer overwrote a real one", cached, ok, err)
	}
}
