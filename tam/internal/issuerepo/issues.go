package issuerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"agile-suite/tam/internal/backend"
)

const (
	defaultPageSize = 25
	maxPageSize     = 500
)

// issueColumns is the SELECT list every row read uses, in scan order.
const issueColumns = `key, id, project, type, summary, status, status_id, assignee, reporter, priority, labels,
	sprint_id, sprint_name, parent_key, story_points, rank, created, updated, ` + pendingFlag

// issueOrder puts drafts first, then ranked rows by rank with unranked rows
// last, then key. ListIssues and the tree share it so the grid and the
// Epics view agree on one row order.
const issueOrder = ` ORDER BY CASE WHEN key LIKE '` + DraftPrefix + `%' THEN 0 WHEN rank = '' THEN 2 ELSE 1 END, rank, key`

// sortColumns maps the sort keys the grid sends to the SQL that orders by
// them. It is a whitelist, not a format string: the value from the frontend
// only ever selects an entry here, so no caller input reaches the query.
//
// Every expression sorts blanks last, so a page of issues does not open on a
// block of rows with nothing in the sorted column. Story points are numeric
// and NULL when unset; the rest are text, compared case-insensitively because
// a Jira status or display name is prose, not an identifier.
var sortColumns = map[string]string{
	"key":         "key COLLATE NOCASE",
	"type":        "type = '' , type COLLATE NOCASE",
	"summary":     "summary = '' , summary COLLATE NOCASE",
	"status":      "status = '' , status COLLATE NOCASE",
	"assignee":    "assignee = '' , assignee COLLATE NOCASE",
	"sprint":      "sprint_name = '' , sprint_name COLLATE NOCASE",
	"storyPoints": "story_points IS NULL, story_points",
}

