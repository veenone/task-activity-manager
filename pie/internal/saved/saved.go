// Package saved keeps an analyst's named queries and the last run of each, so
// PIE opens with the rows it showed last time without asking Jira. It also
// keeps each profile's settings, and holds PIE's whole schema and purge list,
// case_item included: one file is what the purge gate reads.
package saved

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"agile-suite/core/shareddb"
	"agile-suite/core/store"
	"agile-suite/pie/internal/query"
)

// ponytail: a run is stored whole as JSON. Cases will need issues in rows of
// their own; split the table then, not before.
const tables = `
CREATE TABLE IF NOT EXISTS saved_query (
	id         INTEGER PRIMARY KEY,
	profile_id TEXT NOT NULL,
	name       TEXT NOT NULL,
	jql        TEXT NOT NULL,
	run_at     TEXT NOT NULL DEFAULT '',
	result     TEXT NOT NULL DEFAULT '',
	UNIQUE (profile_id, name)
);
CREATE TABLE IF NOT EXISTS profile_setting (
	profile_id    TEXT PRIMARY KEY,
	product_field TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS profile_field (
	profile_id TEXT NOT NULL,
	role       TEXT NOT NULL,
	name       TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (profile_id, role)
);
INSERT OR IGNORE INTO profile_field (profile_id, role, name)
	SELECT profile_id, 'product', product_field FROM profile_setting WHERE product_field != '';
CREATE TABLE IF NOT EXISTS case_item (
	id         INTEGER PRIMARY KEY,
	profile_id TEXT NOT NULL,
	source_key TEXT NOT NULL,
	state      TEXT NOT NULL DEFAULT 'open',
	notes      TEXT NOT NULL DEFAULT '',
	opened_at  TEXT NOT NULL,
	UNIQUE (profile_id, source_key)
);
CREATE TABLE IF NOT EXISTS case_project (
	profile_id  TEXT NOT NULL,
	case_id     INTEGER NOT NULL,
	project_key TEXT NOT NULL,
	verdict     TEXT NOT NULL DEFAULT 'not assessed',
	reason      TEXT NOT NULL DEFAULT '',
	assessed_at TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (profile_id, case_id, project_key)
);
CREATE TABLE IF NOT EXISTS project_group (
	id         INTEGER PRIMARY KEY,
	profile_id TEXT NOT NULL,
	name       TEXT NOT NULL,
	UNIQUE (profile_id, name)
);
CREATE TABLE IF NOT EXISTS evidence (
	id          INTEGER PRIMARY KEY,
	profile_id  TEXT NOT NULL,
	case_id     INTEGER NOT NULL,
	project_key TEXT NOT NULL,
	kind        TEXT NOT NULL,
	ref         TEXT NOT NULL,
	detail      TEXT NOT NULL DEFAULT '',
	state       TEXT NOT NULL,
	UNIQUE (profile_id, case_id, project_key, kind, ref)
);
CREATE TABLE IF NOT EXISTS project_group_member (
	profile_id  TEXT NOT NULL,
	group_id    INTEGER NOT NULL,
	project_key TEXT NOT NULL,
	PRIMARY KEY (profile_id, group_id, project_key)
);
CREATE TABLE IF NOT EXISTS product_baseline (
	profile_id   TEXT NOT NULL,
	project_key  TEXT NOT NULL,
	baseline_key TEXT NOT NULL,
	PRIMARY KEY (profile_id, project_key)
);
CREATE TABLE IF NOT EXISTS report_cycle (
	id           INTEGER PRIMARY KEY,
	profile_id   TEXT NOT NULL,
	name         TEXT NOT NULL,
	taken_at     TEXT NOT NULL,
	cases        TEXT NOT NULL,
	published_at TEXT NOT NULL DEFAULT '',
	UNIQUE (profile_id, name)
);
CREATE TABLE IF NOT EXISTS report_page (
	profile_id TEXT NOT NULL,
	cycle_id   INTEGER NOT NULL,
	page_id    TEXT NOT NULL,
	version    INTEGER NOT NULL,
	PRIMARY KEY (profile_id, cycle_id)
);
`

// purged lists every table Purge empties for a profile. The instruction gate
// checks that each profile-keyed table above is named here. profile_setting
// held the product base name before profile_field; the copy above reads it
// on every open, so it stays until a migration can drop it.
var purged = []string{
	"saved_query", "profile_setting", "profile_field",
	"case_item", "case_project", "project_group", "project_group_member", "evidence",
	"product_baseline", "report_cycle", "report_page",
}

// Schema is PIE's database: core's profile tables plus PIE's own. The tables
// are created if missing, so a pie.db from before this opens unchanged.
func Schema() store.Schema {
	s := shareddb.Schema
	s.Base += tables
	return s
}

// Query is a saved query. RunAt is when it last ran, or "" if it never has.
type Query struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	JQL   string `json:"jql"`
	RunAt string `json:"runAt"`
}

// Run is one run of a query. Added and Dropped are the matched keys that
// differ from the run before it, and are nil when there was none.
type Run struct {
	Result  query.Result `json:"result"`
	RunAt   string       `json:"runAt"`
	Added   []string     `json:"added"`
	Dropped []string     `json:"dropped"`
}

// Store reads and writes saved queries.
type Store struct{ db *sql.DB }

// New returns a Store on a database opened with Schema.
func New(db *sql.DB) *Store { return &Store{db: db} }

