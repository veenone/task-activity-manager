package cases

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// The verdicts an analyst can record for one project on one case. PIE
// suggests evidence and never sets one of these itself.
const (
	NotAssessed   = "not assessed"
	Investigating = "investigating"
	Affected      = "affected"
	NotAffected   = "not affected"
)

var verdicts = []string{NotAssessed, Investigating, Affected, NotAffected}

var projectKey = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// Project is one project on a case and the analyst's verdict for it.
// AssessedAt is when the verdict was last set, "" if never.
type Project struct {
	Key        string `json:"key"`
	Verdict    string `json:"verdict"`
	Reason     string `json:"reason"`
	AssessedAt string `json:"assessedAt"`
}

// Detail is a case with its projects, by key.
type Detail struct {
	Case
	Projects []Project `json:"projects"`
}

var errNoCase = errors.New("that case no longer exists")

// Get returns one of the profile's cases with its projects.
func (s *Store) Get(profileID string, id int64) (Detail, error) {
	var d Detail
	err := s.db.QueryRow(`SELECT id, source_key, state, notes, opened_at FROM case_item WHERE profile_id = ? AND id = ?`, profileID, id).
		Scan(&d.ID, &d.Key, &d.State, &d.Notes, &d.OpenedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Detail{}, errNoCase
	}
	if err != nil {
		return Detail{}, err
	}
	rows, err := s.db.Query(`SELECT project_key, verdict, reason, assessed_at FROM case_project
		WHERE profile_id = ? AND case_id = ? ORDER BY project_key`, profileID, id)
	if err != nil {
		return Detail{}, err
	}
	defer rows.Close()
	d.Projects = []Project{}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.Key, &p.Verdict, &p.Reason, &p.AssessedAt); err != nil {
			return Detail{}, err
		}
		if p.Verdict != NotAssessed {
			d.Assessed++
		}
		if p.Verdict == Affected {
			d.Affected++
		}
		d.Projects = append(d.Projects, p)
	}
	return d, rows.Err()
}

// SetNotes replaces the case's notes.
func (s *Store) SetNotes(profileID string, id int64, notes string) error {
	r, err := s.db.Exec(`UPDATE case_item SET notes = ? WHERE profile_id = ? AND id = ?`, notes, profileID, id)
	return oneRow(r, err, errNoCase)
}

// AddProjects puts projects on the case at not assessed. A project already
// on it keeps its verdict.
func (s *Store) AddProjects(profileID string, id int64, keys []string) error {
	for _, k := range keys {
		if !projectKey.MatchString(k) {
			return fmt.Errorf("%q is not a Jira project key", k)
		}
	}
	if err := s.exists(profileID, id); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, k := range keys {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO case_project (profile_id, case_id, project_key) VALUES (?, ?, ?)`, profileID, id, k); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// RemoveProject takes a project and its verdict off the case.
func (s *Store) RemoveProject(profileID string, id int64, key string) error {
	r, err := s.db.Exec(`DELETE FROM case_project WHERE profile_id = ? AND case_id = ? AND project_key = ?`, profileID, id, key)
	return oneRow(r, err, errNotOnCase)
}

var errNotOnCase = errors.New("that project is not on the case")

// SetVerdict records the analyst's verdict for a project already on the case.
func (s *Store) SetVerdict(profileID string, id int64, key, verdict, reason string, at time.Time) error {
	if !slices.Contains(verdicts, verdict) {
		return fmt.Errorf("%q is not a verdict", verdict)
	}
	r, err := s.db.Exec(`UPDATE case_project SET verdict = ?, reason = ?, assessed_at = ?
		WHERE profile_id = ? AND case_id = ? AND project_key = ?`,
		verdict, strings.TrimSpace(reason), at.UTC().Format(time.RFC3339), profileID, id, key)
	return oneRow(r, err, errNotOnCase)
}

func (s *Store) exists(profileID string, id int64) error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM case_item WHERE profile_id = ? AND id = ?`, profileID, id).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return errNoCase
	}
	return nil
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
