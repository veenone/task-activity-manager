// Package saved keeps an analyst's named queries and the last run of each, so
// PIE opens with the rows it showed last time without asking Jira.
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
`

// purged lists every table Purge empties for a profile. The instruction gate
// checks that each profile-keyed table above is named here.
var purged = []string{"saved_query"}

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
