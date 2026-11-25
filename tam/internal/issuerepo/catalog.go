package issuerepo

import (
	"context"
	"encoding/json"
	"fmt"

	"agile-suite/tam/internal/backend"
)

// ReplaceVersions swaps the profile's synced fix versions for vs. Every
// synced row of the profile goes, whatever its project, so a profile moved
// to another project stops listing the old one's releases; drafts stay. An
// empty vs is an answer, the project has none, and empties the synced rows.
func (r *Repository) ReplaceVersions(ctx context.Context, profileID, project string, vs []backend.Version) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_version WHERE profile_id = ? AND draft = 0`, profileID); err != nil {
		return fmt.Errorf("clear the versions of %s: %w", profileID, err)
	}
	for _, v := range vs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO project_version
			(profile_id, project, id, name, description, start_date, release_date, released, archived, draft)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`,
			profileID, project, v.ID, v.Name, v.Description, v.StartDate, v.ReleaseDate, v.Released, v.Archived); err != nil {
			return fmt.Errorf("store version %s for %s: %w", v.Name, profileID, err)
		}
	}
	return tx.Commit()
}

// ReplaceComponents is ReplaceVersions for the project's components.
func (r *Repository) ReplaceComponents(ctx context.Context, profileID, project string, cs []backend.Component) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_component WHERE profile_id = ? AND draft = 0`, profileID); err != nil {
		return fmt.Errorf("clear the components of %s: %w", profileID, err)
	}
	for _, c := range cs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO project_component
			(profile_id, project, id, name, description, lead_name, lead_display_name, assignee_type, draft)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)`,
			profileID, project, c.ID, c.Name, c.Description, c.LeadName, c.LeadDisplayName, c.AssigneeType); err != nil {
			return fmt.Errorf("store component %s for %s: %w", c.Name, profileID, err)
		}
	}
	return tx.Commit()
}

// Versions lists the project's cached fix versions, drafts included, by
// release date with undated ones last, then by name.
func (r *Repository) Versions(ctx context.Context, profileID, project string) ([]backend.Version, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, project, name, description, start_date, release_date, released, archived, draft
		FROM project_version WHERE profile_id = ? AND project = ?
		ORDER BY release_date = '', release_date, name COLLATE NOCASE`, profileID, project)
	if err != nil {
		return nil, fmt.Errorf("read the versions of %s: %w", profileID, err)
	}
	defer rows.Close()
	out := []backend.Version{}
	for rows.Next() {
		var v backend.Version
		if err := rows.Scan(&v.ID, &v.Project, &v.Name, &v.Description, &v.StartDate, &v.ReleaseDate, &v.Released, &v.Archived, &v.Draft); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Components lists the project's cached components, drafts included, by
// name.
func (r *Repository) Components(ctx context.Context, profileID, project string) ([]backend.Component, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, project, name, description, lead_name, lead_display_name, assignee_type, draft
		FROM project_component WHERE profile_id = ? AND project = ?
		ORDER BY name COLLATE NOCASE`, profileID, project)
	if err != nil {
		return nil, fmt.Errorf("read the components of %s: %w", profileID, err)
	}
	defer rows.Close()
	out := []backend.Component{}
	for rows.Next() {
		var c backend.Component
		if err := rows.Scan(&c.ID, &c.Project, &c.Name, &c.Description, &c.LeadName, &c.LeadDisplayName, &c.AssigneeType, &c.Draft); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// encodeCatalogIDs and decodeCatalogIDs carry an issue's components and fix
// versions to and from their JSON columns, never as null.
func encodeCatalogIDs(iss backend.Issue) (components, fixVersions string, err error) {
	c, err := json.Marshal(backend.NonNil(iss.Components))
	if err != nil {
		return "", "", fmt.Errorf("components for %s: %w", iss.Key, err)
	}
	f, err := json.Marshal(backend.NonNil(iss.FixVersions))
	if err != nil {
		return "", "", fmt.Errorf("fix versions for %s: %w", iss.Key, err)
	}
	return string(c), string(f), nil
}

func decodeCatalogIDs(iss *backend.Issue, components, fixVersions string) error {
	if err := json.Unmarshal([]byte(components), &iss.Components); err != nil {
		return fmt.Errorf("components for %s: %w", iss.Key, err)
	}
	if err := json.Unmarshal([]byte(fixVersions), &iss.FixVersions); err != nil {
		return fmt.Errorf("fix versions for %s: %w", iss.Key, err)
	}
	iss.Components, iss.FixVersions = backend.NonNil(iss.Components), backend.NonNil(iss.FixVersions)
	return nil
}
