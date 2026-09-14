package ritualrepo_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"agile-suite/core/store"
	"agile-suite/tam/internal/ritualrepo"
	"agile-suite/tam/internal/tamstore"
)

func newRepo(t *testing.T) *ritualrepo.Repository {
	t.Helper()
	db, err := tamstore.Open(filepath.Join(t.TempDir(), "tam.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	return ritualrepo.New(db.DB())
}

func sampleDraft() ritualrepo.Draft {
	return ritualrepo.Draft{
		ProfileID:         "p1",
		BoardID:           7,
		SprintID:          12,
		RitualType:        "review",
		Title:             "Sprint 12 review",
		Remark:            "Bring customer feedback",
		Body:              "## Delivered\n\n- Checkout recovery",
		IssuesJSON:        `["PLAT-41","PLAT-52"]`,
		ConfluencePageID:  "92814",
		ConfluenceVersion: 3,
		Status:            "published",
		UpdatedAt:         "2026-09-13T09:15:00Z",
		PublishedAt:       "2026-09-13T09:16:00Z",
	}
}

func TestGetReturnsAnEmptyDraftWhenNothingIsStored(t *testing.T) {
	got, err := newRepo(t).Get(context.Background(), "p1", 7, 12, "review")
	if err != nil {
		t.Fatalf("get empty draft: %v", err)
	}
	if got != (ritualrepo.Draft{}) {
		t.Errorf("empty lookup = %+v, want zero Draft", got)
	}
}

func TestUpsertRoundTripsAndReplacesTheSameDraft(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	want := sampleDraft()
	if err := r.Upsert(ctx, want); err != nil {
		t.Fatalf("upsert draft: %v", err)
	}

	want.Title = "Sprint 12 review: revised"
	want.Body = "## Delivered\n\n- Checkout recovery\n- Invoice export"
	want.IssuesJSON = `["PLAT-41","PLAT-52","PLAT-60"]`
	want.ConfluenceVersion = 4
	want.UpdatedAt = "2026-09-13T10:30:00Z"
	if err := r.Upsert(ctx, want); err != nil {
		t.Fatalf("replace draft: %v", err)
	}

	got, err := r.Get(ctx, want.ProfileID, want.BoardID, want.SprintID, want.RitualType)
	if err != nil {
		t.Fatalf("get draft: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestDraftsAreIsolatedByEveryCompositeKeyField(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	base := sampleDraft()
	variants := []ritualrepo.Draft{
		base,
		base,
		base,
		base,
	}
	variants[0].ProfileID, variants[0].Title = "p2", "other profile"
	variants[1].BoardID, variants[1].Title = 8, "other board"
	variants[2].SprintID, variants[2].Title = 13, "other sprint"
	variants[3].RitualType, variants[3].Title = "retro", "other ritual"

	if err := r.Upsert(ctx, base); err != nil {
		t.Fatalf("upsert base: %v", err)
	}
	for _, draft := range variants {
		if err := r.Upsert(ctx, draft); err != nil {
			t.Fatalf("upsert %q: %v", draft.Title, err)
		}
	}

	for _, want := range append([]ritualrepo.Draft{base}, variants...) {
		got, err := r.Get(ctx, want.ProfileID, want.BoardID, want.SprintID, want.RitualType)
		if err != nil {
			t.Fatalf("get %q: %v", want.Title, err)
		}
		if got.Title != want.Title {
			t.Errorf("key (%q, %d, %d, %q) returned title %q, want %q",
				want.ProfileID, want.BoardID, want.SprintID, want.RitualType, got.Title, want.Title)
		}
	}
}

// TestGetUpsertAndDeleteNormalizeTheRitualTypeCase guards the bug that was
// found and fixed once already at the app layer: ritual_type carries no
// COLLATE NOCASE, so a caller passing "Review" against a stored "review" row
// would otherwise match nothing on a raw-case lookup, and a write that
// followed such a miss would then blank a real row's publication fields.
// Normalization now lives in the repository's key-taking methods themselves,
// so every caller gets the same answer regardless of case without having to
// normalize first.
func TestGetUpsertAndDeleteNormalizeTheRitualTypeCase(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()

	stored := sampleDraft()
	stored.RitualType = "review"
	if err := r.Upsert(ctx, stored); err != nil {
		t.Fatalf("upsert lowercase: %v", err)
	}

	got, err := r.Get(ctx, stored.ProfileID, stored.BoardID, stored.SprintID, "Review")
	if err != nil {
		t.Fatalf("get mixed case: %v", err)
	}
	if got.Title != stored.Title || got.ConfluencePageID != stored.ConfluencePageID {
		t.Fatalf("mixed-case get = %+v, want the lowercase row %+v", got, stored)
	}

	upserted := stored
	upserted.RitualType = "REVIEW"
	upserted.Title = "updated via upper case"
	if err := r.Upsert(ctx, upserted); err != nil {
		t.Fatalf("upsert upper case: %v", err)
	}
	afterUpsert, err := r.Get(ctx, stored.ProfileID, stored.BoardID, stored.SprintID, "review")
	if err != nil {
		t.Fatalf("get after mixed-case upsert: %v", err)
	}
	if afterUpsert.Title != "updated via upper case" {
		t.Fatalf("upper-case upsert did not update the lowercase row, got %+v", afterUpsert)
	}

	if err := r.Delete(ctx, stored.ProfileID, stored.BoardID, stored.SprintID, "Review"); err != nil {
		t.Fatalf("delete mixed case: %v", err)
	}
	if after, err := r.Get(ctx, stored.ProfileID, stored.BoardID, stored.SprintID, "review"); err != nil || after != (ritualrepo.Draft{}) {
		t.Fatalf("after mixed-case delete = %+v, err = %v; want zero Draft and no error", after, err)
	}
}

func TestDeleteRemovesOnlyTheSelectedDraft(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	selected := sampleDraft()
	neighbors := []ritualrepo.Draft{selected, selected, selected, selected}
	neighbors[0].ProfileID = "p2"
	neighbors[1].BoardID = 8
	neighbors[2].SprintID = 13
	neighbors[3].RitualType = "retro"
	for _, draft := range append([]ritualrepo.Draft{selected}, neighbors...) {
		if err := r.Upsert(ctx, draft); err != nil {
			t.Fatalf("upsert %q: %v", draft.Title, err)
		}
	}

	if err := r.Delete(ctx, selected.ProfileID, selected.BoardID, selected.SprintID, selected.RitualType); err != nil {
		t.Fatalf("delete selected draft: %v", err)
	}
	if got, err := r.Get(ctx, selected.ProfileID, selected.BoardID, selected.SprintID, selected.RitualType); err != nil || got != (ritualrepo.Draft{}) {
		t.Fatalf("deleted draft = %+v, err = %v; want zero Draft and no error", got, err)
	}
	for _, neighbor := range neighbors {
		got, err := r.Get(ctx, neighbor.ProfileID, neighbor.BoardID, neighbor.SprintID, neighbor.RitualType)
		if err != nil {
			t.Fatalf("get neighboring draft: %v", err)
		}
		if !reflect.DeepEqual(got, neighbor) {
			t.Errorf("neighbor after delete = %+v, want %+v", got, neighbor)
		}
	}
	if err := r.Delete(ctx, selected.ProfileID, selected.BoardID, selected.SprintID, selected.RitualType); err != nil {
		t.Fatalf("delete absent draft: %v", err)
	}
}

func TestVersionEightDatabaseGainsRitualDocumentsOnOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`DROP TABLE ritual_document`,
		`UPDATE meta SET value = '8' WHERE key = 'schema_version'`,
		`INSERT INTO profile_setting (profile_id, key, value) VALUES ('p1', 'existing', 'retained')`,
	} {
		if _, err := db.DB().Exec(stmt); err != nil {
			_ = db.Close()
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = tamstore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if version, err := store.ReadSchemaVersion(db.DB()); err != nil || version != tamstore.Schema.Version {
		t.Fatalf("schema version = %d, err = %v; want %d", version, err, tamstore.Schema.Version)
	}
	var existing string
	if err := db.DB().QueryRow(`SELECT value FROM profile_setting WHERE profile_id = 'p1' AND key = 'existing'`).Scan(&existing); err != nil || existing != "retained" {
		t.Fatalf("existing data = %q, err = %v", existing, err)
	}
	r := ritualrepo.New(db.DB())
	want := sampleDraft()
	if err := r.Upsert(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	got, err := r.Get(context.Background(), want.ProfileID, want.BoardID, want.SprintID, want.RitualType)
	if err != nil || got != want {
		t.Fatalf("upgraded draft = %+v, err = %v; want %+v", got, err, want)
	}
}

func TestSeedDemoProvidesTwoSprintCyclesAndPreservesEdits(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.SeedDemo(ctx, "demo-profile", "DEMO"); err != nil {
		t.Fatal(err)
	}
	for _, sprint := range []int{12, 13} {
		for _, typ := range []string{"planning", "standup", "review", "retro"} {
			d, err := r.Get(ctx, "demo-profile", 1, sprint, typ)
			if err != nil || d.ProfileID == "" {
				t.Fatalf("missing demo %s sprint %d: %+v (err=%v)", typ, sprint, d, err)
			}
		}
	}
	draft, err := r.Get(ctx, "demo-profile", 1, 13, "retro")
	if err != nil {
		t.Fatal(err)
	}
	if draft.Status != "draft" || draft.Remark == "" || draft.IssuesJSON == "[]" {
		t.Fatalf("demo draft lacks preview metadata: %+v", draft)
	}
	draft.Title = "My edited retrospective"
	if err := r.Upsert(ctx, draft); err != nil {
		t.Fatal(err)
	}
	if err := r.SeedDemo(ctx, "demo-profile", "DEMO"); err != nil {
		t.Fatal(err)
	}
	got, err := r.Get(ctx, "demo-profile", 1, 13, "retro")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "My edited retrospective" {
		t.Fatalf("seed overwrote edit with %q", got.Title)
	}
}

func TestIssuesRoundTripThroughADraft(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)

	issues := []ritualrepo.Issue{
		{Key: "PLAT-14", Remark: "demoed, docs follow-up"},
		{Key: "PLAT-22", Remark: "blocked on infra"},
	}
	encoded, err := ritualrepo.EncodeIssues(issues)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := r.Upsert(ctx, ritualrepo.Draft{
		ProfileID: "p1", BoardID: 1, SprintID: 12, RitualType: "review",
		IssuesJSON: encoded, Status: "draft",
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := r.Get(ctx, "p1", 1, 12, "review")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	back, err := ritualrepo.DecodeIssues(got.IssuesJSON)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(back) != 2 || back[0].Key != "PLAT-14" || back[1].Remark != "blocked on infra" {
		t.Fatalf("issues = %#v, want the two seeded in order", back)
	}
}

func TestListDraftsReturnsOneSprintsRitualsInTypeOrder(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)

	for _, ritualType := range []string{"review", "planning", "standup"} {
		if err := r.Upsert(ctx, ritualrepo.Draft{
			ProfileID: "p1", BoardID: 1, SprintID: 12, RitualType: ritualType, Status: "draft",
		}); err != nil {
			t.Fatalf("upsert %s: %v", ritualType, err)
		}
	}
	// A different sprint, which must not appear.
	if err := r.Upsert(ctx, ritualrepo.Draft{
		ProfileID: "p1", BoardID: 1, SprintID: 13, RitualType: "planning", Status: "draft",
	}); err != nil {
		t.Fatalf("upsert other sprint: %v", err)
	}

	drafts, err := r.ListDrafts(ctx, "p1", 1, 12)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(drafts) != 3 {
		t.Fatalf("got %d drafts, want 3", len(drafts))
	}
	if drafts[0].RitualType != "planning" || drafts[2].RitualType != "standup" {
		t.Fatalf("order = %s, %s, %s; want planning, review, standup",
			drafts[0].RitualType, drafts[1].RitualType, drafts[2].RitualType)
	}
}
