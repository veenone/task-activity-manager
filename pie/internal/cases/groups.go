package cases

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Group is a named list of projects an analyst assesses together, kept per
// profile so a case can take all of them at once.
type Group struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	Projects []string `json:"projects"`
}

// Groups returns the profile's groups by name, each with its projects by key.
func (s *Store) Groups(profileID string) ([]Group, error) {
	rows, err := s.db.Query(`SELECT g.id, g.name, m.project_key FROM project_group g
		JOIN project_group_member m ON m.profile_id = g.profile_id AND m.group_id = g.id
		WHERE g.profile_id = ? ORDER BY g.name, m.project_key`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Group{}
	for rows.Next() {
		var id int64
		var name, key string
		if err := rows.Scan(&id, &name, &key); err != nil {
			return nil, err
		}
		if len(out) == 0 || out[len(out)-1].ID != id {
			out = append(out, Group{ID: id, Name: name})
		}
		out[len(out)-1].Projects = append(out[len(out)-1].Projects, key)
	}
	return out, rows.Err()
}

// SaveGroup stores a new group under a name the profile has not used.
func (s *Store) SaveGroup(profileID, name string, keys []string) (Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Group{}, errors.New("a project group needs a name")
	}
	if len(keys) == 0 {
		return Group{}, errors.New("a project group needs at least one project")
	}
	for _, k := range keys {
		if !projectKey.MatchString(k) {
			return Group{}, fmt.Errorf("%q is not a Jira project key", k)
		}
	}
	keys = slices.Compact(slices.Sorted(slices.Values(keys)))
	tx, err := s.db.Begin()
	if err != nil {
		return Group{}, err
	}
	defer func() { _ = tx.Rollback() }()
	r, err := tx.Exec(`INSERT INTO project_group (profile_id, name) VALUES (?, ?)`, profileID, name)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Group{}, fmt.Errorf("a project group named %q already exists", name)
		}
		return Group{}, err
	}
	id, err := r.LastInsertId()
	if err != nil {
		return Group{}, err
	}
	for _, k := range keys {
		if _, err := tx.Exec(`INSERT INTO project_group_member (profile_id, group_id, project_key) VALUES (?, ?, ?)`, profileID, id, k); err != nil {
			return Group{}, err
		}
	}
	return Group{ID: id, Name: name, Projects: keys}, tx.Commit()
}

// DeleteGroup removes one of the profile's groups. Cases keep their projects.
func (s *Store) DeleteGroup(profileID string, id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	r, err := tx.Exec(`DELETE FROM project_group WHERE profile_id = ? AND id = ?`, profileID, id)
	if err := oneRow(r, err, errors.New("that project group no longer exists")); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM project_group_member WHERE profile_id = ? AND group_id = ?`, profileID, id); err != nil {
		return err
	}
	return tx.Commit()
}
