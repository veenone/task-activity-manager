package issuerepo_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/issuerepo"
)

// sixFieldScreen is the answer a real instance gives for every issue type of
// one project: Story Points and the Epic Link are not on it.
var sixFieldScreen = []string{"summary", "description", "priority", "labels", "assignee"}

func TestEditScreenRoundTripsPerProjectAndType(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	if _, ok, err := r.EditScreen(ctx, "p1", "PLAT", "story"); err != nil || ok {
		t.Fatalf("nothing cached yet = ok %v, %v", ok, err)
	}
	if err := r.PutEditScreen(ctx, "p1", "PLAT", "story", sixFieldScreen); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, ok, err := r.EditScreen(ctx, "p1", "PLAT", "story")
	if err != nil || !ok || !reflect.DeepEqual(got, sixFieldScreen) {
		t.Fatalf("screen = %v (ok %v), %v", got, ok, err)
	}
	// A second read of the same project and type is another type's answer
	// only if the key is wrong, and another profile's only if it leaks.
	if _, ok, _ := r.EditScreen(ctx, "p1", "PLAT", "task"); ok {
		t.Error("a type with no cached screen must not answer with another type's")
	}
	if _, ok, _ := r.EditScreen(ctx, "p2", "PLAT", "story"); ok {
		t.Error("a screen leaked across profiles")
	}

	// A screen read again after an administrator changed it replaces the row
	// rather than adding a second one.
	if err := r.PutEditScreen(ctx, "p1", "PLAT", "story", append(append([]string{}, sixFieldScreen...), "storyPoints")); err != nil {
		t.Fatalf("put again: %v", err)
	}
	got, _, _ = r.EditScreen(ctx, "p1", "PLAT", "story")
	if len(got) != 6 || got[5] != "storyPoints" {
		t.Errorf("rewritten screen = %v", got)
	}
}

func TestPurgeProfileRemovesTheCachedEditScreens(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.PutEditScreen(ctx, "p1", "PLAT", "story", sixFieldScreen); err != nil {
		t.Fatal(err)
	}
	if err := r.PutEditScreen(ctx, "p2", "PLAT", "story", sixFieldScreen); err != nil {
		t.Fatal(err)
	}
	if err := r.PurgeProfile(ctx, "p1"); err != nil {
		t.Fatalf("purge: %v", err)
	}
	if _, ok, _ := r.EditScreen(ctx, "p1", "PLAT", "story"); ok {
		t.Error("the purged profile kept its cached screen")
	}
	if _, ok, _ := r.EditScreen(ctx, "p2", "PLAT", "story"); !ok {
		t.Error("the purge took another profile's cached screen with it")
	}
}

func TestUnpushableEditsNamesTheJournalRowsJiraWillRefuse(t *testing.T) {
	r, db := newRepoWithDB(t)
	ctx := context.Background()
	if err := r.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// PLAT-412 is a story, PLAT-409 a task. Only the story's screen is
	// known, and it does not carry story points.
	if err := r.PutEditScreen(ctx, "p1", "PLAT", "story", sixFieldScreen); err != nil {
		t.Fatal(err)
	}
	for _, e := range []struct{ key, field, val string }{
		{"PLAT-412", "storyPoints", "8"},
		{"PLAT-412", "summary", "Checkout: apply promo codes"},
		{"PLAT-409", "storyPoints", "3"},
	} {
		if err := journal.Upsert(db, "p1", issuerepo.EntityIssue, e.key, e.field, "", e.val, "v1"); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := r.UnpushableEdits(ctx, "p1")
	if err != nil {
		t.Fatalf("unpushable: %v", err)
	}
	// The story's estimate only. Its summary is on the screen, and the
	// task's screen was never read, so nothing can say its estimate is off
	// one: an unread screen is not an empty screen.
	if len(rows) != 1 || rows[0].Key != "PLAT-412" || rows[0].Field != "storyPoints" {
		t.Fatalf("unpushable = %+v", rows)
	}
	if rows[0].ID == 0 {
		t.Errorf("row carries no journal id: %+v", rows[0])
	}

	// Once the field is on the screen, the row is pushable again, and
	// nothing has discarded the user's typed value in the meantime.
	if err := r.PutEditScreen(ctx, "p1", "PLAT", "story", append(append([]string{}, sixFieldScreen...), "storyPoints")); err != nil {
		t.Fatal(err)
	}
	rows, err = r.UnpushableEdits(ctx, "p1")
	if err != nil || len(rows) != 0 {
		t.Fatalf("unpushable after the screen gained the field = %+v, %v", rows, err)
	}
	pending, err := r.ListPendingChanges(ctx, "p1")
	if err != nil || len(pending) != 3 {
		t.Fatalf("the journal must be untouched: %d rows, %v", len(pending), err)
	}
}
