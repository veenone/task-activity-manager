// Package shareddb defines the one database both apps open: the workspaces
// (profiles), their connections, and the global key/value settings. A Jira
// connection set up in either app is visible in the other because they read
// the same file.
//
// The tables keep the XTM-era columns (scope_jql, the bug_* fields) with the
// same defaults XTM uses, so an existing XTM database can be copied in row
// for row. Splitting those into an XTM-only extension is a later change.
package shareddb

import (
	"database/sql"
	"os"
	"path/filepath"

	"agile-suite/core/store"
)

// Schema is the shared database layout.
var Schema = store.Schema{
	Version: 2,
	Base: `
CREATE TABLE IF NOT EXISTS profiles (
	id          TEXT PRIMARY KEY,
	name        TEXT NOT NULL,
	jira_url    TEXT NOT NULL,
	project_key TEXT NOT NULL,
	created_at  TEXT NOT NULL,
	scope_jql   TEXT NOT NULL DEFAULT '',
	bug_issue_type TEXT NOT NULL DEFAULT 'Bug',
	bug_project_mode TEXT NOT NULL DEFAULT 'test',
	bug_project_key TEXT NOT NULL DEFAULT '',
	ca_cert TEXT NOT NULL DEFAULT '',
	allow_untrusted_tls INTEGER NOT NULL DEFAULT 0,
	backend TEXT NOT NULL DEFAULT 'xray',
	cross_project_sources TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS connection (
	id                  TEXT PRIMARY KEY,
	workspace_id        TEXT NOT NULL,
	name                TEXT NOT NULL,
	backend             TEXT NOT NULL DEFAULT 'xray',
	url                 TEXT NOT NULL DEFAULT '',
	project_key         TEXT NOT NULL DEFAULT '',
	scope_jql           TEXT NOT NULL DEFAULT '',
	bug_issue_type      TEXT NOT NULL DEFAULT 'Bug',
	bug_project_mode    TEXT NOT NULL DEFAULT 'test',
	bug_project_key     TEXT NOT NULL DEFAULT '',
	ca_cert             TEXT NOT NULL DEFAULT '',
	allow_untrusted_tls INTEGER NOT NULL DEFAULT 0,
	role                TEXT NOT NULL DEFAULT 'both',
	created_at          TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS app_setting (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS confluence_profile (
	profile_id TEXT PRIMARY KEY,
	base_url TEXT NOT NULL DEFAULT '',
	space_key TEXT NOT NULL DEFAULT '',
	root_page_id TEXT NOT NULL DEFAULT '',
	reports_space_key TEXT NOT NULL DEFAULT '',
	reports_root_page_id TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS confluence_association (
	profile_id TEXT NOT NULL,
	board_id INTEGER NOT NULL DEFAULT 0,
	sprint_id INTEGER NOT NULL DEFAULT 0,
	ritual_type TEXT NOT NULL,
	page_id TEXT NOT NULL,
	page_title TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (profile_id, board_id, sprint_id, ritual_type, page_id)
);

CREATE TABLE IF NOT EXISTS confluence_page_cache (
	profile_id TEXT NOT NULL,
	page_id TEXT NOT NULL,
	fetched_at TEXT NOT NULL,
	payload TEXT NOT NULL,
	PRIMARY KEY (profile_id, page_id)
);
`,
	Migrations: []store.Migration{
		{
			// v2: where a profile's sprint reports are published. Both empty
			// is what every profile written before this had, and it means the
			// rituals space and the rituals placement, unchanged.
			Version: 2,
			Apply: func(db *sql.DB) error {
				for _, column := range []string{
					"reports_space_key TEXT NOT NULL DEFAULT ''",
					"reports_root_page_id TEXT NOT NULL DEFAULT ''",
				} {
					if err := store.AddColumnIfMissing(db, "confluence_profile", column); err != nil {
						return err
					}
				}
				return nil
			},
		},
	},
}

// Open opens (or creates) the shared database at path.
func Open(path string) (*store.DB, error) { return store.Open(path, Schema) }

// DefaultPath is where both apps look for the shared database:
// <user config dir>/agile-suite/profiles.db. The directory is created if it
// does not exist.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	suiteDir := filepath.Join(dir, "agile-suite")
	if err := os.MkdirAll(suiteDir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(suiteDir, "profiles.db"), nil
}
