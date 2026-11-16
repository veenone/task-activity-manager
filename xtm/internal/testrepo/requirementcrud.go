package testrepo

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// requirementEditFields maps an editable requirement field to its DB column.
// Status changes go through a workflow transition, not a field edit.
// NOTE(xtm): live Jira push for these fields is TODO — currently all edits
// are stored locally and only committed to Jira when CommitPendingChanges runs;
// the syncer.Engine.CommitChanges path for requirement field updates is stubbed
// behind the demo short-circuit until it can be verified against a live instance.
var requirementEditFields = map[string]string{
	"summary":      "summary",
	"description":  "description",
	"priority":     "priority",
	"components":   "components",
	"fix_versions": "fix_versions",
	"sprint":       "sprint",
}

// requirementDeleteSnapshot captures a requirement plus its Test links so a
// discarded delete can restore it.
type requirementDeleteSnapshot struct {
	ProjectKey string        `json:"projectKey"`
	IssueType  string        `json:"issueType"`
	Summary    string        `json:"summary"`
	Status     string        `json:"status"`
	Updated    string        `json:"updated"`
	Links      []reqLinkSnap `json:"links"`
}

// EditRequirementField edits a requirement field (summary, description,
// priority, components, fix_versions, or sprint) and queues the change for
// commit (a Jira issue update). Editing a requirement in another project
// requires edit permission there — surfaced at commit time.
func (r *Repository) EditRequirementField(profileID, requirementKey, field, newValue string) error {
	col, ok := requirementEditFields[field]
	if !ok {
		return fmt.Errorf("requirement field %q is not editable", field)
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var current string
	readSQL := fmt.Sprintf(`SELECT %s FROM requirement WHERE profile_id = ? AND jira_key = ?`, col)
	err = tx.QueryRow(readSQL, profileID, requirementKey).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("requirement %s not found", requirementKey)
	}
	if err != nil {
		return fmt.Errorf("read requirement value: %w", err)
	}
	if current == newValue {
		return nil
	}

	updateSQL := fmt.Sprintf(`UPDATE requirement SET %s = ? WHERE profile_id = ? AND jira_key = ?`, col)
	if _, err := tx.Exec(updateSQL, newValue, profileID, requirementKey); err != nil {
		return fmt.Errorf("update requirement: %w", err)
	}
	if err := upsertPendingChange(
		tx, profileID, entityRequirementEdit, requirementKey, field, current, newValue, "",
	); err != nil {
		return err
	}
	if err := writeAudit(
		tx, profileID, entityRequirementEdit, requirementKey,
		"edit-requirement-local", field, current, newValue, "",
	); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteRequirement removes a requirement and its Test coverage links and queues
// the deletion for commit. Deleting a requirement issue (often cross-project) is
// permission-sensitive — surfaced at commit time.
func (r *Repository) DeleteRequirement(profileID, key string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var snap requirementDeleteSnapshot
	err = tx.QueryRow(
		`SELECT project_key, issue_type, summary, status, updated_at
		 FROM requirement WHERE profile_id = ? AND jira_key = ?`,
		profileID, key,
	).Scan(&snap.ProjectKey, &snap.IssueType, &snap.Summary, &snap.Status, &snap.Updated)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("requirement %s not found", key)
	}
	if err != nil {
		return fmt.Errorf("read requirement: %w", err)
	}
	snap.Links, err = requirementLinkSnaps(tx, profileID, key)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(
		`DELETE FROM requirement WHERE profile_id = ? AND jira_key = ?`, profileID, key,
	); err != nil {
		return fmt.Errorf("delete requirement: %w", err)
	}
	if _, err := tx.Exec(
		`DELETE FROM test_requirement WHERE profile_id = ? AND requirement_key = ?`, profileID, key,
	); err != nil {
		return fmt.Errorf("delete requirement links: %w", err)
	}
	// Drop the deleted requirement from any pending coverage set so a committed
	// set never references a now-deleted key.
	if err := rewriteRequirementSets(tx, profileID, key, ""); err != nil {
		return err
	}
	cancelled, err := queueDelete(
		tx, profileID, key, entityRequirementCreate, entityRequirementDelete, "requirement", snap.Summary, snap,
		entityRequirementEdit,
	)
	if err != nil {
		return err
	}
	// A requirement Jira has never seen takes its local links with it, so
	// nothing about it reaches Jira.
	if cancelled {
		if err := dropLocalRequirementLinks(tx, profileID, key); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// requirementLinkSnaps returns the Test links of a requirement as snapshots.
func requirementLinkSnaps(tx *sql.Tx, profileID, requirementKey string) ([]reqLinkSnap, error) {
	rows, err := tx.Query(
		`SELECT test_key, link_id FROM test_requirement
		 WHERE profile_id = ? AND requirement_key = ?`,
		profileID, requirementKey)
	if err != nil {
		return nil, fmt.Errorf("read requirement links: %w", err)
	}
	defer rows.Close()
	out := []reqLinkSnap{}
	for rows.Next() {
		var testKey, linkID string
		if err := rows.Scan(&testKey, &linkID); err != nil {
			return nil, err
		}
		out = append(out, reqLinkSnap{Key: testKey, LinkID: linkID})
	}
	return out, rows.Err()
}

// CreateRequirement queues a brand-new local Requirement (temp "NEW-REQ-N" key)
// and inserts it into the requirement table so it shows up in
// ListRequirementsWithCoverage. Returns the temp key.
func (r *Repository) CreateRequirement(
	profileID, projectKey, issueType, summary, description, priority, components, fixVersions string,
	fields map[string]any,
) (string, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return "", fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	tempKey, err := nextNewReqKey(tx, profileID)
	if err != nil {
		return "", err
	}

	if _, err := tx.Exec(
		`INSERT INTO requirement
		   (profile_id, jira_key, project_key, issue_type, summary, status, updated_at,
		    priority, components, fix_versions, sprint, description)
		 VALUES (?, ?, ?, ?, ?, '', '', ?, ?, ?, '', ?)`,
		profileID, tempKey, projectKey, issueType, summary,
		priority, components, fixVersions, description,
	); err != nil {
		return "", fmt.Errorf("insert local requirement: %w", err)
	}

	type requirementCreatePayload struct {
		ProjectKey  string         `json:"projectKey"`
		IssueType   string         `json:"issueType"`
		Summary     string         `json:"summary"`
		Description string         `json:"description"`
		Priority    string         `json:"priority"`
		Components  string         `json:"components"`
		FixVersions string         `json:"fixVersions"`
		Fields      map[string]any `json:"fields,omitempty"`
	}
	payload := requirementCreatePayload{
		ProjectKey:  projectKey,
		IssueType:   issueType,
		Summary:     summary,
		Description: description,
		Priority:    priority,
		Components:  components,
		FixVersions: fixVersions,
		Fields:      fields,
	}
	encoded, _ := json.Marshal(payload)
	if err := upsertPendingChange(
		tx, profileID, entityRequirementCreate, tempKey, "requirement", "", string(encoded), "",
	); err != nil {
		return "", err
	}
	if err := writeAudit(
		tx, profileID, entityRequirementCreate, tempKey,
		"create-requirement-local", "requirement", "", summary, "",
	); err != nil {
		return "", err
	}
	return tempKey, tx.Commit()
}

// RenameRequirement rewrites a Requirement's key across the cache and any
// pending coverage-link rows, used by the commit path to swap a "NEW-REQ-N"
// placeholder for the real key Jira assigned. A no-op when newKey is empty or
// unchanged.
func (r *Repository) RenameRequirement(profileID, oldKey, newKey string) error {
	if newKey == "" || newKey == oldKey {
		return nil
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(
		`UPDATE requirement SET jira_key = ? WHERE profile_id = ? AND jira_key = ?`,
		newKey, profileID, oldKey,
	); err != nil {
		return fmt.Errorf("rename requirement: %w", err)
	}
	// Rewrite coverage links that referenced the temp key (test_requirement).
	if _, err := tx.Exec(
		`UPDATE test_requirement SET requirement_key = ?
		 WHERE profile_id = ? AND requirement_key = ?`,
		newKey, profileID, oldKey,
	); err != nil {
		return fmt.Errorf("rename requirement links: %w", err)
	}
	// Rewrite req-req links.
	if _, err := tx.Exec(
		`UPDATE requirement_link SET from_requirement_key = ?
		 WHERE profile_id = ? AND from_requirement_key = ?`,
		newKey, profileID, oldKey,
	); err != nil {
		return fmt.Errorf("rename req-req from_requirement_key: %w", err)
	}
	if _, err := tx.Exec(
		`UPDATE requirement_link SET to_requirement_key = ?
		 WHERE profile_id = ? AND to_requirement_key = ?`,
		newKey, profileID, oldKey,
	); err != nil {
		return fmt.Errorf("rename req-req to_requirement_key: %w", err)
	}
	// Pending rows that name the old key have not reached Jira yet; they must
	// go out under the new one.
	if err := rewriteRequirementSets(tx, profileID, oldKey, newKey); err != nil {
		return err
	}
	if err := rewriteReqReqLinkSets(tx, profileID, oldKey, newKey); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`UPDATE pending_change SET entity_key = ?
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
		newKey, profileID, entityRequirementEdit, oldKey,
	); err != nil {
		return fmt.Errorf("rename pending requirement edits: %w", err)
	}
	return tx.Commit()
}

// nextNewReqKey allocates an unused "NEW-REQ-N" placeholder.
func nextNewReqKey(tx *sql.Tx, profileID string) (string, error) {
	return nextFreeKey(tx, "req key", "NEW-REQ-",
		`SELECT 1 FROM requirement WHERE profile_id = ? AND jira_key = ?`, profileID)
}
