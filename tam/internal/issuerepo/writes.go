package issuerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
)

// EditableFields are the fields EditField accepts, in the order the panel
// shows them. Their names are the JSON names on backend.Issue, plus
// description, which lives in the detail cache.
var EditableFields = []string{"summary", "description", "priority", "labels", "storyPoints", "assignee"}

// fieldColumns maps a field name to its issue column; description has none.
var fieldColumns = map[string]string{
	"summary": "summary", "description": "", "priority": "priority",
	"labels": "labels", "storyPoints": "story_points", "assignee": "assignee",
}

// draftTypes are the logical types a draft may have.
var draftTypes = map[string]bool{backend.TypeTask: true, backend.TypeStory: true, backend.TypeBug: true, backend.TypeRequirement: true}

// staleDetailStamp backdates a fabricated detail cache (one writeField built
// from nothing, rather than a real fetch) so it reads as stale at once.
const staleDetailStamp = "1970-01-01T00:00:00Z"

// execer is the subset of *sql.Tx and *sql.DB the field helpers use.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// FieldValue renders one editable field of an issue as the journal and the
// conflict table show it: labels as a comma list, points as a plain
// number, description from the detail cache the caller passes.
func FieldValue(iss backend.Issue, description, field string) string {
	switch field {
	case "summary":
		return iss.Summary
	case "description":
		return description
	case "priority":
		return iss.Priority
	case "assignee":
		return iss.Assignee
	case "labels":
		return strings.Join(iss.Labels, ", ")
	case "storyPoints":
		return backend.FormatPoints(iss.StoryPoints)
	}
	return ""
}

func validateField(field, value string) error {
	if _, ok := fieldColumns[field]; !ok {
		return fmt.Errorf("field %q cannot be edited", field)
	}
	switch field {
	case "summary":
		if strings.TrimSpace(value) == "" {
			return errors.New("summary cannot be empty")
		}
	case "storyPoints":
		if _, err := backend.ParsePoints(value); err != nil {
			return err
		}
	}
	return nil
}

// readField returns the current text form of a field and the row's
// updated stamp, which is the base version of any edit made now.
func readField(ctx context.Context, q execer, profileID, key, field string) (string, string, error) {
	var (
		iss     backend.Issue
		labels  string
		points  sql.NullFloat64
		detail  sql.NullString
		updated string
	)
	err := q.QueryRowContext(ctx,
		`SELECT summary, priority, assignee, labels, story_points, detail_json, updated FROM issue WHERE profile_id = ? AND key = ?`,
		profileID, key).Scan(&iss.Summary, &iss.Priority, &iss.Assignee, &labels, &points, &detail, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w", key, err)
	}
	_ = json.Unmarshal([]byte(labels), &iss.Labels)
	if points.Valid {
		v := points.Float64
		iss.StoryPoints = &v
	}
	description := ""
	if detail.Valid && detail.String != "" {
		var d backend.IssueDetail
		if err := json.Unmarshal([]byte(detail.String), &d); err == nil {
			description = d.Description
		}
	}
	return FieldValue(iss, description, field), updated, nil
}

// writeField stores the text form of a field on the row. Description goes
// into the detail cache, creating a minimal one when none exists so the
// panel can show the edit.
func writeField(ctx context.Context, q execer, profileID, key, field, value string) error {
	switch field {
	case "description":
		var (
			raw       sql.NullString
			fetchedAt sql.NullString
		)
		if err := q.QueryRowContext(ctx, `SELECT detail_json, detail_fetched_at FROM issue WHERE profile_id = ? AND key = ?`, profileID, key).Scan(&raw, &fetchedAt); err != nil {
			return fmt.Errorf("read detail for %s: %w", key, err)
		}
		d := backend.IssueDetail{Key: key, Fields: map[string]any{}}
		if raw.Valid && raw.String != "" {
			_ = json.Unmarshal([]byte(raw.String), &d)
		}
		d.Description = value
		d.Links = nil
		encoded, err := json.Marshal(d)
		if err != nil {
			return fmt.Errorf("encode detail for %s: %w", key, err)
		}
		// No detail was ever fetched for this row (a fresh row, or one a full
		// sync just deleted and reinserted), so this is a fabricated stub, not
		// a real read of Jira. Stamp it old rather than now, or the detail
		// cache looks fresh and the panel serves the stub for ten minutes
		// instead of refetching and picking up the links. A draft's detail
		// already has a stamp from CreateDraft, so it never hits this branch.
		at := fetchedAt.String
		if at == "" {
			at = staleDetailStamp
		}
		_, err = q.ExecContext(ctx, `UPDATE issue SET detail_json = ?, detail_fetched_at = ? WHERE profile_id = ? AND key = ?`, string(encoded), at, profileID, key)
		return err
	case "labels":
		encoded, _ := json.Marshal(backend.SplitLabels(value))
		_, err := q.ExecContext(ctx, `UPDATE issue SET labels = ? WHERE profile_id = ? AND key = ?`, string(encoded), profileID, key)
		return err
	case "storyPoints":
		p, err := backend.ParsePoints(value)
		if err != nil {
			return err
		}
		var points sql.NullFloat64
		if p != nil {
			points = sql.NullFloat64{Float64: *p, Valid: true}
		}
		_, err = q.ExecContext(ctx, `UPDATE issue SET story_points = ? WHERE profile_id = ? AND key = ?`, points, profileID, key)
		return err
	}
	col, ok := fieldColumns[field]
	if !ok || col == "" {
		return fmt.Errorf("field %q cannot be edited", field)
	}
	_, err := q.ExecContext(ctx, `UPDATE issue SET `+col+` = ? WHERE profile_id = ? AND key = ?`, value, profileID, key)
	return err
}

