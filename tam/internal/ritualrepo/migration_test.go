package ritualrepo_test

import (
	"path/filepath"
	"testing"

	"agile-suite/core/store"
	"agile-suite/tam/internal/tamstore"
)

func TestRepairingVersionTenWithoutIssuesColumnKeepsTheRow(t *testing.T) {
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
	// Repeat startup: the repair must be idempotent.
	for pass := 0; pass < 2; pass++ {
		db, err := tamstore.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		var title, remark, body, pageID, issues string
		var version int
		if err := db.DB().QueryRow(`SELECT title, remark, body, confluence_page_id, confluence_version, issues_json
			FROM ritual_document WHERE profile_id = 'demo' AND sprint_id = 12 AND ritual_type = 'planning'`).
			Scan(&title, &remark, &body, &pageID, &version, &issues); err != nil {
			_ = db.Close()
			t.Fatal(err)
		}
		if title != "My plan" || remark != "Keep this remark" || body != "<p>My notes</p>" || pageID != "1234" || version != 3 {
			t.Errorf("repair changed the saved row: %s %s %s %s %d", title, remark, body, pageID, version)
		}
		if issues != `[{"key":"DEMO-412","remark":""},{"key":"DEMO-409","remark":""}]` {
			t.Errorf("lost issue selections: %s", issues)
		}
		if version, err := store.ReadSchemaVersion(db.DB()); err != nil || version != tamstore.Schema.Version {
			t.Errorf("schema = %d, error = %v", version, err)
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
		var issues string
		if err := db.DB().QueryRow(`SELECT issues_json FROM ritual_document
			WHERE profile_id = 'p1' AND board_id = 1 AND sprint_id = 12 AND ritual_type = ?`, typ).
			Scan(&issues); err != nil || issues != want {
			t.Errorf("%s selection = %s, error = %v; want %s", typ, issues, err, want)
		}
	}
}
