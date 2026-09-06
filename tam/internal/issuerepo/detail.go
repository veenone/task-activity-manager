package issuerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"agile-suite/tam/internal/backend"
)

// LinkedTest is a test reached from an issue through the requirement link
// type, for the detail panel's Tests tab.
type LinkedTest struct {
	Key      string `json:"key"`
	Summary  string `json:"summary"`
	LinkType string `json:"linkType"`
}

// ReadDetail returns the cached detail for key, when it was fetched, and
// whether a cached detail exists. A cached row with no detail yet reports
// ok=false and no error.
func (r *Repository) ReadDetail(ctx context.Context, profileID, key string) (backend.IssueDetail, time.Time, bool, error) {
	var (
		raw       sql.NullString
		fetchedAt sql.NullString
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT detail_json, detail_fetched_at FROM issue WHERE profile_id = ? AND key = ?`, profileID, key,
	).Scan(&raw, &fetchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return backend.IssueDetail{}, time.Time{}, false, ErrNotFound
	}
	if err != nil {
		return backend.IssueDetail{}, time.Time{}, false, err
	}
	if !raw.Valid || raw.String == "" {
		return backend.IssueDetail{}, time.Time{}, false, nil
	}
	var d backend.IssueDetail
	if err := json.Unmarshal([]byte(raw.String), &d); err != nil {
		return backend.IssueDetail{}, time.Time{}, false, fmt.Errorf("decode detail for %s: %w", key, err)
	}
	at, err := time.Parse(time.RFC3339, fetchedAt.String)
	if err != nil {
		return backend.IssueDetail{}, time.Time{}, false, fmt.Errorf("detail_fetched_at for %s: %w", key, err)
	}
	links, err := r.ListLinks(ctx, profileID, key)
	if err != nil {
		return backend.IssueDetail{}, time.Time{}, false, err
	}
	d.Key = key
	d.Links = links
	return d, at, true, nil
}

// WriteDetail caches d for key and replaces the issue's links, in one
// transaction. The links are stored in issue_link rather than inside the
// JSON so the Tests tab and later phases can query them.
func (r *Repository) WriteDetail(ctx context.Context, profileID, key string, d backend.IssueDetail, fetchedAt time.Time) error {
	stored := d
	stored.Key = key
	stored.Links = nil
	raw, err := json.Marshal(stored)
	if err != nil {
		return fmt.Errorf("encode detail for %s: %w", key, err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`UPDATE issue SET detail_json = ?, detail_fetched_at = ? WHERE profile_id = ? AND key = ?`,
		string(raw), fetchedAt.UTC().Format(time.RFC3339), profileID, key)
	if err != nil {
		return fmt.Errorf("write detail for %s: %w", key, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM issue_link WHERE profile_id = ? AND from_key = ?`, profileID, key); err != nil {
		return fmt.Errorf("clear links for %s: %w", key, err)
	}
	for _, l := range d.Links {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO issue_link (profile_id, from_key, to_key, link_type, direction, to_summary, to_type) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			profileID, key, l.Key, l.Type, l.Direction, l.Summary, l.IssueType); err != nil {
			return fmt.Errorf("write link %s -> %s: %w", key, l.Key, err)
		}
	}
	if err := reapplyPending(ctx, tx, profileID, map[string]bool{key: true}); err != nil {
		return err
	}
	return tx.Commit()
}

// ListLinks returns the cached links of key, ordered by type, then direction,
// then the other key.
func (r *Repository) ListLinks(ctx context.Context, profileID, key string) ([]backend.Link, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT direction, link_type, to_key, to_summary, to_type FROM issue_link
		 WHERE profile_id = ? AND from_key = ? ORDER BY link_type, direction, to_key`, profileID, key)
	if err != nil {
		return nil, fmt.Errorf("list links for %s: %w", key, err)
	}
	defer rows.Close()
	links := []backend.Link{}
	for rows.Next() {
		var l backend.Link
		if err := rows.Scan(&l.Direction, &l.Type, &l.Key, &l.Summary, &l.IssueType); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// ListLinkedTests returns the tests linked to key through linkType, compared
// case-insensitively, in either direction. When linkType is empty the suite
// has not been told which link type it uses, so anything whose name contains
// "test" counts: Jira instances name it "Tested By", "Tests", "Test Case
// Linkage" and more, and a guess that matches too much beats an exact guess
// that matches nothing.
func (r *Repository) ListLinkedTests(ctx context.Context, profileID, key, linkType string) ([]LinkedTest, error) {
	want := strings.TrimSpace(linkType)
	query := `SELECT to_key, to_summary, link_type FROM issue_link
		 WHERE profile_id = ? AND from_key = ? AND lower(link_type) = lower(?) ORDER BY to_key`
	arg := want
	if want == "" {
		query = `SELECT to_key, to_summary, link_type FROM issue_link
		 WHERE profile_id = ? AND from_key = ? AND lower(link_type) LIKE ? ORDER BY to_key`
		arg = "%test%"
	}
	rows, err := r.db.QueryContext(ctx, query, profileID, key, arg)
	if err != nil {
		return nil, fmt.Errorf("linked tests for %s: %w", key, err)
	}
	defer rows.Close()
	out := []LinkedTest{}
	for rows.Next() {
		var lt LinkedTest
		if err := rows.Scan(&lt.Key, &lt.Summary, &lt.LinkType); err != nil {
			return nil, err
		}
		out = append(out, lt)
	}
	return out, rows.Err()
}
