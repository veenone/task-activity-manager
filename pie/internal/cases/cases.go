// Package cases keeps the issues an analyst has shortlisted for analysis. A
// case is PIE's own record: Jira has no home for it, so pie.db holds it, and
// it outlives the run it was opened from. Its table is in saved's schema.
package cases

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// Case is one shortlisted issue. State is "open" until Phase 3 adds more.
type Case struct {
	ID       int64  `json:"id"`
	Key      string `json:"key"`
	State    string `json:"state"`
	Notes    string `json:"notes"`
	OpenedAt string `json:"openedAt"`
}

// issueKey is a Jira key: a project key of capitals, digits and underscores,
// a dash, and a number.
var issueKey = regexp.MustCompile(`^[A-Z][A-Z0-9_]*-[0-9]+$`)

// Store reads and writes cases.
type Store struct{ db *sql.DB }

// New returns a Store on a database opened with saved.Schema.
func New(db *sql.DB) *Store { return &Store{db: db} }

// List returns a profile's cases by key.
func (s *Store) List(profileID string) ([]Case, error) {
	rows, err := s.db.Query(`SELECT id, source_key, state, notes, opened_at FROM case_item WHERE profile_id = ? ORDER BY source_key`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Case{}
	for rows.Next() {
		var c Case
		if err := rows.Scan(&c.ID, &c.Key, &c.State, &c.Notes, &c.OpenedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Open opens a case for each key. An issue that already has a case keeps it,
// with the date it was first opened. Every key is checked before any is
// stored, because the keys come from the frontend.
func (s *Store) Open(profileID string, keys []string, at time.Time) error {
	if len(keys) == 0 {
		return errors.New("choose at least one issue to open as a case")
	}
	for _, k := range keys {
		if !issueKey.MatchString(k) {
			return fmt.Errorf("%q is not a Jira issue key", k)
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	when := at.UTC().Format(time.RFC3339)
	for _, k := range keys {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO case_item (profile_id, source_key, opened_at) VALUES (?, ?, ?)`, profileID, k, when); err != nil {
			return err
		}
	}
	return tx.Commit()
}
