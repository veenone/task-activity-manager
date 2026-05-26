// Package testrepo is the local repository for cached Xray Test data.
//
// It is the query layer behind the browse / search / filter experience
// (FR-11), the write target of the sync engine (FR-1), and the home of the
// local change-tracking and audit-log machinery (FR-1.5 / FR-1.6 / FR-12.6).
package testrepo

import (
	"database/sql"
	"errors"
	"fmt"
	"os/user"
	"strings"
	"time"

	"xray-test-manager/internal/store"
)

// ErrNotFound is returned when a Test key is not in the local store.
var ErrNotFound = errors.New("test not found")

// TestCase is a Xray Test as cached locally.
type TestCase struct {
	Key         string   `json:"key"`
	ID          string   `json:"id"`
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	Labels      []string `json:"labels"`
	Updated     string   `json:"updated"`
	FolderID    string   `json:"folderId"`
}

// Folder is one node in the Xray Test Repository tree (FR-13.1). The ID is
// the folder's full path ("/Authentication/Login"), so ParentID + Name + ID
// together describe the tree without any extra lookup tables.
type Folder struct {
	ID       string `json:"id"`
	ParentID string `json:"parentId"`
	Name     string `json:"name"`
}

// Precondition mirrors a Xray Precondition issue (FR-13.4).
type Precondition struct {
	Key         string `json:"key"`
	Summary     string `json:"summary"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// PendingChange is one uncommitted local edit awaiting a push (FR-1.5 / 1.6).
// At most one PendingChange exists per (profile, entity, field); repeated
// edits to the same field are coalesced — BeforeVal stays at the original
// value, AfterVal advances. Reverting to the original removes the row.
type PendingChange struct {
	ID          int64  `json:"id"`
	EntityType  string `json:"entityType"`
	EntityKey   string `json:"entityKey"`
	Field       string `json:"field"`
	BeforeVal   string `json:"beforeVal"`
	AfterVal    string `json:"afterVal"`
	BaseVersion string `json:"baseVersion"`
	CreatedAt   string `json:"createdAt"`
}

// AuditEntry is one row of the local audit trail (FR-12.6 / NFR-13). Every
// local change records who / what / when / before → after.
type AuditEntry struct {
	ID         int64  `json:"id"`
	OccurredAt string `json:"occurredAt"`
	Actor      string `json:"actor"`
	EntityType string `json:"entityType"`
	EntityKey  string `json:"entityKey"`
	Action     string `json:"action"`
	Field      string `json:"field"`
	BeforeVal  string `json:"beforeVal"`
	AfterVal   string `json:"afterVal"`
	Note       string `json:"note"`
}

// Query drives a ListTests call: free-text search, filters, sorting, paging.
type Query struct {
	Search   string `json:"search"`
	Status   string `json:"status"`
	FolderID string `json:"folderId"` // empty = any folder
	SortBy   string `json:"sortBy"`
	Desc     bool   `json:"desc"`
	Limit    int    `json:"limit"`
	Offset   int    `json:"offset"`
}

// Page is one page of list results plus the total matching count.
type Page struct {
	Tests []TestCase `json:"tests"`
	Total int        `json:"total"`
}

// SyncState records the outcome of the last sync for a profile.
type SyncState struct {
	ProfileID    string `json:"profileId"`
	LastSyncedAt string `json:"lastSyncedAt"`
	TestCount    int    `json:"testCount"`
}

// sortColumns whitelists user-supplied sort keys to real columns, so Query.SortBy
// can never reach the SQL string directly.
var sortColumns = map[string]string{
	"key":     "jira_key",
	"summary": "summary",
	"status":  "status",
	"updated": "updated_at",
}

// editableFields whitelists which Test fields can be edited via EditTestField.
// Status transitions need workflow logic and are handled separately in a
// later slice (FR-4.2).
var editableFields = map[string]string{
	"summary":     "summary",
	"description": "description",
	"priority":    "priority",
	"labels":      "labels",
}

// Repository reads and writes cached data, scoped per profile.
type Repository struct {
	db *sql.DB
}

// NewRepository returns a repository backed by the given store.
func NewRepository(s *store.Store) *Repository {
	return &Repository{db: s.DB()}
}

// UpsertTests inserts or updates a batch of Tests in one transaction.
//
// TODO(xtm): preserve local pending edits — currently an incoming sync
// overwrites the test_case row regardless of any pending_change. Phase 2
// conflict detection (FR-1.4) will fix this.
func (r *Repository) UpsertTests(profileID string, tests []TestCase) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(
		`INSERT INTO test_case
		   (profile_id, jira_key, jira_id, summary, description, status, priority, labels, updated_at, folder_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(profile_id, jira_key) DO UPDATE SET
		   jira_id     = excluded.jira_id,
		   summary     = excluded.summary,
		   description = excluded.description,
		   status      = excluded.status,
		   priority    = excluded.priority,
		   labels      = excluded.labels,
		   updated_at  = excluded.updated_at,
		   folder_id   = excluded.folder_id`)
	if err != nil {
		return fmt.Errorf("prepare upsert: %w", err)
	}
	defer stmt.Close()

	for _, t := range tests {
		if _, err := stmt.Exec(
			profileID, t.Key, t.ID, t.Summary, t.Description,
			t.Status, t.Priority, strings.Join(t.Labels, " "),
			t.Updated, t.FolderID,
		); err != nil {
			return fmt.Errorf("upsert %s: %w", t.Key, err)
		}
	}
	return tx.Commit()
}

// UpsertFolders inserts or updates a batch of Test Repository folders.
func (r *Repository) UpsertFolders(profileID string, folders []Folder) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(
		`INSERT INTO test_folder (profile_id, id, parent_id, name)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(profile_id, id) DO UPDATE SET
		   parent_id = excluded.parent_id,
		   name      = excluded.name`)
	if err != nil {
		return fmt.Errorf("prepare upsert folder: %w", err)
	}
	defer stmt.Close()

	for _, f := range folders {
		if _, err := stmt.Exec(profileID, f.ID, f.ParentID, f.Name); err != nil {
			return fmt.Errorf("upsert folder %s: %w", f.ID, err)
		}
	}
	return tx.Commit()
}

// ListFolders returns the folder tree for a profile, ordered by id (which is
// the path, so a stable depth-first ordering falls out naturally).
func (r *Repository) ListFolders(profileID string) ([]Folder, error) {
	rows, err := r.db.Query(
		`SELECT id, parent_id, name FROM test_folder WHERE profile_id = ? ORDER BY id`,
		profileID)
	if err != nil {
		return nil, fmt.Errorf("list folders: %w", err)
	}
	defer rows.Close()

	out := []Folder{}
	for rows.Next() {
		var f Folder
		if err := rows.Scan(&f.ID, &f.ParentID, &f.Name); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// UpsertPreconditions inserts or updates a batch of Preconditions.
func (r *Repository) UpsertPreconditions(profileID string, preconditions []Precondition) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(
		`INSERT INTO precondition (profile_id, jira_key, summary, type, description)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(profile_id, jira_key) DO UPDATE SET
		   summary     = excluded.summary,
		   type        = excluded.type,
		   description = excluded.description`)
	if err != nil {
		return fmt.Errorf("prepare upsert precondition: %w", err)
	}
	defer stmt.Close()

	for _, p := range preconditions {
		if _, err := stmt.Exec(profileID, p.Key, p.Summary, p.Type, p.Description); err != nil {
			return fmt.Errorf("upsert precondition %s: %w", p.Key, err)
		}
	}
	return tx.Commit()
}

// ReplaceAllTestPreconditions wipes a profile's Test-to-Precondition link
// table and rewrites it from the provided map. Used by FullSync so removed
// links actually disappear on resync.
func (r *Repository) ReplaceAllTestPreconditions(profileID string, links map[string][]string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(
		`DELETE FROM test_precondition WHERE profile_id = ?`, profileID,
	); err != nil {
		return fmt.Errorf("clear precondition links: %w", err)
	}

	if len(links) == 0 {
		return tx.Commit()
	}

	stmt, err := tx.Prepare(
		`INSERT INTO test_precondition (profile_id, test_key, precondition_key)
		 VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert link: %w", err)
	}
	defer stmt.Close()

	for testKey, preKeys := range links {
		for _, pk := range preKeys {
			if _, err := stmt.Exec(profileID, testKey, pk); err != nil {
				return fmt.Errorf("link %s -> %s: %w", testKey, pk, err)
			}
		}
	}
	return tx.Commit()
}