// reapplyPending rewrites the columns of every pending field edit for the
// given keys, so a sync that just refreshed those rows from Jira does not
// hide a local edit. The journal's base version is left alone: if Jira did
// change, the next Commit sees it.
func reapplyPending(ctx context.Context, q execer, profileID string, keys map[string]bool) error {
	if len(keys) == 0 {
		return nil
	}
	rows, err := q.QueryContext(ctx,
		`SELECT entity_key, field, after_val FROM pending_change WHERE profile_id = ? AND entity_type = ? ORDER BY id`,
		profileID, EntityIssue)
	if err != nil {
		return fmt.Errorf("pending fields: %w", err)
	}
	type edit struct{ key, field, value string }
	var edits []edit
	for rows.Next() {
		var e edit
		if err := rows.Scan(&e.key, &e.field, &e.value); err != nil {
			rows.Close()
			return err
		}
		if keys[e.key] {
			edits = append(edits, e)
		}
	}
	rows.Close()
	for _, e := range edits {
		if err := writeField(ctx, q, profileID, e.key, e.field, e.value); err != nil {
			return fmt.Errorf("reapply %s.%s: %w", e.key, e.field, err)
		}
	}
	return nil
}

// EditField changes one field on the row and journals it. The edit's base
// version is the row's updated stamp. On a draft the create row's JSON is
// rewritten instead of adding a second journal row.
func (r *Repository) EditField(ctx context.Context, profileID, key, field, value string) error {
	if err := validateField(field, value); err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	current, updated, err := readField(ctx, tx, profileID, key, field)
	if err != nil {
		return err
	}
	if current == value {
		return nil
	}
	if err := writeField(ctx, tx, profileID, key, field, value); err != nil {
		return err
	}
	if strings.HasPrefix(key, DraftPrefix) {
		if err := updateDraftJSON(ctx, tx, profileID, key, field, value); err != nil {
			return err
		}
	} else if err := journal.Upsert(tx, profileID, EntityIssue, key, field, current, value, updated); err != nil {
		return err
	}
	if err := journal.Audit(tx, profileID, EntityIssue, key, "edit", field, current, value, ""); err != nil {
		return err
	}
	return tx.Commit()
}

