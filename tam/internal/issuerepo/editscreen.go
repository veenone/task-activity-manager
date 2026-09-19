package issuerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"
)

// UnpushableEdit is one journalled edit whose field is not on the edit screen
// Jira last reported for the issue's project and type. Commit would be
// refused for it, so the surfaces that show pending work say so first. It is
// never a reason to discard the row: the value is what the user typed.
type UnpushableEdit struct {
	ID    int64  `json:"id"`
	Key   string `json:"key"`
	Field string `json:"field"`
}

// PutEditScreen records the fields an issue of this project and type may be
// edited with, by TAM's own names. Writing it again replaces the row, so a
// screen an administrator has changed is the one that answers next.
func (r *Repository) PutEditScreen(ctx context.Context, profileID, project, issueType string, fields []string) error {
	if fields == nil {
		fields = []string{}
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("encode the edit screen of %s %s: %w", project, issueType, err)
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO edit_screen (profile_id, project, issue_type, fields_json, cached_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(profile_id, project, issue_type) DO UPDATE SET fields_json = excluded.fields_json, cached_at = excluded.cached_at`,
		profileID, project, issueType, string(encoded), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("cache the edit screen of %s %s: %w", project, issueType, err)
	}
	return nil
}

// EditScreen reads back what PutEditScreen stored. The second result is
// false when nothing has been cached for this project and type, which is
// what a profile that has never synced looks like: the caller falls back to
// TAM's own fixed list rather than treating an unknown screen as an empty
// one.
func (r *Repository) EditScreen(ctx context.Context, profileID, project, issueType string) ([]string, bool, error) {
	var raw string
	err := r.db.QueryRowContext(ctx,
		`SELECT fields_json FROM edit_screen WHERE profile_id = ? AND project = ? AND issue_type = ?`,
		profileID, project, issueType).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read the edit screen of %s %s: %w", project, issueType, err)
	}
	var fields []string
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		// A row nothing can parse is treated as no row at all, so a bad
		// cache costs the fallback list and never the user's edits.
		return nil, false, nil
	}
	return fields, true, nil
}

// UnpushableEdits lists the profile's journalled issue edits whose field is
// missing from the cached edit screen of the issue's project and type.
//
// Only issues whose screen has been read are considered. A screen nobody has
// read says nothing about what is on it, and guessing here would put a
// warning on an edit that pushes perfectly well.
func (r *Repository) UnpushableEdits(ctx context.Context, profileID string) ([]UnpushableEdit, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT p.id, p.entity_key, p.field, s.fields_json
		   FROM pending_change p
		   JOIN issue i ON i.profile_id = p.profile_id AND i.key = p.entity_key
		   JOIN edit_screen s ON s.profile_id = p.profile_id AND s.project = i.project AND s.issue_type = i.type
		  WHERE p.profile_id = ? AND p.entity_type = ?
		  ORDER BY p.id`,
		profileID, EntityIssue)
	if err != nil {
		return nil, fmt.Errorf("read the unpushable edits: %w", err)
	}
	defer rows.Close()
	out := []UnpushableEdit{}
	for rows.Next() {
		var e UnpushableEdit
		var raw string
		if err := rows.Scan(&e.ID, &e.Key, &e.Field, &raw); err != nil {
			return nil, err
		}
		var fields []string
		if err := json.Unmarshal([]byte(raw), &fields); err != nil {
			continue
		}
		if !slices.Contains(fields, e.Field) {
			out = append(out, e)
		}
	}
	return out, rows.Err()
}
