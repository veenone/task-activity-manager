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
var EditableFields = []string{"summary", "description", "priority", "labels", "storyPoints", "assignee", "parentKey"}

// fieldColumns maps a field name to its issue column; description has none.
var fieldColumns = map[string]string{
	"summary": "summary", "description": "", "priority": "priority",
	"labels": "labels", "storyPoints": "story_points", "assignee": "assignee", "parentKey": "parent_key",
}

// draftTypes are the logical types a draft may have.
var draftTypes = map[string]bool{backend.TypeTask: true, backend.TypeStory: true, backend.TypeBug: true, backend.TypeRequirement: true, backend.TypeEpic: true}

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
	case "parentKey":
		return iss.ParentKey
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

// readField returns the current text form of a field, the row's updated
// stamp (the base version of any edit made now), and the row's own type,
// which a parentKey edit needs to enforce the hierarchy.
func readField(ctx context.Context, q execer, profileID, key, field string) (value, updated, ownType string, err error) {
	var (
		iss    backend.Issue
		labels string
		points sql.NullFloat64
		detail sql.NullString
	)
	err = q.QueryRowContext(ctx,
		`SELECT summary, priority, assignee, labels, story_points, detail_json, updated, parent_key, type FROM issue WHERE profile_id = ? AND key = ?`,
		profileID, key).Scan(&iss.Summary, &iss.Priority, &iss.Assignee, &labels, &points, &detail, &updated, &iss.ParentKey, &iss.Type)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", "", ErrNotFound
	}
	if err != nil {
		return "", "", "", fmt.Errorf("read %s: %w", key, err)
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
	return FieldValue(iss, description, field), updated, iss.Type, nil
}

// validateParent enforces the two-level hierarchy: an epic takes no parent,
// and a parent must be an epic in the profile's cache and not the issue
// itself. An empty value takes the issue out of its epic.
// validateParent checks a parent for the issue's own level. The parent field
// means two different things: for a sub-task it is Jira's own parent, which
// is any ordinary issue, and for everything else it is the Epic Link, which
// must be an epic. A sub-task without one is not a sub-task, so its parent is
// also the one that cannot be blank.
func validateParent(ctx context.Context, q execer, profileID, key, ownType, value string) error {
	value = strings.TrimSpace(value)
	sub := ownType == backend.TypeSubtask
	if ownType == backend.TypeEpic && value != "" {
		return errors.New("an epic cannot be placed under another epic")
	}
	if value == "" {
		if sub {
			return errors.New("a sub-task needs a parent issue")
		}
		return nil
	}
	noun := "epic"
	if sub {
		noun = "parent"
	}
	if strings.EqualFold(value, key) {
		return fmt.Errorf("an issue cannot be its own %s", noun)
	}
	var typ string
	err := q.QueryRowContext(ctx, `SELECT type FROM issue WHERE profile_id = ? AND key = ?`, profileID, value).Scan(&typ)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s is not in the cache; sync first", value)
	}
	if err != nil {
		return fmt.Errorf("check %s %s: %w", noun, value, err)
	}
	if sub {
		// Jira allows a sub-task under any standard issue, and under neither
		// an epic nor another sub-task.
		if typ == backend.TypeEpic {
			return fmt.Errorf("%s is an epic; a sub-task hangs off an issue", value)
		}
		if typ == backend.TypeSubtask {
			return fmt.Errorf("%s is a sub-task; sub-tasks cannot nest", value)
		}
		return nil
	}
	if typ != backend.TypeEpic {
		return fmt.Errorf("%s is not an epic", value)
	}
	return nil
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

