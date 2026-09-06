package issuerepo

import (
	"context"
	"fmt"

	"agile-suite/core/journal"
)

const (
	// DraftPrefix starts the temporary key of an issue created locally and
	// not yet committed. Commit swaps it for Jira's key.
	DraftPrefix = "TAM-NEW-"
	// StatusDraft is the status a draft row shows until Commit creates it.
	StatusDraft = "Draft"
	// EntityIssue is the journal entity type of a field edit on an issue.
	EntityIssue = "issue"
	// EntityIssueCreate is the journal entity type of a draft's create row,
	// whose after_val is the draft as JSON.
	EntityIssueCreate = "issue_create"
	// FieldCreate is the field name on a create row.
	FieldCreate = "create"
	// EntityLink is the journal entity type of a link to create. The row's
	// field is LinkField(d) and its after_val the LinkDraft as JSON.
	EntityLink = "link"
)

// pendingFlag is the computed column every issue read carries.
const pendingFlag = `EXISTS (SELECT 1 FROM pending_change p WHERE p.profile_id = issue.profile_id AND p.entity_key = issue.key)`

// PendingKeys returns the keys with at least one pending change, sorted.
func (r *Repository) PendingKeys(ctx context.Context, profileID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT entity_key FROM pending_change WHERE profile_id = ? ORDER BY entity_key`, profileID)
	if err != nil {
		return nil, fmt.Errorf("pending keys: %w", err)
	}
	defer rows.Close()
	keys := []string{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// ListPendingChanges returns the profile's journal, newest first.
func (r *Repository) ListPendingChanges(ctx context.Context, profileID string) ([]journal.PendingChange, error) {
	return journal.List(r.db, profileID)
}

// PendingForKey returns one issue's pending changes, oldest first.
func (r *Repository) PendingForKey(ctx context.Context, profileID, key string) ([]journal.PendingChange, error) {
	return journal.ListForKey(r.db, profileID, key)
}

// ListActivity returns the audit trail for one issue, newest first.
func (r *Repository) ListActivity(ctx context.Context, profileID, key string, limit int) ([]journal.AuditEntry, error) {
	return journal.Entries(r.db, profileID, key, limit)
}
