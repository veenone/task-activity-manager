package issuerepo_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
)

// projectTypes is what a sync records for a project that has types of its
// own beside the ones TAM models: Improvement is the one in issue #137,
// and the sub-task level is called something else again.
func projectTypes() []backend.IssueType {
	return []backend.IssueType{
		{ID: "1", Name: "Task", Logical: backend.TypeTask},
		{ID: "4", Name: "Improvement", Logical: ""},
		{ID: "5", Name: "Technical task", Subtask: true, Logical: backend.TypeSubtask},
	}
}

// The dialog offers every type the project has, so the store has to take
// them. Before this, Improvement was offered and then refused on save.
func TestADraftMayBeAnyTypeTheProjectOffers(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.PutProjectTypes(ctx, "p1", projectTypes()); err != nil {
		t.Fatalf("record the types: %v", err)
	}
	key, err := r.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: "Improvement", Summary: "Trim the checkout step"})
	if err != nil {
		t.Fatalf("draft an Improvement: %v", err)
	}
	iss, err := r.GetIssue(ctx, "p1", key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if iss.Type != "Improvement" {
		t.Errorf("type = %q, want the project's own name kept on the row", iss.Type)
	}
}

// A sub-task keeps the logical type rather than the project's own name
// for that level. The dialog never offers the instance's sub-task type as
// something to draft at the top level (offeredTypes drops it), and the
// parent rule reads the logical type to tell a sub-task's parent from an
// epic link, so "subtask" is what a draft of one carries whatever the
// instance calls it.
func TestASubtaskIsDraftedUnderTheLogicalType(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.PutProjectTypes(ctx, "p1", projectTypes()); err != nil {
		t.Fatalf("record the types: %v", err)
	}
	if err := r.UpsertPage(ctx, "p1", sample()[:1], time.Now(), false); err != nil {
		t.Fatalf("seed the parent: %v", err)
	}
	if _, err := r.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{
		Type: backend.TypeSubtask, Summary: "Wire the promo input", ParentKey: "PLAT-412",
	}); err != nil {
		t.Errorf("draft a sub-task: %v", err)
	}
}

// A type nobody offers is still refused, and the refusal says what this
// project has rather than only what TAM models.
func TestATypeTheProjectDoesNotOfferIsRefused(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.PutProjectTypes(ctx, "p1", projectTypes()); err != nil {
		t.Fatalf("record the types: %v", err)
	}
	_, err := r.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: "Spike", Summary: "Try the other gateway"})
	if err == nil {
		t.Fatal("a type the project does not have was accepted")
	}
	if !strings.Contains(err.Error(), "Improvement") {
		t.Errorf("refusal = %q, want it to name what the project does offer", err)
	}
}

// A profile that has never synced has no list to check against, and the
// six logical types stay the answer, which is what the dialog offers
// there too.
func TestWithNoSyncedTypesTheSixLogicalOnesStillDraft(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if _, err := r.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeStory, Summary: "Guest checkout"}); err != nil {
		t.Errorf("draft a story on an unsynced profile: %v", err)
	}
	if _, err := r.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: "Improvement", Summary: "Trim it"}); err == nil {
		t.Error("a type nothing has recorded was accepted on an unsynced profile")
	}
}

// The types belong to the profile that synced them. Another profile's
// list says nothing about this one's project.
func TestAnotherProfilesTypesDoNotMakeADraftValid(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.PutProjectTypes(ctx, "p2", projectTypes()); err != nil {
		t.Fatalf("record the types: %v", err)
	}
	if _, err := r.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: "Improvement", Summary: "Trim it"}); err == nil {
		t.Error("a type recorded for another profile was accepted")
	}
}

// The importer drafts through the same function, so it takes the same
// types the dialog offers rather than a narrower set.
func TestTheImporterTakesTheProjectsOwnTypes(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.PutProjectTypes(ctx, "p1", projectTypes()); err != nil {
		t.Fatalf("record the types: %v", err)
	}
	keys, err := r.CreateDrafts(ctx, "p1", "PLAT", []backend.IssueDraft{
		{Type: "Improvement", Summary: "One"},
		{Type: backend.TypeTask, Summary: "Two"},
	}, "backlog.xlsx")
	if err != nil {
		t.Fatalf("import two drafts: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("keys = %v, want one per row", keys)
	}
}
