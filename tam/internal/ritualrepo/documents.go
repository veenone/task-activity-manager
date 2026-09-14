package ritualrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// The statuses a ritual document moves through. Nothing in the sync reads
// status to decide what is dirty (Dirty compares bodies); status is what the
// view draws, written by whichever operation changed it.
const (
	StatusLocal    = "local"
	StatusSynced   = "synced"
	StatusUnsynced = "unsynced"
	StatusConflict = "conflict"
	StatusGone     = "gone"
)

// Key names one ritual document.
type Key struct {
	ProfileID  string
	BoardID    int
	SprintID   int
	RitualType string
}

func (k Key) args() []any {
	return []any{k.ProfileID, k.BoardID, k.SprintID, normalizeRitualType(k.RitualType)}
}

// Document is one ritual page as TAM keeps it. Body is the local page,
// BaseBody the page as of Version, the last remote TAM synced with, and
// ConflictBody a newer remote a Sync found while local edits were pending.
type Document struct {
	ProfileID       string `json:"profileId"`
	BoardID         int    `json:"boardId"`
	SprintID        int    `json:"sprintId"`
	RitualType      string `json:"ritualType"`
	Title           string `json:"title"`
	Body            string `json:"body"`
	BaseBody        string `json:"baseBody"`
	PageID          string `json:"pageId"`
	Version         int    `json:"version"`
	ConflictBody    string `json:"conflictBody"`
	ConflictVersion int    `json:"conflictVersion"`
	Status          string `json:"status"`
	UpdatedAt       string `json:"updatedAt"`
	SyncedAt        string `json:"syncedAt"`
}

// Key is the document's own key.
func (d Document) Key() Key {
	return Key{ProfileID: d.ProfileID, BoardID: d.BoardID, SprintID: d.SprintID, RitualType: d.RitualType}
}

// Dirty is computed and never stored: a page Confluence has never seen, or a
// body that differs from the last one synced. A flag would drift from the
// bodies; this comparison cannot.
func (d Document) Dirty() bool { return d.PageID == "" || d.Body != d.BaseBody }

// Legacy is a document the ensure step has to render a template for, with
// whatever the retired wizard stored on it.
type Legacy struct {
	RitualType string
	Remark     string
	Issues     []Issue
}

// DB is the handle this repository runs on, for tests that need to write a
// row shape no method writes any more.
func (r *Repository) DB() *sql.DB { return r.db }

const documentColumns = `profile_id, board_id, sprint_id, ritual_type, title, body, base_body,
	confluence_page_id, confluence_version, conflict_body, conflict_version, status, updated_at, published_at`

const keyWhere = ` WHERE profile_id = ? AND board_id = ? AND sprint_id = ? AND ritual_type = ?`

type scanner interface{ Scan(...any) error }

func scanDocument(row scanner) (Document, error) {
	var d Document
	err := row.Scan(&d.ProfileID, &d.BoardID, &d.SprintID, &d.RitualType, &d.Title, &d.Body, &d.BaseBody,
		&d.PageID, &d.Version, &d.ConflictBody, &d.ConflictVersion, &d.Status, &d.UpdatedAt, &d.SyncedAt)
	return d, err
}

// Document reads one document; ok is false when none is stored.
func (r *Repository) Document(ctx context.Context, k Key) (Document, bool, error) {
	d, err := scanDocument(r.db.QueryRowContext(ctx, `SELECT `+documentColumns+` FROM ritual_document`+keyWhere, k.args()...))
	if errors.Is(err, sql.ErrNoRows) {
		return Document{}, false, nil
	}
	if err != nil {
		return Document{}, false, fmt.Errorf("read %s ritual for sprint %d: %w", k.RitualType, k.SprintID, err)
	}
	return d, true, nil
}

