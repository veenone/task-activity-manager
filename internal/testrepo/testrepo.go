// Package testrepo is the local repository for cached Xray Test data.
//
// It is the query layer behind the browse / search / filter experience
// (FR-11) and the write target of the sync engine (FR-1).
package testrepo

import (
	"database/sql"
	"errors"
	"fmt"
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

// Repository reads and writes cached data, scoped per profile.
type Repository struct {
	db *sql.DB
}

// NewRepository returns a repository backed by the given store.
func NewRepository(s *store.Store) *Repository {
	return &Repository{db: s.DB()}
}

// UpsertTests inserts or updates a batch of Tests in one transaction.
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

// SetSyncState records that a profile finished syncing with testCount Tests.
func (r *Repository) SetSyncState(profileID string, testCount int) error {
	_, err := r.db.Exec(
		`INSERT INTO sync_state (profile_id, last_synced_at, test_count) VALUES (?, ?, ?)
		 ON CONFLICT(profile_id) DO UPDATE SET
		   last_synced_at = excluded.last_synced_at,
		   test_count     = excluded.test_count`,
		profileID, time.Now().UTC().Format(time.RFC3339), testCount)
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
