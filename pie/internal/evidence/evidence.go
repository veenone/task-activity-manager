// Package evidence keeps what an analyst weighs before setting a verdict for
// a project on a case. Find suggests it from Jira; the analyst accepts or
// dismisses each suggestion and adds notes and issues by hand. Nothing here
// touches a verdict: that is the decision the design rests on, and a test
// pins it. The table is in saved's schema.
package evidence

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"agile-suite/pie/internal/cases"
)

// Kinds of evidence. Link is strong; the name and text matches are weak,
// because a shared name or phrase may be a coincidence. Note and Issue are
// added by hand.
const (
	Link      = "link"
	Component = "component"
	Version   = "version"
	Text      = "text"
	Note      = "note"
	Issue     = "issue"
)

// States. A suggestion starts Suggested; evidence added by hand is Accepted.
const (
	Suggested = "suggested"
	Accepted  = "accepted"
	Dismissed = "dismissed"
)

var kindOrder = []string{Link, Component, Version, Text, Note, Issue}

// Item is one piece of evidence for one project on a case. Ref is the issue
// key, the component or version name, or the note's text.
type Item struct {
	ID      int64  `json:"id"`
	Project string `json:"project"`
	Kind    string `json:"kind"`
	Ref     string `json:"ref"`
	Detail  string `json:"detail"`
	State   string `json:"state"`
	Weak    bool   `json:"weak"`
}

func byHand(kind string) bool { return kind == Note || kind == Issue }

// Store reads and writes evidence.
type Store struct{ db *sql.DB }

// New returns a Store on a database opened with saved.Schema.
func New(db *sql.DB) *Store { return &Store{db: db} }

// List returns the case's evidence by project, strongest kind first.
func (s *Store) List(profileID string, caseID int64) ([]Item, error) {
	rows, err := s.db.Query(`SELECT id, project_key, kind, ref, detail, state FROM evidence
		WHERE profile_id = ? AND case_id = ? ORDER BY project_key, ref`, profileID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.Project, &it.Kind, &it.Ref, &it.Detail, &it.State); err != nil {
			return nil, err
		}
		it.Weak = it.Kind == Component || it.Kind == Version || it.Kind == Text
		out = append(out, it)
	}
	slices.SortStableFunc(out, func(a, b Item) int {
		if c := strings.Compare(a.Project, b.Project); c != 0 {
			return c
		}
		return slices.Index(kindOrder, a.Kind) - slices.Index(kindOrder, b.Kind)
	})
	return out, rows.Err()
}

// SetState accepts or dismisses a suggestion, or puts it back to suggested.
// Evidence added by hand is removed rather than dismissed.
func (s *Store) SetState(profileID string, caseID, id int64, state string) error {
	if state != Suggested && state != Accepted && state != Dismissed {
		return fmt.Errorf("%q is not an evidence state", state)
	}
	r, err := s.db.Exec(`UPDATE evidence SET state = ? WHERE profile_id = ? AND case_id = ? AND id = ? AND kind NOT IN (?, ?)`,
		state, profileID, caseID, id, Note, Issue)
	return oneRow(r, err, errors.New("that suggestion no longer exists"))
}

// AddNote records a note for a project on the case.
func (s *Store) AddNote(profileID string, caseID int64, project, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("a note needs some text")
	}
	return s.addByHand(profileID, caseID, project, Note, text, text)
}

// AddIssue records an issue the analyst found for a project on the case.
func (s *Store) AddIssue(profileID string, caseID int64, project, key string) error {
	key = strings.ToUpper(strings.TrimSpace(key))
	if !cases.IsIssueKey(key) {
		return fmt.Errorf("%q is not a Jira issue key", key)
	}
	return s.addByHand(profileID, caseID, project, Issue, key, "")
}

func (s *Store) addByHand(profileID string, caseID int64, project, kind, ref, detail string) error {
	if !cases.IsProjectKey(project) {
		return fmt.Errorf("%q is not a Jira project key", project)
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM case_item WHERE profile_id = ? AND id = ?`, profileID, caseID).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return errors.New("that case no longer exists")
	}
	_, err := s.db.Exec(`INSERT OR IGNORE INTO evidence (profile_id, case_id, project_key, kind, ref, detail, state)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, profileID, caseID, project, kind, ref, detail, Accepted)
	return err
}

// Remove deletes evidence added by hand. A suggestion is dismissed instead,
// so finding evidence again does not bring it back.
func (s *Store) Remove(profileID string, caseID, id int64) error {
	r, err := s.db.Exec(`DELETE FROM evidence WHERE profile_id = ? AND case_id = ? AND id = ? AND kind IN (?, ?)`, profileID, caseID, id, Note, Issue)
	return oneRow(r, err, errors.New("only evidence added by hand can be removed; dismiss a suggestion instead"))
}

// suggest stores new suggestions and returns how many were new. One that is
// already stored keeps the state the analyst gave it.
func (s *Store) suggest(profileID string, caseID int64, items []Item) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	added := 0
	for _, it := range items {
		r, err := tx.Exec(`INSERT OR IGNORE INTO evidence (profile_id, case_id, project_key, kind, ref, detail, state)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, profileID, caseID, it.Project, it.Kind, it.Ref, it.Detail, Suggested)
		if err != nil {
			return 0, err
		}
		if n, _ := r.RowsAffected(); n > 0 {
			added++
		}
	}
	return added, tx.Commit()
}

func oneRow(r sql.Result, err, missing error) error {
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return missing
	}
	return nil
}
