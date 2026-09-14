package ritualrepo_test

import (
	"context"
	"path/filepath"
	"testing"

	"agile-suite/core/store"
	"agile-suite/tam/internal/ritualrepo"
	"agile-suite/tam/internal/tamstore"
)

func TestDemoSeedAfterRepairingVersionTenWithoutIssuesColumn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`ALTER TABLE ritual_document DROP COLUMN issues_json`,
		`UPDATE meta SET value = '10' WHERE key = 'schema_version'`,
		`INSERT INTO ritual_document (profile_id, board_id, sprint_id, ritual_type, title, remark, body, issue_keys_json, confluence_page_id, confluence_version, status)
		 VALUES ('demo', 1, 12, 'planning', 'My plan', 'Keep this remark', '<p>My notes</p>', '["DEMO-412","DEMO-409"]', '1234', 3, 'published')`,
	} {
		if _, err := db.DB().Exec(stmt); err != nil {
			_ = db.Close()
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	// Repeat startup: repairs and demo seeding must both be idempotent.
	for pass := 0; pass < 2; pass++ {
		db, err := tamstore.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		r := ritualrepo.New(db.DB())
		if err := r.SeedDemo(context.Background(), "demo", "DEMO"); err != nil {
			_ = db.Close()
			t.Fatal(err)
		}
		draft, err := r.Get(context.Background(), "demo", 1, 12, "planning")
		if err != nil {
			_ = db.Close()
			t.Fatal(err)
		}
		if draft.Title != "My plan" || draft.Remark != "Keep this remark" || draft.Body != "<p>My notes</p>" || draft.ConfluencePageID != "1234" || draft.ConfluenceVersion != 3 || draft.Status != "published" {
			t.Errorf("repair or seed changed the saved draft: %+v", draft)
		}
		if draft.IssuesJSON != `[{"key":"DEMO-412","remark":""},{"key":"DEMO-409","remark":""}]` {
			t.Errorf("lost issue selections: %s", draft.IssuesJSON)
		}
		if version, err := store.ReadSchemaVersion(db.DB()); err != nil || version != tamstore.Schema.Version {
			t.Errorf("schema = %d, error = %v, want %d", version, err, tamstore.Schema.Version)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRepairPreservesHealthyVersionTenIssueSelections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tam.db")
	db, err := tamstore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`UPDATE meta SET value = '10' WHERE key = 'schema_version'`,
		`INSERT INTO ritual_document (profile_id, board_id, sprint_id, ritual_type, issue_keys_json, issues_json) VALUES
		 ('p1', 1, 12, 'planning', '["OLD-1"]', '[]'),
		 ('p1', 1, 12, 'review', '["OLD-1"]', '[{"key":"NEW-2","remark":"Team decision"}]')`,
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
	defer db.Close()
	for typ, want := range map[string]string{"planning": "[]", "review": `[{"key":"NEW-2","remark":"Team decision"}]`} {
		draft, err := ritualrepo.New(db.DB()).Get(context.Background(), "p1", 1, 12, typ)
		if err != nil || draft.IssuesJSON != want {
			t.Errorf("%s selection = %s, error = %v; want %s", typ, draft.IssuesJSON, err, want)
		}
	}
}
