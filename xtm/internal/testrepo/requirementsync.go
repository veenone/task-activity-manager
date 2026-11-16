package testrepo

import (
	"database/sql"
	"fmt"
)

// The sync writers below replace a cache with what Jira holds and then put the
// profile's uncommitted changes back on top, so a sync before a commit does not
// undo local work whose pending rows are still queued.

// ReplaceAllRequirements rewrites the cached requirement set for a profile.
// Requirements with a pending create survive, ones with a pending delete stay
// gone, and pending field edits are reapplied.
func (r *Repository) ReplaceAllRequirements(profileID string, reqs []Requirement) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// A locally created requirement exists nowhere but here.
	if _, err := tx.Exec(
		`DELETE FROM requirement WHERE profile_id = ? AND jira_key NOT IN (
		   SELECT entity_key FROM pending_change WHERE profile_id = ? AND entity_type = ?)`,
		profileID, profileID, entityRequirementCreate,
	); err != nil {
		return fmt.Errorf("clear requirements: %w", err)
	}
	for _, rq := range reqs {
		if _, err := tx.Exec(
			`INSERT OR REPLACE INTO requirement
			   (profile_id, jira_key, project_key, issue_type, summary, status, updated_at,
			    priority, components, fix_versions, sprint, description, epic_key)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			profileID, rq.Key, rq.ProjectKey, rq.IssueType, rq.Summary, rq.Status, rq.Updated,
			rq.Priority, rq.Components, rq.FixVersions, rq.Sprint, rq.Description, rq.EpicKey,
		); err != nil {
			return fmt.Errorf("insert requirement %s: %w", rq.Key, err)
		}
	}
	if _, err := tx.Exec(
		`DELETE FROM requirement WHERE profile_id = ? AND jira_key IN (
		   SELECT entity_key FROM pending_change WHERE profile_id = ? AND entity_type = ?)`,
		profileID, profileID, entityRequirementDelete,
	); err != nil {
		return fmt.Errorf("reapply pending requirement deletes: %w", err)
	}
	if err := reapplyFieldEdits(tx, profileID, "requirement", entityRequirementEdit, requirementEditFields); err != nil {
		return err
	}
	return tx.Commit()
}

// ReplaceAllRequirementLinks rewrites the cached Test <-> Requirement links.
// A Test with a pending coverage edit keeps the set that edit asked for, and a
// requirement with a pending delete keeps no links.
func (r *Repository) ReplaceAllRequirementLinks(profileID string, links []RequirementLink) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM test_requirement WHERE profile_id = ?`, profileID); err != nil {
		return fmt.Errorf("clear requirement links: %w", err)
	}
	for _, l := range links {
		if _, err := tx.Exec(
			`INSERT OR REPLACE INTO test_requirement
			   (profile_id, test_key, requirement_key, link_id) VALUES (?, ?, ?, ?)`,
			profileID, l.TestKey, l.RequirementKey, l.LinkID,
		); err != nil {
			return fmt.Errorf("insert requirement link %s->%s: %w", l.TestKey, l.RequirementKey, err)
		}
	}
	if err := reapplyRequirementSets(tx, profileID); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`DELETE FROM test_requirement WHERE profile_id = ? AND requirement_key IN (
		   SELECT entity_key FROM pending_change WHERE profile_id = ? AND entity_type = ?)`,
		profileID, profileID, entityRequirementDelete,
	); err != nil {
		return fmt.Errorf("reapply pending requirement deletes to links: %w", err)
	}
	return tx.Commit()
}

// reapplyRequirementSets makes each Test with a pending requirement_set cover
// exactly the keys that row asks for. A link Jira already has keeps its id, so
// a later removal can still delete it there.
func reapplyRequirementSets(tx *sql.Tx, profileID string) error {
	rows, err := tx.Query(
		`SELECT entity_key, after_val FROM pending_change
		 WHERE profile_id = ? AND entity_type = ?`,
		profileID, entityRequirementSet)
	if err != nil {
		return fmt.Errorf("read pending requirement sets: %w", err)
	}
	pending := map[string][]string{}
	for rows.Next() {
		var testKey, after string
		if err := rows.Scan(&testKey, &after); err != nil {
			rows.Close()
			return err
		}
		pending[testKey] = decodeKeys(after)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for testKey, keys := range pending {
		synced, err := requirementLinkIDs(tx, profileID, testKey)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(
			`DELETE FROM test_requirement WHERE profile_id = ? AND test_key = ?`,
			profileID, testKey,
		); err != nil {
			return fmt.Errorf("reapply requirement set for %s: %w", testKey, err)
		}
		for _, k := range keys {
			if _, err := tx.Exec(
				`INSERT OR REPLACE INTO test_requirement
				   (profile_id, test_key, requirement_key, link_id) VALUES (?, ?, ?, ?)`,
				profileID, testKey, k, synced[k], // empty link_id for a link Jira lacks
			); err != nil {
				return fmt.Errorf("reapply requirement link %s->%s: %w", testKey, k, err)
			}
		}
	}
	return nil
}

// requirementLinkIDs maps each requirement a Test is linked to in the cache to
// that link's Jira id.
func requirementLinkIDs(tx *sql.Tx, profileID, testKey string) (map[string]string, error) {
	rows, err := tx.Query(
		`SELECT requirement_key, link_id FROM test_requirement
		 WHERE profile_id = ? AND test_key = ?`,
		profileID, testKey)
	if err != nil {
		return nil, fmt.Errorf("read requirement links of %s: %w", testKey, err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var key, id string
		if err := rows.Scan(&key, &id); err != nil {
			return nil, err
		}
		out[key] = id
	}
	return out, rows.Err()
}
