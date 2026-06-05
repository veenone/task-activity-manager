// Package testrepo is the local repository for cached Xray Test data.
//
// It is the query layer behind the browse / search / filter experience
// (FR-11), the write target of the sync engine (FR-1), and the home of the
// local change-tracking and audit-log machinery (FR-1.5 / FR-1.6 / FR-12.6).
package testrepo

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os/user"
	"sort"
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

// Container is a cached Xray Test Set, Test Plan or Test Execution (FR-1.3).
// Kind is one of "testset" / "testplan" / "testexec" — the three share a shape
// and differ only in how they relate to Tests.
type Container struct {
	Key     string `json:"key"`
	Kind    string `json:"kind"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
}

// ContainerLink is one Test's membership in a Container. RunStatus carries the
// Test Run result for execution memberships and is empty for sets / plans.
type ContainerLink struct {
	ContainerKey string `json:"containerKey"`
	TestKey      string `json:"testKey"`
	RunStatus    string `json:"runStatus"`
}

// ContainerMembership is a Container a Test belongs to, joined with that Test's
// run status — what the detail panel lists under Test Sets / Plans /
// Executions.
type ContainerMembership struct {
	Key       string `json:"key"`
	Kind      string `json:"kind"`
	Summary   string `json:"summary"`
	Status    string `json:"status"`
	RunStatus string `json:"runStatus"`
}

// Step is one cached Xray Test Step (FR-2.5). XrayID is Xray's per-step
// identifier — kept on the row so the future edit-steps API can target
// each step individually without us having to rebuild the list.
type Step struct {
	XrayID   string `json:"xrayId"`
	Index    int    `json:"index"`
	Action   string `json:"action"`
	Data     string `json:"data"`
	Expected string `json:"expected"`
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

// Statistics is a per-profile rollup of the local Test cache for the dashboard
// (FR-9). Everything here is computed from SQLite with no Jira round-trips
// (FR-9.5). Test type and execution coverage (FR-9.3/9.4) are absent because
// the sync doesn't store them yet.
type Statistics struct {
	Total          int      `json:"total"`
	PendingChanges int      `json:"pendingChanges"`
	ByStatus       []Bucket `json:"byStatus"`
	ByPriority     []Bucket `json:"byPriority"`
	ByLabel        []Bucket `json:"byLabel"`
	ByFolder       []Bucket `json:"byFolder"`
	UpdatedTrend   []Bucket `json:"updatedTrend"`
}

// Bucket is one (label, count) pair in a distribution — also used for the
// month-keyed update trend.
type Bucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
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

// columnForField returns the test_case column corresponding to a field
// name. It includes 'status' — which isn't free-text editable (status is
// changed via TransitionTest, not EditTestField) but is still tracked in
// pending_change rows and needs a column lookup for the discard / sync
// paths.
func columnForField(field string) (string, bool) {
	if c, ok := editableFields[field]; ok {
		return c, true
	}
	if field == "status" {
		return "status", true
	}
	return "", false
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
// For Tests that have local pending edits, the ON CONFLICT clause preserves
// the user's edited value for each editable field that has a pending row —
// only fields without a pending change are overwritten by the incoming
// sync. base_version on the pending change stays untouched, so commit-time
// conflict detection (FR-1.4, next slice) still has the original watermark
// to compare against.
func (r *Repository) UpsertTests(profileID string, tests []TestCase) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Each editable field's UPDATE branch checks for a pending change on
	// that (profile, test, field) and, if one exists, keeps the existing
	// local value instead of overwriting from the incoming sync.
	stmt, err := tx.Prepare(
		`INSERT INTO test_case
		   (profile_id, jira_key, jira_id, summary, description, status, priority, labels, updated_at, folder_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(profile_id, jira_key) DO UPDATE SET
		   jira_id     = excluded.jira_id,
		   summary     = CASE WHEN EXISTS (
		       SELECT 1 FROM pending_change
		       WHERE pending_change.profile_id  = excluded.profile_id
		         AND pending_change.entity_type = 'test_case'
		         AND pending_change.entity_key  = excluded.jira_key
		         AND pending_change.field       = 'summary'
		     ) THEN test_case.summary ELSE excluded.summary END,
		   description = CASE WHEN EXISTS (
		       SELECT 1 FROM pending_change
		       WHERE pending_change.profile_id  = excluded.profile_id
		         AND pending_change.entity_type = 'test_case'
		         AND pending_change.entity_key  = excluded.jira_key
		         AND pending_change.field       = 'description'
		     ) THEN test_case.description ELSE excluded.description END,
		   status      = CASE WHEN EXISTS (
		       SELECT 1 FROM pending_change
		       WHERE pending_change.profile_id  = excluded.profile_id
		         AND pending_change.entity_type = 'test_case'
		         AND pending_change.entity_key  = excluded.jira_key
		         AND pending_change.field       = 'status'
		     ) THEN test_case.status ELSE excluded.status END,
		   priority    = CASE WHEN EXISTS (
		       SELECT 1 FROM pending_change
		       WHERE pending_change.profile_id  = excluded.profile_id
		         AND pending_change.entity_type = 'test_case'
		         AND pending_change.entity_key  = excluded.jira_key
		         AND pending_change.field       = 'priority'
		     ) THEN test_case.priority ELSE excluded.priority END,
		   labels      = CASE WHEN EXISTS (
		       SELECT 1 FROM pending_change
		       WHERE pending_change.profile_id  = excluded.profile_id
		         AND pending_change.entity_type = 'test_case'
		         AND pending_change.entity_key  = excluded.jira_key
		         AND pending_change.field       = 'labels'
		     ) THEN test_case.labels ELSE excluded.labels END,
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

// UpsertContainers inserts or updates a batch of Test Sets / Plans /
// Executions (FR-1.3).
func (r *Repository) UpsertContainers(profileID string, containers []Container) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(
		`INSERT INTO test_container (profile_id, jira_key, kind, summary, status)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(profile_id, jira_key) DO UPDATE SET
		   kind    = excluded.kind,
		   summary = excluded.summary,
		   status  = excluded.status`)
	if err != nil {
		return fmt.Errorf("prepare upsert container: %w", err)
	}
	defer stmt.Close()

	for _, c := range containers {
		if _, err := stmt.Exec(profileID, c.Key, c.Kind, c.Summary, c.Status); err != nil {
			return fmt.Errorf("upsert container %s: %w", c.Key, err)
		}
	}
	return tx.Commit()
}

// ReplaceAllContainerLinks wipes a profile's Test-to-Container memberships and
// rewrites them from the provided list, so memberships removed in Jira
// actually disappear on resync (mirrors ReplaceAllTestPreconditions).
func (r *Repository) ReplaceAllContainerLinks(profileID string, links []ContainerLink) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(
		`DELETE FROM test_container_test WHERE profile_id = ?`, profileID,
	); err != nil {
		return fmt.Errorf("clear container links: %w", err)
	}

	if len(links) == 0 {
		return tx.Commit()
	}

	stmt, err := tx.Prepare(
		`INSERT INTO test_container_test (profile_id, container_key, test_key, run_status)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(profile_id, container_key, test_key) DO UPDATE SET
		   run_status = excluded.run_status`)
	if err != nil {
		return fmt.Errorf("prepare insert container link: %w", err)
	}
	defer stmt.Close()

	for _, l := range links {
		if _, err := stmt.Exec(profileID, l.ContainerKey, l.TestKey, l.RunStatus); err != nil {
			return fmt.Errorf("link %s -> %s: %w", l.ContainerKey, l.TestKey, err)
		}
	}
	return tx.Commit()
}

