// Package ritualrepo stores locally editable ritual documents in tam.db.
package ritualrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Draft is one ritual document and its Confluence publication state.
type Draft struct {
	ProfileID         string `json:"profileId"`
	BoardID           int    `json:"boardId"`
	SprintID          int    `json:"sprintId"`
	RitualType        string `json:"ritualType"`
	Title             string `json:"title"`
	Remark            string `json:"remark"`
	Body              string `json:"body"`
	IssueKeysJSON     string `json:"issueKeysJson"`
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
		issue_keys_json, confluence_page_id, confluence_version, status,
		updated_at, published_at
	FROM ritual_document
	WHERE profile_id = ? AND board_id = ? AND sprint_id = ? AND ritual_type = ?`

// Get returns the selected draft, or a zero Draft when no document has been
// stored for the composite key.
func (r *Repository) Get(ctx context.Context, profileID string, boardID, sprintID int, ritualType string) (Draft, error) {
	var draft Draft
	err := r.db.QueryRowContext(ctx, selectDraftSQL, profileID, boardID, sprintID, ritualType).Scan(
		&draft.ProfileID, &draft.BoardID, &draft.SprintID, &draft.RitualType,
		&draft.Title, &draft.Remark, &draft.Body, &draft.IssueKeysJSON,
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
		issue_keys_json, confluence_page_id, confluence_version, status,
		updated_at, published_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(profile_id, board_id, sprint_id, ritual_type) DO UPDATE SET
		title = excluded.title,
		remark = excluded.remark,
		body = excluded.body,
		issue_keys_json = excluded.issue_keys_json,
		confluence_page_id = excluded.confluence_page_id,
		confluence_version = excluded.confluence_version,
		status = excluded.status,
		updated_at = excluded.updated_at,
		published_at = excluded.published_at`

// Upsert inserts a new document or replaces the editable and publication
// fields of the document with the same composite key.
func (r *Repository) Upsert(ctx context.Context, draft Draft) error {
	_, err := r.db.ExecContext(ctx, upsertDraftSQL,
		draft.ProfileID, draft.BoardID, draft.SprintID, draft.RitualType,
		draft.Title, draft.Remark, draft.Body, draft.IssueKeysJSON,
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
