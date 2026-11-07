package cases

import (
	"database/sql"
	"errors"
	"fmt"
)

// Baseline is a Jira project the analyst marks as a product baseline, with the
// projects derived from it. The mapping is kept by hand, per profile; it is
// separate from the Product base field on issues.
type Baseline struct {
	Key         string   `json:"key"`
	Descendants []string `json:"descendants"`
}

// In product_baseline a baseline is the row whose project is itself, and a
// descendant is a row pointing at one. The key on (profile, project) is what
// keeps a project under one baseline at most.

// Baselines returns the profile's baselines by key, each with its descendants
// by key.
func (s *Store) Baselines(profileID string) ([]Baseline, error) {
	rows, err := s.db.Query(`SELECT baseline_key, project_key FROM product_baseline
		WHERE profile_id = ? ORDER BY baseline_key, project_key <> baseline_key, project_key`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Baseline{}
	for rows.Next() {
		var base, key string
		if err := rows.Scan(&base, &key); err != nil {
			return nil, err
		}
		if key == base {
			out = append(out, Baseline{Key: base, Descendants: []string{}})
		} else {
			out[len(out)-1].Descendants = append(out[len(out)-1].Descendants, key)
		}
	}
	return out, rows.Err()
}

// AddBaseline marks a project as a baseline.
func (s *Store) AddBaseline(profileID, key string) error {
	if err := s.free(profileID, key, key); err != nil {
		return err
	}
	_, err := s.db.Exec(`INSERT INTO product_baseline (profile_id, project_key, baseline_key) VALUES (?, ?, ?)`, profileID, key, key)
	return err
}

// AddDescendants puts projects under a baseline. One already under it stays.
func (s *Store) AddDescendants(profileID, baseline string, keys []string) error {
	var one int
	err := s.db.QueryRow(`SELECT 1 FROM product_baseline WHERE profile_id = ? AND project_key = ? AND baseline_key = ?`,
		profileID, baseline, baseline).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s is not a baseline", baseline)
	}
	if err != nil {
		return err
	}
	for _, k := range keys {
		if err := s.free(profileID, k, baseline); err != nil {
			return err
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, k := range keys {
		// The baseline may have gone since the check above; a descendant row
		// without it would have nothing to sit under.
		r, err := tx.Exec(`INSERT OR IGNORE INTO product_baseline (profile_id, project_key, baseline_key)
			SELECT ?, ?, ? WHERE EXISTS (SELECT 1 FROM product_baseline WHERE profile_id = ? AND project_key = ? AND baseline_key = ?)`,
			profileID, k, baseline, profileID, baseline, baseline)
		if err != nil {
			return err
		}
		if n, _ := r.RowsAffected(); n == 0 {
			var one int
			if tx.QueryRow(`SELECT 1 FROM product_baseline WHERE profile_id = ? AND project_key = ?`, profileID, baseline).Scan(&one) != nil {
				return fmt.Errorf("%s is not a baseline", baseline)
			}
		}
	}
	return tx.Commit()
}

// RemoveBaseline drops a baseline and its descendants from the mapping.
func (s *Store) RemoveBaseline(profileID, key string) error {
	_, err := s.db.Exec(`DELETE FROM product_baseline WHERE profile_id = ? AND baseline_key = ?`, profileID, key)
	return err
}

// RemoveDescendant takes a project out from under its baseline.
func (s *Store) RemoveDescendant(profileID, key string) error {
	_, err := s.db.Exec(`DELETE FROM product_baseline WHERE profile_id = ? AND project_key = ? AND baseline_key <> project_key`,
		profileID, key)
	return err
}

// free says why key cannot go under baseline (itself, for a new baseline):
// a project sits in the mapping once. Already under that baseline is fine.
func (s *Store) free(profileID, key, baseline string) error {
	if !projectKey.MatchString(key) {
		return fmt.Errorf("%q is not a Jira project key", key)
	}
	var base string
	err := s.db.QueryRow(`SELECT baseline_key FROM product_baseline WHERE profile_id = ? AND project_key = ?`, profileID, key).Scan(&base)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil
	case err != nil:
		return err
	case base == key:
		return fmt.Errorf("%s is already a baseline", key)
	case base == baseline && key != baseline:
		return nil
	default:
		return fmt.Errorf("%s is already a descendant of %s", key, base)
	}
}