// List returns a profile's saved queries by name.
func (s *Store) List(profileID string) ([]Query, error) {
	rows, err := s.db.Query(`SELECT id, name, jql, run_at FROM saved_query WHERE profile_id = ? ORDER BY name`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Query{}
	for rows.Next() {
		var q Query
		if err := rows.Scan(&q.ID, &q.Name, &q.JQL, &q.RunAt); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// Save adds a query under a name the profile has not used yet.
func (s *Store) Save(profileID, name, jql string) (Query, error) {
	name, jql = strings.TrimSpace(name), strings.TrimSpace(jql)
	if name == "" {
		return Query{}, errors.New("a saved query needs a name")
	}
	if jql == "" {
		return Query{}, errors.New("enter a JQL query to save")
	}
	r, err := s.db.Exec(`INSERT INTO saved_query (profile_id, name, jql) VALUES (?, ?, ?)`, profileID, name, jql)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Query{}, fmt.Errorf("a saved query named %q already exists", name)
		}
		return Query{}, err
	}
	id, err := r.LastInsertId()
	return Query{ID: id, Name: name, JQL: jql}, err
}

// Delete removes one of the profile's queries and its run.
func (s *Store) Delete(profileID string, id int64) error {
	r, err := s.db.Exec(`DELETE FROM saved_query WHERE profile_id = ? AND id = ?`, profileID, id)
	return oneRow(r, err)
}

// Last returns the query's stored run, with an empty RunAt if it never ran.
func (s *Store) Last(profileID string, id int64) (Run, error) {
	var run Run
	var raw string
	err := s.db.QueryRow(`SELECT run_at, result FROM saved_query WHERE profile_id = ? AND id = ?`, profileID, id).
		Scan(&run.RunAt, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return Run{}, errors.New("that saved query no longer exists")
	}
	if err != nil || raw == "" {
		return run, err
	}
	if err := json.Unmarshal([]byte(raw), &run.Result); err != nil {
		return Run{}, fmt.Errorf("read the stored run: %w", err)
	}
	return run, nil
}

// Record stores res as the query's run, with the JQL that produced it, and
// returns it compared with the run it replaces. Runs of different JQL are not
// compared: their difference says nothing about what changed in Jira.
func (s *Store) Record(profileID string, id int64, jql string, res query.Result, at time.Time) (Run, error) {
	before, err := s.Last(profileID, id)
	if err != nil {
		return Run{}, err
	}
	var beforeJQL string
	if err := s.db.QueryRow(`SELECT jql FROM saved_query WHERE profile_id = ? AND id = ?`, profileID, id).Scan(&beforeJQL); err != nil {
		return Run{}, err
	}
	jql = strings.TrimSpace(jql)
	raw, err := json.Marshal(res)
	if err != nil {
		return Run{}, err
	}
	run := Run{Result: res, RunAt: at.UTC().Format(time.RFC3339)}
	r, err := s.db.Exec(`UPDATE saved_query SET jql = ?, run_at = ?, result = ? WHERE profile_id = ? AND id = ?`,
		jql, run.RunAt, string(raw), profileID, id)
	if err := oneRow(r, err); err != nil {
		return Run{}, err
	}
	if before.RunAt != "" && beforeJQL == jql {
		run.Added, run.Dropped = missing(before.Result, res), missing(res, before.Result)
	}
	return run, nil
}

// Fields are the names of the Jira custom fields a profile reads. "" is a
// field the profile does not use.
type Fields struct {
	ProductBase      string `json:"productBase"`
	ClientVisibility string `json:"clientVisibility"`
	ClientImpact     string `json:"clientImpact"`
}

// roles names each field as profile_field stores it.
func (f *Fields) roles() map[string]*string {
	return map[string]*string{"product": &f.ProductBase, "visibility": &f.ClientVisibility, "impact": &f.ClientImpact}
}

// Fields returns the profile's field names.
func (s *Store) Fields(profileID string) (Fields, error) {
	var f Fields
	rows, err := s.db.Query(`SELECT role, name FROM profile_field WHERE profile_id = ?`, profileID)
	if err != nil {
		return f, err
	}
	defer rows.Close()
	into := f.roles()
	for rows.Next() {
		var role, name string
		if err := rows.Scan(&role, &name); err != nil {
			return Fields{}, err
		}
		if p := into[role]; p != nil {
			*p = name
		}
	}
	return f, rows.Err()
}

// SetFields stores all three names at once. A cleared name is stored as "",
// not deleted, so the carry-over in tables never brings an old one back.
// Checking the names against Jira is the caller's job.
func (s *Store) SetFields(profileID string, f Fields) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for role, name := range f.roles() {
		if _, err := tx.Exec(`INSERT INTO profile_field (profile_id, role, name) VALUES (?, ?, ?)
			ON CONFLICT (profile_id, role) DO UPDATE SET name = excluded.name`,
			profileID, role, strings.TrimSpace(*name)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Purge removes everything the profile owns in PIE's tables.
func (s *Store) Purge(profileID string) error {
	for _, table := range purged {
		if _, err := s.db.Exec(`DELETE FROM `+table+` WHERE profile_id = ?`, profileID); err != nil {
			return fmt.Errorf("purge %s: %w", table, err)
		}
	}
	return nil
}

// missing lists the keys res matched that other did not.
func missing(other, res query.Result) []string {
	had := map[string]bool{}
	for _, is := range other.Issues {
		had[is.Key] = is.Matched
	}
	out := []string{}
	for _, is := range res.Issues {
		if is.Matched && !had[is.Key] {
			out = append(out, is.Key)
		}
	}
	return out
}

func oneRow(r sql.Result, err error) error {
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return errors.New("that saved query no longer exists")
	}
	return nil
}