func (r *Repository) documents(ctx context.Context, where string, args ...any) ([]Document, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+documentColumns+` FROM ritual_document `+where, args...)
	if err != nil {
		return nil, fmt.Errorf("list ritual documents: %w", err)
	}
	defer rows.Close()
	out := []Document{}
	for rows.Next() {
		d, err := scanDocument(rows)
		if err != nil {
			return nil, fmt.Errorf("scan ritual document: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Documents lists one sprint's documents on one board.
func (r *Repository) Documents(ctx context.Context, profileID string, boardID, sprintID int) ([]Document, error) {
	return r.documents(ctx, `WHERE profile_id = ? AND board_id = ? AND sprint_id = ? ORDER BY ritual_type`, profileID, boardID, sprintID)
}

// ProfileDocuments lists every document of a profile that has a page, which
// is what the demo space is rebuilt from when the app restarts.
func (r *Repository) ProfileDocuments(ctx context.Context, profileID string) ([]Document, error) {
	return r.documents(ctx, `WHERE profile_id = ? ORDER BY board_id, sprint_id, ritual_type`, profileID)
}

// BoardSprintIDs lists the sprints of a board that hold any document, so a
// closed sprint that already has pages keeps syncing them.
func (r *Repository) BoardSprintIDs(ctx context.Context, profileID string, boardID int) ([]int, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT sprint_id FROM ritual_document WHERE profile_id = ? AND board_id = ? ORDER BY sprint_id`, profileID, boardID)
	if err != nil {
		return nil, fmt.Errorf("list ritual sprints for board %d: %w", boardID, err)
	}
	defer rows.Close()
	out := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// NeedsTemplate answers which of types have no usable document: no row, or a
// row with neither a body nor a page, which is every row written before the
// ritual sync existed. Each carries the retired wizard's remark and issues so
// the ensure step can keep what somebody typed. A malformed legacy column
// reads as no issues rather than failing, because it must not stop a sprint
// opening.
func (r *Repository) NeedsTemplate(ctx context.Context, profileID string, boardID, sprintID int, types []string) ([]Legacy, error) {
	out := []Legacy{}
	for _, t := range types {
		t = normalizeRitualType(t)
		var body, pageID, remark, issuesJSON string
		err := r.db.QueryRowContext(ctx, `SELECT body, confluence_page_id, remark, issues_json FROM ritual_document`+keyWhere,
			profileID, boardID, sprintID, t).Scan(&body, &pageID, &remark, &issuesJSON)
		if errors.Is(err, sql.ErrNoRows) {
			out = append(out, Legacy{RitualType: t, Issues: []Issue{}})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("check %s ritual for sprint %d: %w", t, sprintID, err)
		}
		if body != "" || pageID != "" {
			continue
		}
		issues, err := DecodeIssues(issuesJSON)
		if err != nil {
			issues = []Issue{}
		}
		out = append(out, Legacy{RitualType: t, Remark: remark, Issues: issues})
	}
	return out, nil
}

// WriteTemplate stores a rendered template as a local document. It never
// replaces a row that already has a body or a page.
func (r *Repository) WriteTemplate(ctx context.Context, k Key, title, body, now string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ritual_document (profile_id, board_id, sprint_id, ritual_type, title, body, base_body, status, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, '', 'local', ?)
		ON CONFLICT(profile_id, board_id, sprint_id, ritual_type) DO UPDATE SET
			title = excluded.title, body = excluded.body, base_body = '', status = 'local', updated_at = excluded.updated_at
		WHERE ritual_document.body = '' AND ritual_document.confluence_page_id = ''`,
		k.ProfileID, k.BoardID, k.SprintID, normalizeRitualType(k.RitualType), title, body, now)
	if err != nil {
		return fmt.Errorf("write %s template for sprint %d: %w", k.RitualType, k.SprintID, err)
	}
	return nil
}

func (r *Repository) update(ctx context.Context, what string, k Key, set string, setArgs []any, extraWhere string, extraArgs ...any) (int64, error) {
	args := append(append(setArgs, k.args()...), extraArgs...)
	res, err := r.db.ExecContext(ctx, `UPDATE ritual_document SET `+set+keyWhere+extraWhere, args...)
	if err != nil {
		return 0, fmt.Errorf("%s %s ritual for sprint %d: %w", what, k.RitualType, k.SprintID, err)
	}
	return res.RowsAffected()
}

// SaveBody is the editor's local save. It takes no lock: the sync's
// compare-and-set writes are what make a save during a pass safe. A row in
// conflict or gone keeps that status, since a save resolves neither.
func (r *Repository) SaveBody(ctx context.Context, k Key, body, now string) (Document, error) {
	n, err := r.update(ctx, "save", k, `body = ?, updated_at = ?,
		status = CASE
			WHEN status IN ('conflict', 'gone') THEN status
			WHEN confluence_page_id = '' THEN 'local'
			WHEN base_body = ? THEN 'synced'
			ELSE 'unsynced' END`, []any{body, now, body}, "")
	if err != nil {
		return Document{}, err
	}
	if n == 0 {
		return Document{}, fmt.Errorf("no %s ritual is stored for sprint %d", k.RitualType, k.SprintID)
	}
	d, _, err := r.Document(ctx, k)
	return d, err
}

// ApplyCreated records a page Sync created from pushed. body is left alone,
// so a save that landed while the create was in flight stays unsynced.
func (r *Repository) ApplyCreated(ctx context.Context, k Key, pageID, pushed string, version int, now string) error {
	_, err := r.update(ctx, "record created", k, `confluence_page_id = ?, base_body = ?, confluence_version = ?, published_at = ?,
		status = CASE WHEN body = ? THEN 'synced' ELSE 'unsynced' END`, []any{pageID, pushed, version, now, pushed}, "")
	return err
}

// ApplyPushed records a push of pushed at version, with ApplyCreated's rule
// for a save that landed during it.
func (r *Repository) ApplyPushed(ctx context.Context, k Key, pushed string, version int, now string) error {
	_, err := r.update(ctx, "record pushed", k, `base_body = ?, confluence_version = ?, published_at = ?,
		status = CASE WHEN body = ? THEN 'synced' ELSE 'unsynced' END`, []any{pushed, version, now, pushed}, "")
	return err
}

// ApplyPulled takes a remote body, but only while the local body is still
// readBody, the value the pass read. false means a save landed in between
// and nothing was written; the caller records a conflict instead.
func (r *Repository) ApplyPulled(ctx context.Context, k Key, pageID, readBody, remote string, version int, now string) (bool, error) {
	n, err := r.update(ctx, "record pulled", k, `confluence_page_id = ?, body = ?, base_body = ?, confluence_version = ?,
		conflict_body = '', conflict_version = 0, status = 'synced', published_at = ?`,
		[]any{pageID, remote, remote, version, now}, ` AND body = ?`, readBody)
	return n == 1, err
}

// ApplyConflict keeps a newer remote beside the local body. It never touches
// body, so it needs no compare-and-set.
func (r *Repository) ApplyConflict(ctx context.Context, k Key, pageID, remote string, version int) error {
	_, err := r.update(ctx, "record conflict", k, `confluence_page_id = ?, conflict_body = ?, conflict_version = ?, status = 'conflict'`,
		[]any{pageID, remote, version}, "")
	return err
}

// MarkGone records a page Confluence no longer answers for.
func (r *Repository) MarkGone(ctx context.Context, k Key) error {
	_, err := r.update(ctx, "mark gone", k, `status = 'gone'`, nil, "")
	return err
}

// ResolveMine rebases local edits onto the newer remote, so the next Sync
// pushes them over it. It is RebaseMoves for a page.
func (r *Repository) ResolveMine(ctx context.Context, k Key, now string) error {
	n, err := r.update(ctx, "keep mine", k, `base_body = conflict_body, confluence_version = conflict_version,
		conflict_body = '', conflict_version = 0, updated_at = ?,
		status = CASE WHEN body = conflict_body THEN 'synced' ELSE 'unsynced' END`, []any{now}, ` AND status = 'conflict'`)
	if err == nil && n == 0 {
		err = fmt.Errorf("the %s ritual for sprint %d has no conflict to resolve", k.RitualType, k.SprintID)
	}
	return err
}

// ResolveTheirs discards local edits for the newer remote.
func (r *Repository) ResolveTheirs(ctx context.Context, k Key, now string) error {
	n, err := r.update(ctx, "take theirs", k, `body = conflict_body, base_body = conflict_body, confluence_version = conflict_version,
		conflict_body = '', conflict_version = 0, updated_at = ?, status = 'synced'`, []any{now}, ` AND status = 'conflict'`)
	if err == nil && n == 0 {
		err = fmt.Errorf("the %s ritual for sprint %d has no conflict to resolve", k.RitualType, k.SprintID)
	}
	return err
}

// ForgetPage lets a gone page be created again on the next Sync, keeping the
// local body it will be created from.
func (r *Repository) ForgetPage(ctx context.Context, k Key, now string) error {
	n, err := r.update(ctx, "forget page", k, `confluence_page_id = '', confluence_version = 0, base_body = '',
		conflict_body = '', conflict_version = 0, updated_at = ?, status = 'local'`, []any{now}, ` AND status = 'gone'`)
	if err == nil && n == 0 {
		err = fmt.Errorf("the %s ritual for sprint %d is not gone from Confluence", k.RitualType, k.SprintID)
	}
	return err
}

// DeleteDocument removes the local copy only. A page in Confluence is left
// exactly where it is.
func (r *Repository) DeleteDocument(ctx context.Context, k Key) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM ritual_document`+keyWhere, k.args()...)
	if err != nil {
		return fmt.Errorf("delete %s ritual for sprint %d: %w", k.RitualType, k.SprintID, err)
	}
	return nil
}
