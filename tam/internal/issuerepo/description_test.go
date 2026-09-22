package issuerepo_test

import (
	"context"
	"testing"
	"time"

	"agile-suite/tam/internal/issuerepo"
)

func text(s string) *string { return &s }

// A description the sync read comes back off the row, with no second call.
func TestSyncKeepsTheDescriptionOnTheRow(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	page := sample()
	page[0].Description = text("Apply the code at the payment step.")
	if err := r.UpsertPage(ctx, "p1", page, time.Now(), false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	iss, err := r.GetIssue(ctx, "p1", "PLAT-412")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if iss.Description == nil {
		t.Fatal("description is nil after a sync that carried one")
	}
	if *iss.Description != "Apply the code at the payment step." {
		t.Errorf("description = %q", *iss.Description)
	}
}

// The trap this change exists to avoid: an issue nobody has synced a
// description for and an issue whose description is genuinely blank must not
// arrive as the same value. The first is nil, the second is a pointer to "".
func TestANeverSyncedDescriptionIsNotAnEmptyOne(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	page := sample()[:2]
	page[0].Description = text("")
	page[1].Description = nil
	if err := r.UpsertPage(ctx, "p1", page, time.Now(), false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	blank, err := r.GetIssue(ctx, "p1", page[0].Key)
	if err != nil {
		t.Fatalf("get %s: %v", page[0].Key, err)
	}
	if blank.Description == nil || *blank.Description != "" {
		t.Errorf("an issue Jira says has no description read back as %v, want a pointer to the empty string", blank.Description)
	}
	unknown, err := r.GetIssue(ctx, "p1", page[1].Key)
	if err != nil {
		t.Fatalf("get %s: %v", page[1].Key, err)
	}
	if unknown.Description != nil {
		t.Errorf("an issue whose description was never read back as %q, want nil", *unknown.Description)
	}
}

// A row refreshed by something that did not read the description (the
// post-commit row refresh, an importer write) must not turn a description
// the store already holds back into "never synced".
func TestAnUpsertWithoutADescriptionKeepsTheOneOnTheRow(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	page := sample()[:1]
	page[0].Description = text("The original text.")
	if err := r.UpsertPage(ctx, "p1", page, time.Now(), false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	refreshed := sample()[:1]
	refreshed[0].Description = nil
	refreshed[0].Summary = "Checkout: apply promo code at payment step"
	if err := r.UpsertPage(ctx, "p1", refreshed, time.Now(), false); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	iss, err := r.GetIssue(ctx, "p1", "PLAT-412")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if iss.Summary != "Checkout: apply promo code at payment step" {
		t.Errorf("summary = %q, the refresh should have landed", iss.Summary)
	}
	if iss.Description == nil || *iss.Description != "The original text." {
		t.Errorf("description = %v, want the text the store already held", iss.Description)
	}
}

// Local first: an edit that has not been committed wins over whatever the
// next sync brings back, the same way every other edited column does.
func TestAPendingDescriptionEditSurvivesASync(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	page := sample()[:1]
	page[0].Description = text("What Jira had.")
	if err := r.UpsertPage(ctx, "p1", page, time.Now(), false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := r.EditField(ctx, "p1", "PLAT-412", "description", "What I typed."); err != nil {
		t.Fatalf("edit: %v", err)
	}
	later := sample()[:1]
	later[0].Description = text("What Jira has now.")
	if err := r.UpsertPage(ctx, "p1", later, time.Now().Add(time.Minute), true); err != nil {
		t.Fatalf("sync: %v", err)
	}
	iss, err := r.GetIssue(ctx, "p1", "PLAT-412")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if iss.Description == nil || *iss.Description != "What I typed." {
		t.Errorf("description = %v, want the uncommitted local edit", iss.Description)
	}
	if !iss.Pending {
		t.Error("the row is not marked pending, so the journal row went missing with it")
	}
}

// The journal has to record the description the row actually held, not "",
// or a Commit would report a conflict against a value nobody ever saw.
func TestADescriptionEditJournalsTheSyncedTextAsItsBase(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	page := sample()[:1]
	page[0].Description = text("What Jira had.")
	if err := r.UpsertPage(ctx, "p1", page, time.Now(), false); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := r.EditField(ctx, "p1", "PLAT-412", "description", "What I typed."); err != nil {
		t.Fatalf("edit: %v", err)
	}
	rows, err := r.PendingForKey(ctx, "p1", "PLAT-412")
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	var found bool
	for _, p := range rows {
		if p.EntityType == issuerepo.EntityIssue && p.Field == "description" {
			found = true
			if p.BeforeVal != "What Jira had." {
				t.Errorf("journalled base = %q, want the synced text", p.BeforeVal)
			}
			if p.AfterVal != "What I typed." {
				t.Errorf("journalled value = %q", p.AfterVal)
			}
		}
	}
	if !found {
		t.Fatal("no description row in the journal")
	}
}
