// Package ritualrepo stores locally editable ritual documents in tam.db.
package ritualrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Issue is one Jira issue chosen for a ritual, with the remark the team made
// about it. The slice order is the order the published table uses, so it is
// preserved exactly as stored and never re-sorted from a query result.
type Issue struct {
	Key    string `json:"key"`
	Remark string `json:"remark"`
}

// EncodeIssues renders the chosen issues for storage. A nil slice encodes as an
// empty array rather than null, so the column's default and a cleared selection
// are the same value.
func EncodeIssues(issues []Issue) (string, error) {
	if issues == nil {
		issues = []Issue{}
	}
	encoded, err := json.Marshal(issues)
	if err != nil {
		return "", fmt.Errorf("encode ritual issues: %w", err)
	}
	return string(encoded), nil
}

// DecodeIssues reads stored issues. An empty column decodes as an empty slice.
func DecodeIssues(encoded string) ([]Issue, error) {
	trimmed := strings.TrimSpace(encoded)
	if trimmed == "" {
		return []Issue{}, nil
	}
	var issues []Issue
	if err := json.Unmarshal([]byte(trimmed), &issues); err != nil {
		return nil, fmt.Errorf("decode ritual issues: %w", err)
	}
	if issues == nil {
		issues = []Issue{}
	}
	return issues, nil
}

// Draft is one ritual document and its Confluence publication state.
type Draft struct {
	ProfileID         string `json:"profileId"`
	BoardID           int    `json:"boardId"`
	SprintID          int    `json:"sprintId"`
	RitualType        string `json:"ritualType"`
	Title             string `json:"title"`
	Remark            string `json:"remark"`
	Body              string `json:"body"`
	IssuesJSON        string `json:"issuesJson"`
	ConfluencePageID  string `json:"confluencePageId"`
	ConfluenceVersion int    `json:"confluenceVersion"`
	Status            string `json:"status"`
	UpdatedAt         string `json:"updatedAt"`
	PublishedAt       string `json:"publishedAt"`
}

// Repository runs ritual document queries against an open tam.db handle.
type Repository struct {
	db *sql.DB
}

// New wraps an open tam.db handle.
func New(db *sql.DB) *Repository { return &Repository{db: db} }

const selectDraftSQL = `
	SELECT profile_id, board_id, sprint_id, ritual_type, title, remark, body,
		issues_json, confluence_page_id, confluence_version, status,
		updated_at, published_at
	FROM ritual_document
	WHERE profile_id = ? AND board_id = ? AND sprint_id = ? AND ritual_type = ?`

// normalizeRitualType matches ritual_type's case-insensitive treatment
// everywhere in this package: the column carries no COLLATE NOCASE, so a
// caller passing "Review" against a stored "review" row would otherwise
// match nothing, and a write that follows such a miss would silently blank
// a real row's publication fields instead of updating it. Every key-taking
// method normalizes here rather than trusting a caller that has already
// validated the type against knownRitualType, so Get, Upsert, and Delete can
// never disagree about which row a given type names.
func normalizeRitualType(ritualType string) string {
	return strings.TrimSpace(strings.ToLower(ritualType))
}

// Get returns the selected draft, or a zero Draft when no document has been
// stored for the composite key.
func (r *Repository) Get(ctx context.Context, profileID string, boardID, sprintID int, ritualType string) (Draft, error) {
	ritualType = normalizeRitualType(ritualType)
	var draft Draft
	err := r.db.QueryRowContext(ctx, selectDraftSQL, profileID, boardID, sprintID, ritualType).Scan(
		&draft.ProfileID, &draft.BoardID, &draft.SprintID, &draft.RitualType,
		&draft.Title, &draft.Remark, &draft.Body, &draft.IssuesJSON,
		&draft.ConfluencePageID, &draft.ConfluenceVersion, &draft.Status,
		&draft.UpdatedAt, &draft.PublishedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Draft{}, nil
	}
	if err != nil {
		return Draft{}, fmt.Errorf("get %s ritual for board %d sprint %d: %w", ritualType, boardID, sprintID, err)
	}
	return draft, nil
}

const upsertDraftSQL = `
	INSERT INTO ritual_document (
		profile_id, board_id, sprint_id, ritual_type, title, remark, body,
		issues_json, confluence_page_id, confluence_version, status,
		updated_at, published_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(profile_id, board_id, sprint_id, ritual_type) DO UPDATE SET
		title = excluded.title,
		remark = excluded.remark,
		body = excluded.body,
		issues_json = excluded.issues_json,
		confluence_page_id = excluded.confluence_page_id,
		confluence_version = excluded.confluence_version,
		status = excluded.status,
		updated_at = excluded.updated_at,
		published_at = excluded.published_at`

// Upsert inserts a new document or replaces the editable and publication
// fields of the document with the same composite key.
func (r *Repository) Upsert(ctx context.Context, draft Draft) error {
	draft.RitualType = normalizeRitualType(draft.RitualType)
	_, err := r.db.ExecContext(ctx, upsertDraftSQL,
		draft.ProfileID, draft.BoardID, draft.SprintID, draft.RitualType,
		draft.Title, draft.Remark, draft.Body, draft.IssuesJSON,
		draft.ConfluencePageID, draft.ConfluenceVersion, draft.Status,
		draft.UpdatedAt, draft.PublishedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert %s ritual for board %d sprint %d: %w", draft.RitualType, draft.BoardID, draft.SprintID, err)
	}
	return nil
}

// Delete removes only the document identified by the full composite key.
func (r *Repository) Delete(ctx context.Context, profileID string, boardID, sprintID int, ritualType string) error {
	ritualType = normalizeRitualType(ritualType)
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM ritual_document
		WHERE profile_id = ? AND board_id = ? AND sprint_id = ? AND ritual_type = ?`,
		profileID, boardID, sprintID, ritualType,
	)
	if err != nil {
		return fmt.Errorf("delete %s ritual for board %d sprint %d: %w", ritualType, boardID, sprintID, err)
	}
	return nil
}

const listDraftsSQL = `
	SELECT profile_id, board_id, sprint_id, ritual_type, title, remark, body,
		issues_json, confluence_page_id, confluence_version, status,
		updated_at, published_at
	FROM ritual_document
	WHERE profile_id = ? AND board_id = ? AND sprint_id = ?
	ORDER BY ritual_type`

// ListDrafts returns every ritual document stored for one sprint, ordered by
// ritual type so the view's slots do not reshuffle between reads.
func (r *Repository) ListDrafts(ctx context.Context, profileID string, boardID, sprintID int) ([]Draft, error) {
	rows, err := r.db.QueryContext(ctx, listDraftsSQL, profileID, boardID, sprintID)
	if err != nil {
		return nil, fmt.Errorf("list rituals for board %d sprint %d: %w", boardID, sprintID, err)
	}
	defer rows.Close()

	drafts := []Draft{}
	for rows.Next() {
		var draft Draft
		if err := rows.Scan(
			&draft.ProfileID, &draft.BoardID, &draft.SprintID, &draft.RitualType,
			&draft.Title, &draft.Remark, &draft.Body, &draft.IssuesJSON,
			&draft.ConfluencePageID, &draft.ConfluenceVersion, &draft.Status,
			&draft.UpdatedAt, &draft.PublishedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ritual for board %d sprint %d: %w", boardID, sprintID, err)
		}
		drafts = append(drafts, draft)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rituals for board %d sprint %d: %w", boardID, sprintID, err)
	}
	return drafts, nil
}
