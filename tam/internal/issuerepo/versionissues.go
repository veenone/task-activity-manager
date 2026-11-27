package issuerepo

import (
	"context"
	"encoding/json"
	"fmt"

	"agile-suite/tam/internal/backend"
)

// VersionCount is how many of a release's cached issues there are and how
// many of them are finished (backend.Finished).
type VersionCount struct {
	Done  int
	Total int
}

// VersionProgress counts every release's cached issues, by release id. It
// reads the cache only, so issues the profile's scope JQL leaves out are not
// counted; the Releases view says so.
func (r *Repository) VersionProgress(ctx context.Context, profileID string) (map[int]VersionCount, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT status, status_category, fix_versions FROM issue WHERE profile_id = ? AND fix_versions != '[]'`, profileID)
	if err != nil {
		return nil, fmt.Errorf("count release issues: %w", err)
	}
	defer rows.Close()
	out := map[int]VersionCount{}
	for rows.Next() {
		var iss backend.Issue
		var raw string
		if err := rows.Scan(&iss.Status, &iss.StatusCategory, &raw); err != nil {
			return nil, err
		}
		var ids []int
		if json.Unmarshal([]byte(raw), &ids) != nil {
			continue
		}
		done := backend.Finished(iss)
		for _, id := range ids {
			c := out[id]
			c.Total++
			if done {
				c.Done++
			}
			out[id] = c
		}
	}
	return out, rows.Err()
}

// VersionIssues lists the cached issues whose fix versions hold the
// release, in rank order. Never nil.
func (r *Repository) VersionIssues(ctx context.Context, profileID string, id int) ([]backend.Issue, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+issueColumns+` FROM issue
		WHERE profile_id = ? AND EXISTS (SELECT 1 FROM json_each(issue.fix_versions) WHERE json_each.value = ?)
		ORDER BY rank, key`, profileID, id)
	if err != nil {
		return nil, fmt.Errorf("issues in release %d: %w", id, err)
	}
	defer rows.Close()
	out := []backend.Issue{}
	for rows.Next() {
		iss, err := scanIssue(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, iss)
	}
	return out, rows.Err()
}