// SortColumns lists the sort keys ListIssues accepts, for the frontend and
// the tests to agree with without repeating the list.
func SortColumns() []string {
	out := make([]string, 0, len(sortColumns))
	for k := range sortColumns {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// orderFor is the ORDER BY for q: the default rank order when no sort column
// is named or the name is not one this store knows, otherwise the named
// column with the key as the tie-break so equal values keep a stable page
// boundary. Drafts stay pinned to the top under every sort: they are the
// user's own uncommitted rows and burying them under a sort would hide work
// that has not reached Jira yet.
func orderFor(q IssueQuery) string {
	expr, ok := sortColumns[q.Sort]
	if !ok {
		return issueOrder
	}
	dir := " ASC"
	if q.Desc {
		dir = " DESC"
	}
	// The blanks-last guard and the tie-break are not reversed with the
	// column: blanks belong at the bottom either way, and the tie-break only
	// has to be deterministic.
	parts := strings.Split(expr, " , ")
	ordered := make([]string, len(parts))
	for i, part := range parts {
		if i < len(parts)-1 {
			ordered[i] = part // the "is blank" guard, always ascending
			continue
		}
		ordered[i] = part + dir
	}
	return ` ORDER BY CASE WHEN key LIKE '` + DraftPrefix + `%' THEN 0 ELSE 1 END, ` +
		strings.Join(ordered, ", ") + `, key`
}

const upsertIssueSQL = `
	INSERT INTO issue (profile_id, key, id, project, type, summary, status, status_id, assignee, reporter, priority, labels,
		sprint_id, sprint_name, parent_key, story_points, rank, created, updated, synced_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(profile_id, key) DO UPDATE SET
		id = excluded.id, project = excluded.project, type = excluded.type, summary = excluded.summary,
		status = excluded.status, status_id = excluded.status_id, assignee = excluded.assignee, reporter = excluded.reporter,
		priority = excluded.priority, labels = excluded.labels, sprint_id = excluded.sprint_id,
		sprint_name = excluded.sprint_name, parent_key = excluded.parent_key,
		story_points = excluded.story_points, rank = excluded.rank, created = excluded.created,
		updated = excluded.updated, synced_at = excluded.synced_at`

func upsertIssue(ctx context.Context, q execer, profileID string, iss backend.Issue, syncedAt time.Time) error {
	labels, err := json.Marshal(nonNil(iss.Labels))
	if err != nil {
		return fmt.Errorf("labels for %s: %w", iss.Key, err)
	}
	var points sql.NullFloat64
	if iss.StoryPoints != nil {
		points = sql.NullFloat64{Float64: *iss.StoryPoints, Valid: true}
	}
	if _, err := q.ExecContext(ctx, upsertIssueSQL, profileID, iss.Key, iss.ID, iss.Project, iss.Type, iss.Summary, iss.Status, iss.StatusID,
		iss.Assignee, iss.Reporter, iss.Priority, string(labels), iss.SprintID, iss.SprintName, iss.ParentKey,
		points, iss.Rank, iss.Created, iss.Updated, syncedAt.UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("upsert %s: %w", iss.Key, err)
	}
	return nil
}

// UpsertPage lands one page from Jira. With clearFirst the profile's synced
// rows go first (drafts stay). Columns with a pending edit are written back
// from the journal afterwards, so a sync never hides a local change.
func (r *Repository) UpsertPage(ctx context.Context, profileID string, page []backend.Issue, syncedAt time.Time, clearFirst bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if clearFirst {
		if _, err := tx.ExecContext(ctx, `DELETE FROM issue_link WHERE profile_id = ? AND from_key NOT LIKE ?`, profileID, DraftPrefix+"%"); err != nil {
			return fmt.Errorf("clear links: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM issue WHERE profile_id = ? AND key NOT LIKE ?`, profileID, DraftPrefix+"%"); err != nil {
			return fmt.Errorf("clear issues: %w", err)
		}
	}
	keys := make(map[string]bool, len(page))
	for _, iss := range page {
		if err := upsertIssue(ctx, tx, profileID, iss, syncedAt); err != nil {
			return err
		}
		keys[iss.Key] = true
	}
	if err := reapplyPending(ctx, tx, profileID, keys); err != nil {
		return err
	}
	return tx.Commit()
}

// ListIssues returns one page matching q. Rows come back in q's sort order,
// or drafts first then rank with unranked rows last then key when q names no
// sort column.
func (r *Repository) ListIssues(ctx context.Context, profileID string, q IssueQuery) (IssuePage, error) {
	where, args := issueFilter(profileID, q)
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM issue WHERE `+where, args...).Scan(&total); err != nil {
		return IssuePage{}, fmt.Errorf("count issues: %w", err)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+issueColumns+` FROM issue WHERE `+where+orderFor(q)+` LIMIT ? OFFSET ?`,
		append(args, limit, offset)...)
	if err != nil {
		return IssuePage{}, fmt.Errorf("list issues: %w", err)
	}
	defer rows.Close()
	issues := make([]backend.Issue, 0, limit)
	for rows.Next() {
		iss, err := scanIssue(rows)
		if err != nil {
			return IssuePage{}, err
		}
		issues = append(issues, iss)
	}
	if err := rows.Err(); err != nil {
		return IssuePage{}, err
	}
	return IssuePage{Issues: issues, Total: total}, nil
}

// GetIssue returns one cached row or ErrNotFound.
func (r *Repository) GetIssue(ctx context.Context, profileID, key string) (backend.Issue, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+issueColumns+` FROM issue WHERE profile_id = ? AND key = ?`, profileID, key)
	iss, err := scanIssue(row)
	if errors.Is(err, sql.ErrNoRows) {
		return backend.Issue{}, ErrNotFound
	}
	return iss, err
}

// CountIssues is the profile's cached row count, for the status bar.
func (r *Repository) CountIssues(ctx context.Context, profileID string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM issue WHERE profile_id = ?`, profileID).Scan(&n)
	return n, err
}

// ListSprints returns the distinct sprints in the cache, sorted by numeric
// id so "Sprint 12" precedes "Sprint 13".
func (r *Repository) ListSprints(ctx context.Context, profileID string) ([]SprintRef, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT sprint_id, sprint_name FROM issue WHERE profile_id = ? AND sprint_id <> '' ORDER BY CAST(sprint_id AS INTEGER), sprint_id`,
		profileID)
	if err != nil {
		return nil, fmt.Errorf("list sprints: %w", err)
	}
	defer rows.Close()
	var out []SprintRef
	for rows.Next() {
		var s SprintRef
		if err := rows.Scan(&s.ID, &s.Name); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// keyChunk is how many keys go into one IN list. SQLite's default limit on
// host parameters is 999, so a board of thousands of cards is read in
// several statements rather than one that would fail.
const keyChunk = 500

// IssuesByKeys returns the cached issues for keys, in the caller's own
// order, skipping the keys the cache does not hold. A board's card order is
// the board's, not the database's, and no ORDER BY can ask SQL to return an
// IN list in the order it was given, so the rows are scanned into a map and
// the caller's slice is walked to build the result.
func (r *Repository) IssuesByKeys(ctx context.Context, profileID string, keys []string) ([]backend.Issue, error) {
	if len(keys) == 0 {
		return []backend.Issue{}, nil
	}
	found := make(map[string]backend.Issue, len(keys))
	for start := 0; start < len(keys); start += keyChunk {
		end := start + keyChunk
		if end > len(keys) {
			end = len(keys)
		}
		chunk := keys[start:end]
		marks := strings.TrimSuffix(strings.Repeat("?, ", len(chunk)), ", ")
		args := make([]any, 0, len(chunk)+1)
		args = append(args, profileID)
		for _, k := range chunk {
			args = append(args, k)
		}
		if err := r.scanIssuesInto(ctx, found,
			`SELECT `+issueColumns+` FROM issue WHERE profile_id = ? AND key IN (`+marks+`)`, args); err != nil {
			return nil, err
		}
	}
	out := make([]backend.Issue, 0, len(keys))
	for _, k := range keys {
		if iss, ok := found[k]; ok {
			out = append(out, iss)
		}
	}
	return out, nil
}

// scanIssuesInto runs one row read and files every row under its key.
func (r *Repository) scanIssuesInto(ctx context.Context, into map[string]backend.Issue, query string, args []any) error {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("issues by keys: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		iss, err := scanIssue(rows)
		if err != nil {
			return err
		}
		into[iss.Key] = iss
	}
	return rows.Err()
}

// issueFilter builds the WHERE clause for q. The profile is always the first
// condition, so an empty query still scopes to one profile.
func issueFilter(profileID string, q IssueQuery) (string, []any) {
	where := []string{"profile_id = ?"}
	args := []any{profileID}
	if len(q.Types) > 0 {
		marks := strings.TrimSuffix(strings.Repeat("?, ", len(q.Types)), ", ")
		where = append(where, "type IN ("+marks+")")
		for _, t := range q.Types {
			args = append(args, t)
		}
	}
	if q.SprintID != "" {
		where = append(where, "sprint_id = ?")
		args = append(args, q.SprintID)
	}
	if text := strings.TrimSpace(q.Text); text != "" {
		like := "%" + escapeLike(text) + "%"
		where = append(where, "(key LIKE ? ESCAPE '\\' OR summary LIKE ? ESCAPE '\\' OR labels LIKE ? ESCAPE '\\')")
		args = append(args, like, like, like)
	}
	return strings.Join(where, " AND "), args
}

// escapeLike makes the user's text literal inside a LIKE pattern.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	return strings.ReplaceAll(s, "_", `\_`)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanIssue(s scanner) (backend.Issue, error) {
	var (
		iss     backend.Issue
		labels  string
		points  sql.NullFloat64
		pending int
	)
	if err := s.Scan(&iss.Key, &iss.ID, &iss.Project, &iss.Type, &iss.Summary, &iss.Status, &iss.StatusID, &iss.Assignee,
		&iss.Reporter, &iss.Priority, &labels, &iss.SprintID, &iss.SprintName, &iss.ParentKey, &points,
		&iss.Rank, &iss.Created, &iss.Updated, &pending); err != nil {
		return backend.Issue{}, err
	}
	if err := json.Unmarshal([]byte(labels), &iss.Labels); err != nil {
		return backend.Issue{}, fmt.Errorf("labels for %s: %w", iss.Key, err)
	}
	iss.Labels = nonNil(iss.Labels)
	if points.Valid {
		v := points.Float64
		iss.StoryPoints = &v
	}
	iss.Pending = pending != 0
	iss.Draft = strings.HasPrefix(iss.Key, DraftPrefix)
	return iss, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