// reapplyPending rewrites the columns of every pending field edit and every
// pending board move for the given keys, so a sync that just refreshed
// those rows from Jira does not hide a local change. The journal's base
// version is left alone: if Jira did change, the next Commit sees it.
//
// The board types are here for the same reason the field edits are, and the
// omission would be quieter: a full sync deletes and reinserts every row, so
// without them it would put a moved card back in its old column while the
// journal still said it moved, and the Backlog and the board would then
// disagree about the same issue.
func reapplyPending(ctx context.Context, q execer, profileID string, keys map[string]bool) error {
	if len(keys) == 0 {
		return nil
	}
	types := append([]string{EntityIssue}, BoardEntities...)
	args := make([]any, 0, len(types)+1)
	args = append(args, profileID)
	for _, t := range types {
		args = append(args, t)
	}
	marks := strings.TrimSuffix(strings.Repeat("?, ", len(types)), ", ")
	rows, err := q.QueryContext(ctx,
		`SELECT entity_type, entity_key, field, after_val FROM pending_change
		 WHERE profile_id = ? AND entity_type IN (`+marks+`) ORDER BY id`, args...)
	if err != nil {
		return fmt.Errorf("pending fields: %w", err)
	}
	type edit struct{ entityType, key, field, value string }
	var edits []edit
	for rows.Next() {
		var e edit
		if err := rows.Scan(&e.entityType, &e.key, &e.field, &e.value); err != nil {
			rows.Close()
			return err
		}
		if keys[e.key] {
			edits = append(edits, e)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, e := range edits {
		if e.entityType == EntityIssue {
			if err := writeField(ctx, q, profileID, e.key, e.field, e.value); err != nil {
				return fmt.Errorf("reapply %s.%s: %w", e.key, e.field, err)
			}
			continue
		}
		if err := applyMoveColumns(ctx, q, profileID, e.key, e.entityType, e.value); err != nil {
			return fmt.Errorf("reapply the move of %s: %w", e.key, err)
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

	current, updated, ownType, err := readField(ctx, tx, profileID, key, field)
	if err != nil {
		return err
	}
	if field == "parentKey" {
		value = strings.TrimSpace(value)
	}
	if current == value {
		return nil
	}
	if field == "parentKey" {
		if err := validateParent(ctx, tx, profileID, key, ownType, value); err != nil {
			return err
		}
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

// Edit is one field change of one issue, for a batch that has to land or
// fail as a whole.
type Edit struct {
	Key   string
	Field string
	Value string
}

// EditFields journals many field changes in one transaction, the write half
// of an import that updates issues rather than creating them. A field whose
// value already matches is skipped rather than journaled, so re-importing an
// unchanged file leaves nothing pending. note is what the audit records, the
// same way CreateDrafts records the file an import came from.
//
// It returns the keys it actually changed, in the order they were given and
// each once, so the caller can report what an import touched.
func (r *Repository) EditFields(ctx context.Context, profileID string, edits []Edit, note string) ([]string, error) {
	if len(edits) == 0 {
		return []string{}, nil
	}
	for _, e := range edits {
		if err := validateField(e.Field, e.Value); err != nil {
			return nil, fmt.Errorf("%s %s: %w", e.Key, e.Field, err)
		}
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	touched := []string{}
	seen := map[string]bool{}
	for _, e := range edits {
		current, updated, ownType, err := readField(ctx, tx, profileID, e.Key, e.Field)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Key, err)
		}
		value := e.Value
		if e.Field == "parentKey" {
			value = strings.TrimSpace(value)
			if err := validateParent(ctx, tx, profileID, e.Key, ownType, value); err != nil {
				return nil, fmt.Errorf("%s: %w", e.Key, err)
			}
		}
		if current == value {
			continue
		}
		if err := writeField(ctx, tx, profileID, e.Key, e.Field, value); err != nil {
			return nil, err
		}
		if err := journal.Upsert(tx, profileID, EntityIssue, e.Key, e.Field, current, value, updated); err != nil {
			return nil, err
		}
		if err := journal.Audit(tx, profileID, EntityIssue, e.Key, "edit", e.Field, current, value, note); err != nil {
			return nil, err
		}
		if !seen[e.Key] {
			seen[e.Key] = true
			touched = append(touched, e.Key)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return touched, nil
}

func updateDraftJSON(ctx context.Context, tx *sql.Tx, profileID, key, field, value string) error {
	return editDraft(ctx, tx, profileID, key, func(d *backend.IssueDraft) {
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
		case "parentKey":
			d.ParentKey = strings.TrimSpace(value)
		}
	})
}

// editDraft applies fn to the draft a create row carries and writes the
// JSON back. Every local change to a draft goes through here, whether it is
// a field edit or a board move, so there is one read, one decode, and one
// encode of a draft rather than a copy per caller.
func editDraft(ctx context.Context, tx *sql.Tx, profileID, key string, fn func(d *backend.IssueDraft)) error {
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
	fn(&d)
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
			return nil, fmt.Errorf("type %q cannot be created here; tasks, stories, bugs, requirements, and epics can", d.Type)
		}
		if strings.TrimSpace(d.Summary) == "" {
			return nil, errors.New("summary cannot be empty")
		}
		d.Summary = strings.TrimSpace(d.Summary)
		d.ParentKey = strings.TrimSpace(d.ParentKey)
		if d.Type == backend.TypeEpic && d.ParentKey != "" {
			return nil, errors.New("an epic cannot be placed under another epic")
		}
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

	last, err := nextDraftNumberTx(ctx, tx, profileID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	keys := make([]string, 0, len(drafts))
	for _, d := range drafts {
		last++
		key := fmt.Sprintf("%s%d", DraftPrefix, last)
		if d.ParentKey != "" {
			if err := validateParent(ctx, tx, profileID, key, d.Type, d.ParentKey); err != nil {
				return nil, err
			}
		}
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
		// status_id is left at its default: a draft has no Jira status, and
		// an empty status id is what puts it in the board's first real
		// column instead of nowhere.
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

// NextDraftNumber returns the suffix the next TAM-NEW-n key would get. This
// is a preview only, for the importer's own file: it lets a file's Epic rows
// predict the key a later row's Parent cell names, before any of the batch
// exists in the cache. It reads outside any transaction, so a concurrent
// CreateDrafts call can claim the number this returns; CreateDrafts re-reads
// the same query inside its own transaction rather than trusting this value.
func (r *Repository) NextDraftNumber(ctx context.Context, profileID string) (int, error) {
	return nextDraftNumberTx(ctx, r.db, profileID)
}

// nextDraftNumberTx runs the next-draft-number query against the given
// handle. CreateDrafts calls it inside its transaction, after BeginTx, so
// the read and the insert commit together: SQLite serialises writers, so
// that is enough to keep two concurrent creates from landing on the same
// number.
func nextDraftNumberTx(ctx context.Context, q execer, profileID string) (int, error) {
	var last int
	if err := q.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(CAST(SUBSTR(key, ?) AS INTEGER)), 0) FROM issue WHERE profile_id = ? AND key LIKE ?`,
		len(DraftPrefix)+1, profileID, DraftPrefix+"%").Scan(&last); err != nil {
		return 0, fmt.Errorf("next draft key: %w", err)
	}
	return last, nil
}