func updateDraftJSON(ctx context.Context, tx *sql.Tx, profileID, key, field, value string) error {
	var raw string
	err := tx.QueryRowContext(ctx,
		`SELECT after_val FROM pending_change WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
		profileID, EntityIssueCreate, key).Scan(&raw)
	if err != nil {
		return fmt.Errorf("draft %s has no create row: %w", key, err)
	}
	var d backend.IssueDraft
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return fmt.Errorf("decode draft %s: %w", key, err)
	}
	switch field {
	case "summary":
		d.Summary = value
	case "description":
		d.Description = value
	case "priority":
		d.Priority = value
	case "assignee":
		d.Assignee = value
	case "labels":
		d.Labels = backend.SplitLabels(value)
	case "storyPoints":
		d.StoryPoints, _ = backend.ParsePoints(value)
	}
	encoded, err := json.Marshal(d)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE pending_change SET after_val = ? WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
		string(encoded), profileID, EntityIssueCreate, key)
	return err
}

// CreateDraft inserts one draft. See CreateDrafts.
func (r *Repository) CreateDraft(ctx context.Context, profileID, projectKey string, d backend.IssueDraft) (string, error) {
	keys, err := r.CreateDrafts(ctx, profileID, projectKey, []backend.IssueDraft{d}, "")
	if err != nil {
		return "", err
	}
	return keys[0], nil
}

// CreateDrafts inserts placeholder rows under the next temporary keys, one
// create row each holding the draft as JSON, in one transaction: any
// invalid draft fails the whole batch. note lands on every audit entry
// (the import puts the file name there). It returns the temporary keys in
// order.
func (r *Repository) CreateDrafts(ctx context.Context, profileID, projectKey string, drafts []backend.IssueDraft, note string) ([]string, error) {
	if len(drafts) == 0 {
		return nil, errors.New("nothing to create")
	}
	for i := range drafts {
		d := &drafts[i]
		if !draftTypes[d.Type] {
			return nil, fmt.Errorf("type %q cannot be created here; tasks, stories, bugs, and requirements can", d.Type)
		}
		if strings.TrimSpace(d.Summary) == "" {
			return nil, errors.New("summary cannot be empty")
		}
		d.Summary = strings.TrimSpace(d.Summary)
		d.ParentKey = strings.TrimSpace(d.ParentKey)
		if d.Labels == nil {
			d.Labels = []string{}
		}
		if d.Extra == nil {
			d.Extra = map[string]string{}
		}
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var last int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(CAST(SUBSTR(key, ?) AS INTEGER)), 0) FROM issue WHERE profile_id = ? AND key LIKE ?`,
		len(DraftPrefix)+1, profileID, DraftPrefix+"%").Scan(&last); err != nil {
		return nil, fmt.Errorf("next draft key: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	keys := make([]string, 0, len(drafts))
	for _, d := range drafts {
		last++
		key := fmt.Sprintf("%s%d", DraftPrefix, last)
		encoded, err := json.Marshal(d)
		if err != nil {
			return nil, fmt.Errorf("encode draft: %w", err)
		}
		labels, _ := json.Marshal(d.Labels)
		var points sql.NullFloat64
		if d.StoryPoints != nil {
			points = sql.NullFloat64{Float64: *d.StoryPoints, Valid: true}
		}
		detail, _ := json.Marshal(backend.IssueDetail{Key: key, Description: d.Description, Fields: map[string]any{}})
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO issue (profile_id, key, id, project, type, summary, status, assignee, reporter, priority, labels,
				sprint_id, sprint_name, parent_key, story_points, rank, created, updated, synced_at, detail_json, detail_fetched_at)
			VALUES (?, ?, '', ?, ?, ?, ?, ?, '', ?, ?, '', '', ?, ?, '', ?, '', '', ?, ?)`,
			profileID, key, projectKey, d.Type, d.Summary, StatusDraft, d.Assignee, d.Priority, string(labels),
			d.ParentKey, points, now, string(detail), now); err != nil {
			return nil, fmt.Errorf("insert draft: %w", err)
		}
		if err := journal.Put(tx, profileID, EntityIssueCreate, key, FieldCreate, "", string(encoded), ""); err != nil {
			return nil, err
		}
		if err := journal.Audit(tx, profileID, EntityIssue, key, "create", "", "", d.Summary, note); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return keys, nil
}

// Rekey moves a draft to the key Jira assigned, across the row, its links,
// its journal rows, and its audit trail, and audits the creation.
func (r *Repository) Rekey(ctx context.Context, profileID, tempKey, realKey string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`UPDATE issue SET key = ? WHERE profile_id = ? AND key = ?`,
		`UPDATE issue_link SET from_key = ? WHERE profile_id = ? AND from_key = ?`,
		`UPDATE pending_change SET entity_key = ? WHERE profile_id = ? AND entity_key = ?`,
		`UPDATE audit_log SET entity_key = ? WHERE profile_id = ? AND entity_key = ?`,
	} {
		if _, err := tx.ExecContext(ctx, stmt, realKey, profileID, tempKey); err != nil {
			return fmt.Errorf("rekey %s to %s: %w", tempKey, realKey, err)
		}
	}
	if err := journal.Audit(tx, profileID, EntityIssue, realKey, "created", "", tempKey, realKey, "created in Jira"); err != nil {
		return err
	}
	return tx.Commit()
}

// ReplaceRow overwrites every column of one issue from a fresh Jira read and
// drops its detail cache so the panel refetches. It does not touch the
// journal; callers delete the rows first when that is the intent.
func (r *Repository) ReplaceRow(ctx context.Context, profileID string, iss backend.Issue) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := upsertIssue(ctx, tx, profileID, iss, time.Now()); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE issue SET detail_json = NULL, detail_fetched_at = NULL WHERE profile_id = ? AND key = ?`,
		profileID, iss.Key); err != nil {
		return fmt.Errorf("clear detail for %s: %w", iss.Key, err)
	}
	// A pending edit that was not part of the push this row came from (made
	// while the commit was in flight, or left behind because only some of the
	// issue's fields were pushed) must survive the overwrite.
	if err := reapplyPending(ctx, tx, profileID, map[string]bool{iss.Key: true}); err != nil {
		return err
	}
	return tx.Commit()
}

// SetBaseVersion rebases an issue's pending changes onto the remote version
// the user chose to override, and audits the choice.
func (r *Repository) SetBaseVersion(ctx context.Context, profileID, key, version string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := journal.SetBaseVersion(tx, profileID, key, version); err != nil {
		return err
	}
	if err := journal.Audit(tx, profileID, EntityIssue, key, "override", "", "", version, "pending edits rebased onto the remote version"); err != nil {
		return err
	}
	return tx.Commit()
}

// DiscardPendingChange reverts one journal row: a field edit goes back to
// its before value, a create row takes its draft row with it.
func (r *Repository) DiscardPendingChange(ctx context.Context, profileID string, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	p, err := journal.Get(tx, profileID, id)
	if err != nil {
		return err
	}
	if err := discardOne(ctx, tx, profileID, p); err != nil {
		return err
	}
	return tx.Commit()
}

// DiscardAllPendingChanges reverts every journal row of the profile and
// returns how many it reverted.
func (r *Repository) DiscardAllPendingChanges(ctx context.Context, profileID string) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	all, err := journal.List(tx, profileID)
	if err != nil {
		return 0, err
	}
	for _, p := range all {
		if err := discardOne(ctx, tx, profileID, p); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(all), nil
}

// DiscardKey reverts every pending change of one issue, which is what a
// keep-remote resolution does before it replaces the row.
func (r *Repository) DiscardKey(ctx context.Context, profileID, key string) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	rows, err := journal.ListForKey(tx, profileID, key)
	if err != nil {
		return 0, err
	}
	for _, p := range rows {
		if err := discardOne(ctx, tx, profileID, p); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(rows), nil
}

func discardOne(ctx context.Context, tx *sql.Tx, profileID string, p journal.PendingChange) error {
	if p.EntityType == EntityIssueCreate {
		if _, err := tx.ExecContext(ctx, `DELETE FROM issue_link WHERE profile_id = ? AND from_key = ?`, profileID, p.EntityKey); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM issue WHERE profile_id = ? AND key = ?`, profileID, p.EntityKey); err != nil {
			return fmt.Errorf("drop draft %s: %w", p.EntityKey, err)
		}
	} else {
		var exists int
		err := tx.QueryRowContext(ctx, `SELECT 1 FROM issue WHERE profile_id = ? AND key = ?`, profileID, p.EntityKey).Scan(&exists)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			// A full sync no longer returns this issue. There is no row left
			// to revert; still drop the journal row and record the discard.
		case err != nil:
			return fmt.Errorf("check %s exists: %w", p.EntityKey, err)
		default:
			if err := writeField(ctx, tx, profileID, p.EntityKey, p.Field, p.BeforeVal); err != nil {
				return fmt.Errorf("revert %s.%s: %w", p.EntityKey, p.Field, err)
			}
		}
	}
	if err := journal.Delete(tx, profileID, []int64{p.ID}); err != nil {
		return err
	}
	return journal.Audit(tx, profileID, p.EntityType, p.EntityKey, "discard", p.Field, p.AfterVal, p.BeforeVal, "")
}

// MarkCommitted deletes the journal rows a commit pushed and audits each.
func (r *Repository) MarkCommitted(ctx context.Context, profileID string, changes []journal.PendingChange) error {
	if len(changes) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ids := make([]int64, 0, len(changes))
	for _, p := range changes {
		ids = append(ids, p.ID)
		if err := journal.Audit(tx, profileID, p.EntityType, p.EntityKey, "commit", p.Field, p.BeforeVal, p.AfterVal, ""); err != nil {
			return err
		}
	}
	if err := journal.Delete(tx, profileID, ids); err != nil {
		return err
	}
	return tx.Commit()
}

// MarkCreatedWithoutRekey clears a draft's journal rows when Jira accepted
// the create but the local rename to the real key failed: it deletes the
// create (and any edit) rows under the temp key, same as MarkCommitted, then
// audits the creation under the temp key with the real key in the note, so a
// retry sees nothing pending and does not post the draft again. The draft
// row keeps its temporary key; the next sync brings the real row in.
func (r *Repository) MarkCreatedWithoutRekey(ctx context.Context, profileID, tempKey, realKey string, rows []journal.PendingChange) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ids := make([]int64, 0, len(rows))
	for _, p := range rows {
		ids = append(ids, p.ID)
		if err := journal.Audit(tx, profileID, p.EntityType, p.EntityKey, "commit", p.Field, p.BeforeVal, p.AfterVal, ""); err != nil {
			return err
		}
	}
	if err := journal.Delete(tx, profileID, ids); err != nil {
		return err
	}
	note := fmt.Sprintf("created in Jira as %s but the local rename failed; the row keeps its temporary key until the next sync", realKey)
	if err := journal.Audit(tx, profileID, EntityIssue, tempKey, "created", "", tempKey, realKey, note); err != nil {
		return err
	}
	return tx.Commit()
}