// ListTestPreconditions returns the Preconditions linked to a Test.
func (r *Repository) ListTestPreconditions(profileID, testKey string) ([]Precondition, error) {
	rows, err := r.db.Query(
		`SELECT p.jira_key, p.summary, p.type, p.description
		 FROM test_precondition tp
		 JOIN precondition p
		   ON p.profile_id = tp.profile_id AND p.jira_key = tp.precondition_key
		 WHERE tp.profile_id = ? AND tp.test_key = ?
		 ORDER BY p.jira_key`,
		profileID, testKey)
	if err != nil {
		return nil, fmt.Errorf("list test preconditions: %w", err)
	}
	defer rows.Close()

	out := []Precondition{}
	for rows.Next() {
		var p Precondition
		if err := rows.Scan(&p.Key, &p.Summary, &p.Type, &p.Description); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListTests returns a filtered, sorted, paginated page of Tests for a profile.
// A FolderID filter matches the folder itself plus any descendants so
// selecting a category in the tree shows everything beneath it.
func (r *Repository) ListTests(profileID string, q Query) (Page, error) {
	where := []string{"profile_id = ?"}
	args := []any{profileID}

	if q.Search != "" {
		where = append(where, "(jira_key LIKE ? OR summary LIKE ? OR description LIKE ?)")
		like := "%" + q.Search + "%"
		args = append(args, like, like, like)
	}
	if q.Status != "" {
		where = append(where, "status = ?")
		args = append(args, q.Status)
	}
	if q.FolderID != "" {
		where = append(where, "(folder_id = ? OR folder_id LIKE ?)")
		args = append(args, q.FolderID, q.FolderID+"/%")
	}
	whereSQL := "WHERE " + strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(
		"SELECT COUNT(*) FROM test_case "+whereSQL, args...,
	).Scan(&total); err != nil {
		return Page{}, fmt.Errorf("count tests: %w", err)
	}

	sortCol, ok := sortColumns[q.SortBy]
	if !ok {
		sortCol = "jira_key"
	}
	dir := "ASC"
	if q.Desc {
		dir = "DESC"
	}
	limit := q.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	listSQL := fmt.Sprintf(
		`SELECT jira_key, jira_id, summary, description, status, priority, labels, updated_at, folder_id
		 FROM test_case %s ORDER BY %s %s LIMIT ? OFFSET ?`,
		whereSQL, sortCol, dir)

	rows, err := r.db.Query(listSQL, append(args, limit, q.Offset)...)
	if err != nil {
		return Page{}, fmt.Errorf("list tests: %w", err)
	}
	defer rows.Close()

	page := Page{Total: total, Tests: []TestCase{}}
	for rows.Next() {
		t, err := scanTest(rows)
		if err != nil {
			return Page{}, err
		}
		page.Tests = append(page.Tests, t)
	}
	return page, rows.Err()
}

// GetTest returns one Test by its Jira key, or ErrNotFound.
func (r *Repository) GetTest(profileID, key string) (TestCase, error) {
	row := r.db.QueryRow(
		`SELECT jira_key, jira_id, summary, description, status, priority, labels, updated_at, folder_id
		 FROM test_case WHERE profile_id = ? AND jira_key = ?`, profileID, key)
	t, err := scanTest(row)
	if errors.Is(err, sql.ErrNoRows) {
		return TestCase{}, ErrNotFound
	}
	return t, err
}

// SetSyncState records that a profile finished syncing now. The test count
// is derived from the current row count so the state stays accurate after
// both full and incremental syncs.
func (r *Repository) SetSyncState(profileID string) error {
	var count int
	if err := r.db.QueryRow(
		`SELECT COUNT(*) FROM test_case WHERE profile_id = ?`, profileID,
	).Scan(&count); err != nil {
		return fmt.Errorf("count tests for sync state: %w", err)
	}
	_, err := r.db.Exec(
		`INSERT INTO sync_state (profile_id, last_synced_at, test_count) VALUES (?, ?, ?)
		 ON CONFLICT(profile_id) DO UPDATE SET
		   last_synced_at = excluded.last_synced_at,
		   test_count     = excluded.test_count`,
		profileID, time.Now().UTC().Format(time.RFC3339), count)
	if err != nil {
		return fmt.Errorf("set sync state: %w", err)
	}
	return nil
}

// GetSyncState returns the last sync outcome for a profile. A profile that
// has never synced yields a zero-valued state (no error).
func (r *Repository) GetSyncState(profileID string) (SyncState, error) {
	var (
		s    SyncState
		last sql.NullString
	)
	err := r.db.QueryRow(
		`SELECT profile_id, last_synced_at, test_count FROM sync_state WHERE profile_id = ?`,
		profileID,
	).Scan(&s.ProfileID, &last, &s.TestCount)
	if errors.Is(err, sql.ErrNoRows) {
		return SyncState{ProfileID: profileID}, nil
	}
	if err != nil {
		return SyncState{}, fmt.Errorf("get sync state: %w", err)
	}
	s.LastSyncedAt = last.String
	return s, nil
}

// --- Local editing & change tracking (FR-2 / FR-1.5 / FR-12.6) ---

// EditTestField applies a local edit to a Test field, coalescing it into the
// per-field pending change for this Test and writing an audit entry. The
// editable fields are whitelisted (see editableFields). Reverting back to the
// original value drops the pending change.
//
// The audit log records every individual edit faithfully; only the
// pending_change table is coalesced.
func (r *Repository) EditTestField(profileID, testKey, field, newValue string) error {
	col, ok := editableFields[field]
	if !ok {
		return fmt.Errorf("field %q is not editable", field)
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var currentVal, baseVersion string
	readSQL := fmt.Sprintf(
		`SELECT %s, updated_at FROM test_case WHERE profile_id = ? AND jira_key = ?`,
		col,
	)
	err = tx.QueryRow(readSQL, profileID, testKey).Scan(&currentVal, &baseVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read current value: %w", err)
	}

	if currentVal == newValue {
		return nil // no-op
	}

	updateSQL := fmt.Sprintf(
		`UPDATE test_case SET %s = ? WHERE profile_id = ? AND jira_key = ?`, col,
	)
	if _, err := tx.Exec(updateSQL, newValue, profileID, testKey); err != nil {
		return fmt.Errorf("update test_case: %w", err)
	}

	if err := upsertPendingChange(
		tx, profileID, testKey, field, currentVal, newValue, baseVersion,
	); err != nil {
		return err
	}
	if err := writeAudit(
		tx, profileID, testKey, "edit-local", field, currentVal, newValue, "",
	); err != nil {
		return err
	}
	return tx.Commit()
}

// DiscardPendingChange reverts a Test field to its before_val and removes the
// pending change. An audit entry records the discard.
func (r *Repository) DiscardPendingChange(profileID string, changeID int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var entityKey, field, beforeVal, afterVal string
	err = tx.QueryRow(
		`SELECT entity_key, field, before_val, after_val FROM pending_change
		 WHERE profile_id = ? AND id = ?`,
		profileID, changeID,
	).Scan(&entityKey, &field, &beforeVal, &afterVal)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("pending change %d not found", changeID)
	}
	if err != nil {
		return fmt.Errorf("read pending change: %w", err)
	}

	col, ok := editableFields[field]
	if !ok {
		return fmt.Errorf("field %q is not editable (audit log corrupt?)", field)
	}

	revertSQL := fmt.Sprintf(
		`UPDATE test_case SET %s = ? WHERE profile_id = ? AND jira_key = ?`, col,
	)
	if _, err := tx.Exec(revertSQL, beforeVal, profileID, entityKey); err != nil {
		return fmt.Errorf("revert test_case: %w", err)
	}

	if _, err := tx.Exec(
		`DELETE FROM pending_change WHERE profile_id = ? AND id = ?`,
		profileID, changeID,
	); err != nil {
		return fmt.Errorf("delete pending: %w", err)
	}

	if err := writeAudit(
		tx, profileID, entityKey, "discard-pending", field, afterVal, beforeVal,
		fmt.Sprintf("discarded change #%d", changeID),
	); err != nil {
		return err
	}
	return tx.Commit()
}

// CommitPendingChanges deletes the given pending_change rows and writes a
// "commit" audit entry for each, in one transaction. Called by the sync
// engine after Jira accepts the corresponding PUT for that Test.
func (r *Repository) CommitPendingChanges(profileID string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	selectStmt, err := tx.Prepare(
		`SELECT entity_key, field, before_val, after_val FROM pending_change
		 WHERE profile_id = ? AND id = ?`)
	if err != nil {
		return fmt.Errorf("prepare select: %w", err)
	}
	defer selectStmt.Close()

	deleteStmt, err := tx.Prepare(
		`DELETE FROM pending_change WHERE profile_id = ? AND id = ?`)
	if err != nil {
		return fmt.Errorf("prepare delete: %w", err)
	}
	defer deleteStmt.Close()

	for _, id := range ids {
		var entityKey, field, beforeVal, afterVal string
		err := selectStmt.QueryRow(profileID, id).Scan(
			&entityKey, &field, &beforeVal, &afterVal,
		)
		if errors.Is(err, sql.ErrNoRows) {
			continue // already gone — commit stays idempotent
		}
		if err != nil {
			return fmt.Errorf("read pending %d: %w", id, err)
		}
		if _, err := deleteStmt.Exec(profileID, id); err != nil {
			return fmt.Errorf("delete pending %d: %w", id, err)
		}
		if err := writeAudit(
			tx, profileID, entityKey, "commit", field, beforeVal, afterVal, "",
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListPendingChanges returns all uncommitted local edits for a profile,
// newest first.
func (r *Repository) ListPendingChanges(profileID string) ([]PendingChange, error) {
	rows, err := r.db.Query(
		`SELECT id, entity_type, entity_key, field, before_val, after_val, base_version, created_at
		 FROM pending_change WHERE profile_id = ?
		 ORDER BY created_at DESC, id DESC`,
		profileID,
	)
	if err != nil {
		return nil, fmt.Errorf("list pending changes: %w", err)
	}
	defer rows.Close()

	out := []PendingChange{}
	for rows.Next() {
		var p PendingChange
		if err := rows.Scan(
			&p.ID, &p.EntityType, &p.EntityKey, &p.Field,
			&p.BeforeVal, &p.AfterVal, &p.BaseVersion, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListAuditEntries returns the most recent audit log entries for a profile.
// A limit ≤ 0 or > 1000 defaults to 200.
func (r *Repository) ListAuditEntries(profileID string, limit int) ([]AuditEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := r.db.Query(
		`SELECT id, occurred_at, actor, entity_type, entity_key, action, field, before_val, after_val, note
		 FROM audit_log WHERE profile_id = ?
		 ORDER BY occurred_at DESC, id DESC LIMIT ?`,
		profileID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list audit entries: %w", err)
	}
	defer rows.Close()

	out := []AuditEntry{}
	for rows.Next() {
		var a AuditEntry
		if err := rows.Scan(
			&a.ID, &a.OccurredAt, &a.Actor, &a.EntityType, &a.EntityKey,
			&a.Action, &a.Field, &a.BeforeVal, &a.AfterVal, &a.Note,
		); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// --- Helpers ---

// upsertPendingChange records (or coalesces) a pending field change. If a row
// already exists for this (profile, entity, field) the AfterVal is updated;
// if the new value matches the existing BeforeVal (i.e. the user reverted to
// the original), the row is deleted.
func upsertPendingChange(tx *sql.Tx, profileID, entityKey, field, currentVal, newValue, baseVersion string) error {
	var existingBefore string
	err := tx.QueryRow(
		`SELECT before_val FROM pending_change
		 WHERE profile_id = ? AND entity_type = 'test_case' AND entity_key = ? AND field = ?`,
		profileID, entityKey, field,
	).Scan(&existingBefore)

	now := time.Now().UTC().Format(time.RFC3339)

	if errors.Is(err, sql.ErrNoRows) {
		_, ierr := tx.Exec(
			`INSERT INTO pending_change
			   (profile_id, entity_type, entity_key, field, before_val, after_val, base_version, created_at)
			 VALUES (?, 'test_case', ?, ?, ?, ?, ?, ?)`,
			profileID, entityKey, field, currentVal, newValue, baseVersion, now,
		)
		if ierr != nil {
			return fmt.Errorf("insert pending change: %w", ierr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("read existing pending: %w", err)
	}

	if newValue == existingBefore {
		// Reverted to original — drop the pending change.
		if _, derr := tx.Exec(
			`DELETE FROM pending_change
			 WHERE profile_id = ? AND entity_type = 'test_case' AND entity_key = ? AND field = ?`,
			profileID, entityKey, field,
		); derr != nil {
			return fmt.Errorf("delete pending: %w", derr)
		}
		return nil
	}

	if _, uerr := tx.Exec(
		`UPDATE pending_change SET after_val = ?, created_at = ?
		 WHERE profile_id = ? AND entity_type = 'test_case' AND entity_key = ? AND field = ?`,
		newValue, now, profileID, entityKey, field,
	); uerr != nil {
		return fmt.Errorf("update pending: %w", uerr)
	}
	return nil
}

// writeAudit appends one row to audit_log. Called from EditTestField,
// DiscardPendingChange, and (later) commit / conflict paths.
func writeAudit(tx *sql.Tx, profileID, entityKey, action, field, beforeVal, afterVal, note string) error {
	if _, err := tx.Exec(
		`INSERT INTO audit_log
		   (profile_id, occurred_at, actor, entity_type, entity_key, action, field, before_val, after_val, note)
		 VALUES (?, ?, ?, 'test_case', ?, ?, ?, ?, ?, ?)`,
		profileID, time.Now().UTC().Format(time.RFC3339),
		currentActor(), entityKey, action, field, beforeVal, afterVal, note,
	); err != nil {
		return fmt.Errorf("audit log: %w", err)
	}
	return nil
}

// currentActor returns the OS username for the audit trail, falling back to
// "user" if it cannot be resolved.
func currentActor() string {
	u, err := user.Current()
	if err != nil || u == nil || u.Username == "" {
		return "user"
	}
	return u.Username
}

// scanner abstracts *sql.Row and *sql.Rows so scanTest serves Get and List.
type scanner interface {
	Scan(dest ...any) error
}

func scanTest(s scanner) (TestCase, error) {
	var (
		t      TestCase
		labels string
	)
	if err := s.Scan(
		&t.Key, &t.ID, &t.Summary, &t.Description,
		&t.Status, &t.Priority, &labels, &t.Updated, &t.FolderID,
	); err != nil {
		return TestCase{}, err
	}
	if labels != "" {
		t.Labels = strings.Fields(labels)
	}
	return t, nil
}