// ListContainersForTest returns the Test Sets, Plans and Executions a Test
// belongs to, ordered by kind then key, with the run status for execution
// memberships.
func (r *Repository) ListContainersForTest(profileID, testKey string) ([]ContainerMembership, error) {
	rows, err := r.db.Query(
		`SELECT c.jira_key, c.kind, c.summary, c.status, l.run_status
		 FROM test_container_test l
		 JOIN test_container c
		   ON c.profile_id = l.profile_id AND c.jira_key = l.container_key
		 WHERE l.profile_id = ? AND l.test_key = ?
		 ORDER BY c.kind, c.jira_key`,
		profileID, testKey)
	if err != nil {
		return nil, fmt.Errorf("list containers for test: %w", err)
	}
	defer rows.Close()

	out := []ContainerMembership{}
	for rows.Next() {
		var m ContainerMembership
		if err := rows.Scan(&m.Key, &m.Kind, &m.Summary, &m.Status, &m.RunStatus); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// buildTestFilter builds the WHERE clause + args for a Test query. Shared
// by ListTests and ListMatchingKeys so both see the same filter semantics
// — when the user clicks "select all matching", they get the exact set
// the grid is showing across all pages.
func buildTestFilter(profileID string, q Query) (string, []any) {
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
	return "WHERE " + strings.Join(where, " AND "), args
}

// SetTestSteps replaces the cached Step list for one Test (FR-2.5). The
// whole list is rewritten on each call — Xray returns steps as an ordered
// array and we mirror that semantic rather than trying to diff. Pass an
// empty slice to clear the cache for a Test.
func (r *Repository) SetTestSteps(profileID, testKey string, steps []Step) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(
		`DELETE FROM test_step WHERE profile_id = ? AND test_key = ?`,
		profileID, testKey,
	); err != nil {
		return fmt.Errorf("clear steps: %w", err)
	}
	for _, s := range steps {
		if _, err := tx.Exec(
			`INSERT INTO test_step
			   (profile_id, test_key, xray_id, idx, action, data, expected)
			   VALUES (?, ?, ?, ?, ?, ?, ?)`,
			profileID, testKey, s.XrayID, s.Index, s.Action, s.Data, s.Expected,
		); err != nil {
			return fmt.Errorf("insert step: %w", err)
		}
	}
	return tx.Commit()
}

// ListTestSteps returns the cached Steps for a Test in index order.
// Returns an empty slice (not an error) for tests with no cached steps —
// the caller is responsible for deciding whether to fetch from Jira.
func (r *Repository) ListTestSteps(profileID, testKey string) ([]Step, error) {
	rows, err := r.db.Query(
		`SELECT xray_id, idx, action, data, expected
		 FROM test_step
		 WHERE profile_id = ? AND test_key = ?
		 ORDER BY idx`,
		profileID, testKey,
	)
	if err != nil {
		return nil, fmt.Errorf("list test steps: %w", err)
	}
	defer rows.Close()

	out := []Step{}
	for rows.Next() {
		var s Step
		if err := rows.Scan(&s.XrayID, &s.Index, &s.Action, &s.Data, &s.Expected); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListTests returns a filtered, sorted, paginated page of Tests for a profile.
// A FolderID filter matches the folder itself plus any descendants so
// selecting a category in the tree shows everything beneath it.
func (r *Repository) ListTests(profileID string, q Query) (Page, error) {
	whereSQL, args := buildTestFilter(profileID, q)

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

// ListMatchingKeys returns every Test key matching the query's filter,
// ignoring pagination and sort. The bulk toolbar uses this to honor
// "select all 4,812 matching" without forcing the user to paginate
// through 96 pages (FR-3.1).
//
// Order is unspecified — the frontend treats the result as an unordered
// set when building the selection.
func (r *Repository) ListMatchingKeys(profileID string, q Query) ([]string, error) {
	whereSQL, args := buildTestFilter(profileID, q)
	rows, err := r.db.Query(
		"SELECT jira_key FROM test_case "+whereSQL, args...,
	)
	if err != nil {
		return nil, fmt.Errorf("list matching keys: %w", err)
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
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

// GetStatistics computes the dashboard rollup for a profile (FR-9) in a single
// table scan plus one count query — fast enough to recompute on demand even at
// 50k Tests. Status / priority distributions are returned in full; labels and
// folders are capped to the top buckets; the trend is the most recent months
// keyed by the Test's last-updated month.
func (r *Repository) GetStatistics(profileID string) (Statistics, error) {
	stats := Statistics{
		ByStatus:     []Bucket{},
		ByPriority:   []Bucket{},
		ByLabel:      []Bucket{},
		ByFolder:     []Bucket{},
		UpdatedTrend: []Bucket{},
	}

	rows, err := r.db.Query(
		`SELECT status, priority, labels, folder_id, updated_at
		 FROM test_case WHERE profile_id = ?`, profileID)
	if err != nil {
		return stats, fmt.Errorf("read tests for stats: %w", err)
	}
	defer rows.Close()

	statusCounts := map[string]int{}
	priorityCounts := map[string]int{}
	labelCounts := map[string]int{}
	folderCounts := map[string]int{}
	trendCounts := map[string]int{}

	for rows.Next() {
		var status, priority, labels, folderID, updated string
		if err := rows.Scan(&status, &priority, &labels, &folderID, &updated); err != nil {
			return stats, err
		}
		stats.Total++
		statusCounts[blankAs(status, "(none)")]++
		priorityCounts[blankAs(priority, "(none)")]++
		for _, l := range strings.Fields(labels) {
			labelCounts[l]++
		}
		folderCounts[topFolder(folderID)]++
		if m := monthOf(updated); m != "" {
			trendCounts[m]++
		}
	}
	if err := rows.Err(); err != nil {
		return stats, err
	}

	stats.ByStatus = topBuckets(statusCounts, 0)
	stats.ByPriority = topBuckets(priorityCounts, 0)
	stats.ByLabel = topBuckets(labelCounts, 12)
	stats.ByFolder = topBuckets(folderCounts, 12)
	stats.UpdatedTrend = recentMonths(trendCounts, 12)

	if err := r.db.QueryRow(
		`SELECT COUNT(*) FROM pending_change WHERE profile_id = ?`, profileID,
	).Scan(&stats.PendingChanges); err != nil {
		return stats, fmt.Errorf("count pending for stats: %w", err)
	}
	return stats, nil
}

// blankAs returns def when s is empty, else s — so empty status / priority
// values show as a labelled bucket rather than a blank one.
func blankAs(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// topFolder reduces a folder path ("/Authentication/Login") to its top-level
// category ("Authentication") so the folder distribution stays readable.
func topFolder(id string) string {
	if id == "" {
		return "(none)"
	}
	s := strings.TrimPrefix(id, "/")
	if i := strings.Index(s, "/"); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return "(none)"
	}
	return s
}

// monthOf extracts the "YYYY-MM" prefix from a Jira timestamp. Both the demo
// format ("2006-01-02T15:04:05.000-0700") and RFC 3339 start with the date, so
// a prefix slice is enough — no full parse needed.
func monthOf(updated string) string {
	if len(updated) >= 7 && updated[4] == '-' {
		return updated[:7]
	}
	return ""
}

// topBuckets turns a count map into buckets sorted by count descending (ties
// broken by label) and, when limit > 0, keeps only the top `limit`.
func topBuckets(counts map[string]int, limit int) []Bucket {
	out := make([]Bucket, 0, len(counts))
	for k, v := range counts {
		out = append(out, Bucket{Label: k, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Label < out[j].Label
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// recentMonths returns up to n trend buckets in chronological order, keeping
// the most recent months (YYYY-MM sorts lexicographically by time).
func recentMonths(counts map[string]int, n int) []Bucket {
	months := make([]string, 0, len(counts))
	for k := range counts {
		months = append(months, k)
	}
	sort.Strings(months)
	if n > 0 && len(months) > n {
		months = months[len(months)-n:]
	}
	out := make([]Bucket, 0, len(months))
	for _, m := range months {
		out = append(out, Bucket{Label: m, Count: counts[m]})
	}
	return out
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
		tx, profileID, entityTestCase, testKey, field, currentVal, newValue, baseVersion,
	); err != nil {
		return err
	}
	if err := writeAudit(
		tx, profileID, entityTestCase, testKey,
		"edit-local", field, currentVal, newValue, "",
	); err != nil {
		return err
	}
	return tx.Commit()
}

// stepFields whitelists which Step columns can be edited via
// EditTestStepField. The map value is the on-disk column name in test_step.
var stepFields = map[string]string{
	"action":   "action",
	"data":     "data",
	"expected": "expected",
}

// EditTestStepField applies a local edit to one field of one Test Step
// (FR-2.5). The change is queued in pending_change with entity_type =
// "test_step" and entity_key = "<testKey>:<xrayID>" so the commit path
// can route step updates to /rest/raven/2.0/api/test/{key}/steps/{stepId}
// while keeping the same coalesce / discard machinery as test_case fields.
func (r *Repository) EditTestStepField(profileID, testKey, xrayID, field, newValue string) error {
	col, ok := stepFields[field]
	if !ok {
		return fmt.Errorf("step field %q is not editable", field)
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// The conflict base_version we capture is the parent Test's updated_at —
	// step edits without parallel field edits still want to conflict-check
	// against the same remote "updated" the syncer reads.
	var currentVal, baseVersion string
	readSQL := fmt.Sprintf(
		`SELECT s.%s, t.updated_at
		   FROM test_step s
		   JOIN test_case t
		     ON t.profile_id = s.profile_id AND t.jira_key = s.test_key
		   WHERE s.profile_id = ? AND s.test_key = ? AND s.xray_id = ?`,
		col,
	)
	err = tx.QueryRow(readSQL, profileID, testKey, xrayID).Scan(&currentVal, &baseVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read current step value: %w", err)
	}
	if currentVal == newValue {
		return nil
	}

	updateSQL := fmt.Sprintf(
		`UPDATE test_step SET %s = ? WHERE profile_id = ? AND test_key = ? AND xray_id = ?`, col,
	)
	if _, err := tx.Exec(updateSQL, newValue, profileID, testKey, xrayID); err != nil {
		return fmt.Errorf("update test_step: %w", err)
	}

	ek := stepEntityKey(testKey, xrayID)
	// A not-yet-committed step has no remote id to PUT against, so its edits
	// fold into the queued add's JSON instead of becoming standalone
	// step-edit rows. Reverting such an edit doesn't drop anything — the add
	// row stays until the step is committed or discarded.
	folded, err := foldStepEditIntoAdd(tx, profileID, ek, field, newValue)
	if err != nil {
		return err
	}
	if !folded {
		if err := upsertPendingChange(
			tx, profileID, entityTestStep, ek, field, currentVal, newValue, baseVersion,
		); err != nil {
			return err
		}
	}
	if err := writeAudit(
		tx, profileID, entityTestStep, ek,
		"edit-local", field, currentVal, newValue, "",
	); err != nil {
		return err
	}
	return tx.Commit()
}

// foldStepEditIntoAdd updates the field inside a pending test_step_add row's
// JSON snapshot, returning false (without touching anything) when no add row
// exists for the step. This keeps a brand-new step to a single pending row so
// the commit POST carries the latest content.
func foldStepEditIntoAdd(tx *sql.Tx, profileID, ek, field, newValue string) (bool, error) {
	var afterVal string
	err := tx.QueryRow(
		`SELECT after_val FROM pending_change
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = 'step'`,
		profileID, entityTestStepAdd, ek,
	).Scan(&afterVal)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read pending add: %w", err)
	}

	var s Step
	if err := json.Unmarshal([]byte(afterVal), &s); err != nil {
		return false, fmt.Errorf("decode pending add: %w", err)
	}
	switch field {
	case "action":
		s.Action = newValue
	case "data":
		s.Data = newValue
	case "expected":
		s.Expected = newValue
	}
	snapshot, err := json.Marshal(s)
	if err != nil {
		return false, fmt.Errorf("marshal pending add: %w", err)
	}
	if _, err := tx.Exec(
		`UPDATE pending_change SET after_val = ?, created_at = ?
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = 'step'`,
		string(snapshot), time.Now().UTC().Format(time.RFC3339),
		profileID, entityTestStepAdd, ek,
	); err != nil {
		return false, fmt.Errorf("update pending add: %w", err)
	}
	return true, nil
}

// DeleteTestStep queues a Test Step for deletion (FR-2.5). The step is
// hidden from the local list immediately; the actual DELETE call to Xray
// fires at commit time. Discarding the pending row restores the local
// step from the JSON snapshot stashed in before_val.
//
// Parent test_case.updated_at is captured as base_version so the conflict
// pre-check at commit time uses the same timestamp the field-edit path
// does — a delete and an unrelated remote update on the same Test still
// surface as a conflict.
func (r *Repository) DeleteTestStep(profileID, testKey, xrayID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var s Step
	err = tx.QueryRow(
		`SELECT xray_id, idx, action, data, expected
		   FROM test_step WHERE profile_id = ? AND test_key = ? AND xray_id = ?`,
		profileID, testKey, xrayID,
	).Scan(&s.XrayID, &s.Index, &s.Action, &s.Data, &s.Expected)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read step: %w", err)
	}

	snapshot, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	ek := stepEntityKey(testKey, xrayID)

	if _, err := tx.Exec(
		`DELETE FROM test_step WHERE profile_id = ? AND test_key = ? AND xray_id = ?`,
		profileID, testKey, xrayID,
	); err != nil {
		return fmt.Errorf("delete test_step: %w", err)
	}

	// Deleting a step that was only ever added locally cancels the add — the
	// step never reached Xray, so there's nothing to delete remotely. Drop
	// the queued add (its folded edits go with it) instead of recording a
	// delete.
	var addID int64
	err = tx.QueryRow(
		`SELECT id FROM pending_change
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = 'step'`,
		profileID, entityTestStepAdd, ek,
	).Scan(&addID)
	if err == nil {
		if _, derr := tx.Exec(
			`DELETE FROM pending_change WHERE profile_id = ? AND id = ?`,
			profileID, addID,
		); derr != nil {
			return fmt.Errorf("cancel pending add: %w", derr)
		}
		if aerr := writeAudit(
			tx, profileID, entityTestStepAdd, ek,
			"add-cancelled", "step", string(snapshot), "", "",
		); aerr != nil {
			return aerr
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("probe pending add: %w", err)
	}

	// Deleting a committed step: any queued field edits on it are superseded
	// by the delete — drop them so the commit doesn't PUT a step it's about
	// to DELETE.
	if _, err := tx.Exec(
		`DELETE FROM pending_change
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
		profileID, entityTestStep, ek,
	); err != nil {
		return fmt.Errorf("clear superseded step edits: %w", err)
	}

	var baseVersion string
	if err := tx.QueryRow(
		`SELECT updated_at FROM test_case WHERE profile_id = ? AND jira_key = ?`,
		profileID, testKey,
	).Scan(&baseVersion); err != nil {
		return fmt.Errorf("read parent updated_at: %w", err)
	}
	if err := upsertPendingChange(
		tx, profileID, entityTestStepDelete, ek, "step",
		string(snapshot), "1", baseVersion,
	); err != nil {
		return err
	}
	if err := writeAudit(
		tx, profileID, entityTestStepDelete, ek,
		"delete-local", "step", string(snapshot), "1", "",
	); err != nil {
		return err
	}
	return tx.Commit()
}

// AddTestStep appends a new Step to a Test locally and queues it for creation
// in Xray on commit (FR-2.5). The new step gets a temporary xray_id ("new-N")
// because Xray only assigns the real one when the POST lands; the commit path
// renames the cached row to the real id afterwards. The step content travels
// in the pending row's after_val as JSON so the commit POST has everything it
// needs, and later field edits on a not-yet-committed step fold into that same
// JSON (see EditTestStepField) rather than spawning step-edit rows that would
// PUT to a non-existent remote step.
//
// Returns the created Step so the caller can render it without a re-fetch.
func (r *Repository) AddTestStep(profileID, testKey, action, data, expected string) (Step, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return Step{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var baseVersion string
	err = tx.QueryRow(
		`SELECT updated_at FROM test_case WHERE profile_id = ? AND jira_key = ?`,
		profileID, testKey,
	).Scan(&baseVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return Step{}, ErrNotFound
	}
	if err != nil {
		return Step{}, fmt.Errorf("read parent updated_at: %w", err)
	}

	var nextIdx int
	if err := tx.QueryRow(
		`SELECT COALESCE(MAX(idx), 0) + 1 FROM test_step WHERE profile_id = ? AND test_key = ?`,
		profileID, testKey,
	).Scan(&nextIdx); err != nil {
		return Step{}, fmt.Errorf("compute next step index: %w", err)
	}

	tempID, err := nextTempStepID(tx, profileID, testKey)
	if err != nil {
		return Step{}, err
	}

	s := Step{XrayID: tempID, Index: nextIdx, Action: action, Data: data, Expected: expected}
	if _, err := tx.Exec(
		`INSERT INTO test_step (profile_id, test_key, xray_id, idx, action, data, expected)
		   VALUES (?, ?, ?, ?, ?, ?, ?)`,
		profileID, testKey, s.XrayID, s.Index, s.Action, s.Data, s.Expected,
	); err != nil {
		return Step{}, fmt.Errorf("insert test_step: %w", err)
	}

	snapshot, err := json.Marshal(s)
	if err != nil {
		return Step{}, fmt.Errorf("marshal step: %w", err)
	}
	ek := stepEntityKey(testKey, tempID)
	if err := upsertPendingChange(
		tx, profileID, entityTestStepAdd, ek, "step", "", string(snapshot), baseVersion,
	); err != nil {
		return Step{}, err
	}
	if err := writeAudit(
		tx, profileID, entityTestStepAdd, ek,
		"add-local", "step", "", string(snapshot), "",
	); err != nil {
		return Step{}, err
	}
	if err := tx.Commit(); err != nil {
		return Step{}, fmt.Errorf("commit add step: %w", err)
	}
	return s, nil
}

// nextTempStepID returns a step xray_id of the form "new-N" not already used
// by another step on this Test. New steps need a placeholder id until Xray
// assigns the real one at commit time.
func nextTempStepID(tx *sql.Tx, profileID, testKey string) (string, error) {
	for n := 1; ; n++ {
		id := fmt.Sprintf("new-%d", n)
		var one int
		err := tx.QueryRow(
			`SELECT 1 FROM test_step WHERE profile_id = ? AND test_key = ? AND xray_id = ?`,
			profileID, testKey, id,
		).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return id, nil
		}
		if err != nil {
			return "", fmt.Errorf("probe temp step id: %w", err)
		}
	}
}

// RenameTestStepID rewrites a cached step's xray_id, used by the commit path
// to swap a "new-N" placeholder for the real id Xray returned from the create
// POST. A no-op when newID is empty (Xray didn't return one) or unchanged.
func (r *Repository) RenameTestStepID(profileID, testKey, oldID, newID string) error {
	if newID == "" || newID == oldID {
		return nil
	}
	if _, err := r.db.Exec(
		`UPDATE test_step SET xray_id = ?
		   WHERE profile_id = ? AND test_key = ? AND xray_id = ?`,
		newID, profileID, testKey, oldID,
	); err != nil {
		return fmt.Errorf("rename step id: %w", err)
	}
	return nil
}

// ReorderTestSteps records a new ordering for a Test's steps (FR-2.5). The
// caller passes the full set of step xray_ids in their new order; the order is
// stored as a single test-level pending row (before/after id-lists) rather
// than per-step edits, because reordering is a property of the list, not of
// any one step. Xray gets the new positions on commit.
//
// orderedXrayIDs must be exactly the current step set, just permuted — adding
// or removing a step is the job of AddTestStep / DeleteTestStep, not this.
// Reordering back to the original order drops the pending row.
func (r *Repository) ReorderTestSteps(profileID, testKey string, orderedXrayIDs []string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.Query(
		`SELECT xray_id FROM test_step WHERE profile_id = ? AND test_key = ? ORDER BY idx`,
		profileID, testKey,
	)
	if err != nil {
		return fmt.Errorf("read current step order: %w", err)
	}
	currentIDs := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		currentIDs = append(currentIDs, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	if len(currentIDs) == 0 {
		return fmt.Errorf("test %s has no steps to reorder", testKey)
	}
	if !samePermutation(currentIDs, orderedXrayIDs) {
		return fmt.Errorf("reorder set must match the current steps exactly")
	}
	if equalOrder(currentIDs, orderedXrayIDs) {
		return nil // no-op
	}

	var baseVersion string
	if err := tx.QueryRow(
		`SELECT updated_at FROM test_case WHERE profile_id = ? AND jira_key = ?`,
		profileID, testKey,
	).Scan(&baseVersion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("read parent updated_at: %w", err)
	}

	if err := applyStepOrder(tx, profileID, testKey, orderedXrayIDs); err != nil {
		return err
	}

	beforeJSON, err := json.Marshal(currentIDs)
	if err != nil {
		return fmt.Errorf("marshal current order: %w", err)
	}
	afterJSON, err := json.Marshal(orderedXrayIDs)
	if err != nil {
		return fmt.Errorf("marshal new order: %w", err)
	}
	if err := upsertPendingChange(
		tx, profileID, entityTestStepOrder, testKey, "order",
		string(beforeJSON), string(afterJSON), baseVersion,
	); err != nil {
		return err
	}
	if err := writeAudit(
		tx, profileID, entityTestStepOrder, testKey,
		"reorder-local", "order", string(beforeJSON), string(afterJSON), "",
	); err != nil {
		return err
	}
	return tx.Commit()
}

// samePermutation reports whether a and b contain exactly the same ids
// (ignoring order). Used to reject a reorder that secretly adds or drops a
// step.
func samePermutation(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, x := range a {
		counts[x]++
	}
	for _, x := range b {
		counts[x]--
		if counts[x] < 0 {
			return false
		}
	}
	return true
}

// equalOrder reports whether a and b are identical in order.
func equalOrder(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// applyStepOrder rewrites the idx column so steps fall in the order given by
// orderedXrayIDs. Shared by the reorder-discard path. Caller supplies an open
// transaction.
func applyStepOrder(tx *sql.Tx, profileID, testKey string, orderedXrayIDs []string) error {
	for i, id := range orderedXrayIDs {
		if _, err := tx.Exec(
			`UPDATE test_step SET idx = ? WHERE profile_id = ? AND test_key = ? AND xray_id = ?`,
			i+1, profileID, testKey, id,
		); err != nil {
			return fmt.Errorf("restore step index: %w", err)
		}
	}
	return nil
}

// DiscardPendingChange reverts a Test field to its before_val and removes the
// pending change. An audit entry records the discard.
func (r *Repository) DiscardPendingChange(profileID string, changeID int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var entityType, entityKey, field, beforeVal, afterVal string
	err = tx.QueryRow(
		`SELECT entity_type, entity_key, field, before_val, after_val FROM pending_change
		 WHERE profile_id = ? AND id = ?`,
		profileID, changeID,
	).Scan(&entityType, &entityKey, &field, &beforeVal, &afterVal)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("pending change %d not found", changeID)
	}
	if err != nil {
		return fmt.Errorf("read pending change: %w", err)
	}

	switch entityType {
	case entityTestCase:
		col, ok := columnForField(field)
		if !ok {
			return fmt.Errorf("field %q is not editable (audit log corrupt?)", field)
		}
		revertSQL := fmt.Sprintf(
			`UPDATE test_case SET %s = ? WHERE profile_id = ? AND jira_key = ?`, col,
		)
		if _, err := tx.Exec(revertSQL, beforeVal, profileID, entityKey); err != nil {
			return fmt.Errorf("revert test_case: %w", err)
		}
	case entityTestStep:
		col, ok := stepFields[field]
		if !ok {
			return fmt.Errorf("step field %q is not editable (audit log corrupt?)", field)
		}
		testKey, xrayID, ok := parseStepEntityKey(entityKey)
		if !ok {
			return fmt.Errorf("malformed step entity_key %q", entityKey)
		}
		revertSQL := fmt.Sprintf(
			`UPDATE test_step SET %s = ?
			   WHERE profile_id = ? AND test_key = ? AND xray_id = ?`, col,
		)
		if _, err := tx.Exec(revertSQL, beforeVal, profileID, testKey, xrayID); err != nil {
			return fmt.Errorf("revert test_step: %w", err)
		}
	case entityTestStepDelete:
		testKey, _, ok := parseStepEntityKey(entityKey)
		if !ok {
			return fmt.Errorf("malformed step entity_key %q", entityKey)
		}
		var snap Step
		if err := json.Unmarshal([]byte(beforeVal), &snap); err != nil {
			return fmt.Errorf("decode step snapshot: %w", err)
		}
		if _, err := tx.Exec(
			`INSERT INTO test_step (profile_id, test_key, xray_id, idx, action, data, expected)
			   VALUES (?, ?, ?, ?, ?, ?, ?)`,
			profileID, testKey, snap.XrayID, snap.Index, snap.Action, snap.Data, snap.Expected,
		); err != nil {
			return fmt.Errorf("restore test_step: %w", err)
		}
	case entityTestStepAdd:
		// Discarding an add removes the locally-created step entirely — it
		// never existed remotely, so there's nothing to restore.
		testKey, xrayID, ok := parseStepEntityKey(entityKey)
		if !ok {
			return fmt.Errorf("malformed step entity_key %q", entityKey)
		}
		if _, err := tx.Exec(
			`DELETE FROM test_step WHERE profile_id = ? AND test_key = ? AND xray_id = ?`,
			profileID, testKey, xrayID,
		); err != nil {
			return fmt.Errorf("remove added test_step: %w", err)
		}
	case entityTestStepOrder:
		// entity_key is the test key itself; restore the idx column from the
		// original order snapshot in before_val.
		var order []string
		if err := json.Unmarshal([]byte(beforeVal), &order); err != nil {
			return fmt.Errorf("decode order snapshot: %w", err)
		}
		if err := applyStepOrder(tx, profileID, entityKey, order); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown entity_type %q", entityType)
	}

	if _, err := tx.Exec(
		`DELETE FROM pending_change WHERE profile_id = ? AND id = ?`,
		profileID, changeID,
	); err != nil {
		return fmt.Errorf("delete pending: %w", err)
	}

	if err := writeAudit(
		tx, profileID, entityType, entityKey,
		"discard-pending", field, afterVal, beforeVal,
		fmt.Sprintf("discarded change #%d", changeID),
	); err != nil {
		return err
	}
	return tx.Commit()
}

// TransitionTest queues a workflow transition on a Test (FR-4.2). The
// resulting status is recorded as a pending change on the "status" field;
// commit posts to /rest/api/2/issue/{key}/transitions rather than PUTting
// the status field.
//
// The caller is responsible for picking a targetStatus that's reachable
// from the Test's current status — the UI does this by listing available
// transitions via GetTransitions before invoking this method.
//
// TODO(xtm): multi-step transitions (A->B->C locally) coalesce to a single
// pending row A->C, which needs a direct A->C transition to exist on the
// remote workflow at commit time. A future slice could record the
// transition path instead of just the target status.
func (r *Repository) TransitionTest(profileID, testKey, targetStatus string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var currentVal, baseVersion string
	err = tx.QueryRow(
		`SELECT status, updated_at FROM test_case WHERE profile_id = ? AND jira_key = ?`,
		profileID, testKey,
	).Scan(&currentVal, &baseVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read current status: %w", err)
	}
	if currentVal == targetStatus {
		return nil
	}
	if _, err := tx.Exec(
		`UPDATE test_case SET status = ? WHERE profile_id = ? AND jira_key = ?`,
		targetStatus, profileID, testKey,
	); err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	if err := upsertPendingChange(
		tx, profileID, entityTestCase, testKey, "status", currentVal, targetStatus, baseVersion,
	); err != nil {
		return err
	}
	if err := writeAudit(
		tx, profileID, entityTestCase, testKey,
		"transition-local", "status",
		currentVal, targetStatus, "",
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
		`SELECT entity_type, entity_key, field, before_val, after_val FROM pending_change
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
		var entityType, entityKey, field, beforeVal, afterVal string
		err := selectStmt.QueryRow(profileID, id).Scan(
			&entityType, &entityKey, &field, &beforeVal, &afterVal,
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
			tx, profileID, entityType, entityKey,
			"commit", field, beforeVal, afterVal, "",
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

// --- Bulk operations (FR-3) ---

// BulkEdit describes a single field-level operation to apply to a set of
// Tests. Operations:
//
//   - "set":          replace the field's value with op.Value (any editable field)
//   - "append":       append op.Value to the existing value with a newline
//     (description only)
//   - "add_label":    add op.Value as a label if not already present
//   - "remove_label": remove op.Value from the labels list if present
//
// For label operations the Field is implied to be "labels" regardless of
// what the caller sets.
type BulkEdit struct {
	Operation string `json:"operation"`
	Field     string `json:"field"`
	Value     string `json:"value"`
}

// BulkEditResult reports the outcome of a bulk operation, per Test.
type BulkEditResult struct {
	Succeeded []string      `json:"succeeded"`
	Failed    []BulkFailure `json:"failed"`
}

// BulkFailure is one Test the bulk operation could not be applied to.
type BulkFailure struct {
	TestKey string `json:"testKey"`
	Error   string `json:"error"`
}

// BulkEditTests applies a single field-level operation to a batch of Tests,
// queuing a pending change for each modified Test (FR-3.2 / FR-3.3 / FR-3.7).
// Each Test is processed in its own transaction (via EditTestField) so one
// failure doesn't block the others. No-op edits — for example, add_label
// when the label is already present — are reported as succeeded.
func (r *Repository) BulkEditTests(profileID string, testKeys []string, op BulkEdit) (BulkEditResult, error) {
	result := BulkEditResult{
		Succeeded: []string{},
		Failed:    []BulkFailure{},
	}

	field, err := resolveBulkField(op)
	if err != nil {
		return result, fmt.Errorf("bulk edit: %w", err)
	}
	col, ok := editableFields[field]
	if !ok {
		return result, fmt.Errorf("bulk edit: field %q is not editable", field)
	}

	readSQL := fmt.Sprintf(
		`SELECT %s FROM test_case WHERE profile_id = ? AND jira_key = ?`, col,
	)

	for _, key := range testKeys {
		var current string
		err := r.db.QueryRow(readSQL, profileID, key).Scan(&current)
		if errors.Is(err, sql.ErrNoRows) {
			result.Failed = append(result.Failed, BulkFailure{TestKey: key, Error: "not found"})
			continue
		}
		if err != nil {
			result.Failed = append(result.Failed, BulkFailure{TestKey: key, Error: err.Error()})
			continue
		}
		newVal, applyErr := applyBulkOperation(op, current)
		if applyErr != nil {
			result.Failed = append(result.Failed, BulkFailure{TestKey: key, Error: applyErr.Error()})
			continue
		}
		if newVal == current {
			// No-op (e.g. add_label when the label is already present) —
			// still report success so the user knows the request was handled.
			result.Succeeded = append(result.Succeeded, key)
			continue
		}
		if err := r.EditTestField(profileID, key, field, newVal); err != nil {
			result.Failed = append(result.Failed, BulkFailure{TestKey: key, Error: err.Error()})
			continue
		}
		result.Succeeded = append(result.Succeeded, key)
	}
	return result, nil
}

// resolveBulkField derives which test_case column the operation targets.
// Label operations always target the labels column; other operations need
// an explicit field.
func resolveBulkField(op BulkEdit) (string, error) {
	if op.Operation == "add_label" || op.Operation == "remove_label" {
		return "labels", nil
	}
	if op.Field == "" {
		return "", fmt.Errorf("field is required")
	}
	return op.Field, nil
}

// applyBulkOperation computes the new field value given the current value
// and the operation. It does not write anything.
func applyBulkOperation(op BulkEdit, current string) (string, error) {
	switch op.Operation {
	case "set":
		return op.Value, nil

	case "append":
		if op.Field != "description" {
			return "", fmt.Errorf("append is only supported for description")
		}
		if current == "" {
			return op.Value, nil
		}
		return current + "\n" + op.Value, nil

	case "add_label":
		if strings.TrimSpace(op.Value) == "" {
			return "", fmt.Errorf("label value is required")
		}
		labels := strings.Fields(current)
		for _, l := range labels {
			if l == op.Value {
				return current, nil
			}
		}
		labels = append(labels, op.Value)
		return strings.Join(labels, " "), nil

	case "remove_label":
		if strings.TrimSpace(op.Value) == "" {
			return "", fmt.Errorf("label value is required")
		}
		labels := strings.Fields(current)
		out := make([]string, 0, len(labels))
		for _, l := range labels {
			if l != op.Value {
				out = append(out, l)
			}
		}
		return strings.Join(out, " "), nil
	}
	return "", fmt.Errorf("unknown operation %q", op.Operation)
}

// --- Helpers ---

// Entity types for pending_change / audit_log rows. New ones get added
// here so the switch/lookup code stays grep-friendly.
const (
	entityTestCase       = "test_case"
	entityTestStep       = "test_step"
	entityTestStepDelete = "test_step_delete"
	entityTestStepAdd    = "test_step_add"
	entityTestStepOrder  = "test_step_order"
)

// stepEntityKey encodes a step's parent Test plus its Xray step ID into a
// single entity_key, since pending_change has just one key column.
// "QA-12:abc-uuid" — the first colon splits cleanly because Xray test
// keys never contain one.
const stepEntityKeySep = ":"

func stepEntityKey(testKey, xrayID string) string {
	return testKey + stepEntityKeySep + xrayID
}

func parseStepEntityKey(s string) (testKey, xrayID string, ok bool) {
	i := strings.Index(s, stepEntityKeySep)
	if i < 0 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

// upsertPendingChange records (or coalesces) a pending field change. If a row
// already exists for this (profile, entityType, entity, field) the AfterVal
// is updated; if the new value matches the existing BeforeVal (i.e. the user
// reverted to the original), the row is deleted.
func upsertPendingChange(tx *sql.Tx, profileID, entityType, entityKey, field, currentVal, newValue, baseVersion string) error {
	var existingBefore string
	err := tx.QueryRow(
		`SELECT before_val FROM pending_change
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = ?`,
		profileID, entityType, entityKey, field,
	).Scan(&existingBefore)

	now := time.Now().UTC().Format(time.RFC3339)

	if errors.Is(err, sql.ErrNoRows) {
		_, ierr := tx.Exec(
			`INSERT INTO pending_change
			   (profile_id, entity_type, entity_key, field, before_val, after_val, base_version, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			profileID, entityType, entityKey, field, currentVal, newValue, baseVersion, now,
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
			 WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = ?`,
			profileID, entityType, entityKey, field,
		); derr != nil {
			return fmt.Errorf("delete pending: %w", derr)
		}
		return nil
	}

	if _, uerr := tx.Exec(
		`UPDATE pending_change SET after_val = ?, created_at = ?
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = ?`,
		newValue, now, profileID, entityType, entityKey, field,
	); uerr != nil {
		return fmt.Errorf("update pending: %w", uerr)
	}
	return nil
}

// writeAudit appends one row to audit_log. Called from EditTestField,
// EditTestStepField, DiscardPendingChange, TransitionTest, and the
// commit / conflict paths.
func writeAudit(tx *sql.Tx, profileID, entityType, entityKey, action, field, beforeVal, afterVal, note string) error {
	if _, err := tx.Exec(
		`INSERT INTO audit_log
		   (profile_id, occurred_at, actor, entity_type, entity_key, action, field, before_val, after_val, note)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		profileID, time.Now().UTC().Format(time.RFC3339),
		currentActor(), entityType, entityKey, action, field, beforeVal, afterVal, note,
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
