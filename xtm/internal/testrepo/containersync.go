package testrepo

import (
	"database/sql"
	"fmt"
)

// The container sync writers. Both leave alone what the pending journal still
// owns, so a sync before a commit does not undo local work whose pending rows
// are still queued.

// UpsertContainers inserts or updates a batch of Test Sets / Plans /
// Executions (FR-1.3). A container with a pending delete stays gone.
func (r *Repository) UpsertContainers(profileID string, containers []Container) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// summary and environments are preserved when a pending container_edit or
	// container_env edit exists, so a sync can't clobber an uncommitted local
	// edit (mirrors the per-field guard in UpsertTests).
	// fix_versions is a plain synced field (read-only, never locally edited), so
	// it is overwritten unconditionally on conflict.
	stmt, err := tx.Prepare(
		`INSERT INTO test_container (profile_id, jira_key, kind, summary, status, parent_key, issue_type, parent_summary, labels, environments, fix_versions, created, updated, resolved, description)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(profile_id, jira_key) DO UPDATE SET
		   kind           = excluded.kind,
		   status         = excluded.status,
		   parent_key     = excluded.parent_key,
		   issue_type     = excluded.issue_type,
		   parent_summary = excluded.parent_summary,
		   labels         = excluded.labels,
		   fix_versions   = excluded.fix_versions,
		   created        = excluded.created,
		   updated        = excluded.updated,
		   resolved       = excluded.resolved,
		   description    = excluded.description,
		   summary        = CASE WHEN EXISTS (
		       SELECT 1 FROM pending_change
		       WHERE pending_change.profile_id  = excluded.profile_id
		         AND pending_change.entity_type = 'container_edit'
		         AND pending_change.entity_key  = excluded.jira_key
		         AND pending_change.field       = 'summary'
		     ) THEN test_container.summary ELSE excluded.summary END,
		   environments   = CASE WHEN EXISTS (
		       SELECT 1 FROM pending_change
		       WHERE pending_change.profile_id  = excluded.profile_id
		         AND pending_change.entity_type = 'container_env'
		         AND pending_change.entity_key  = excluded.jira_key
		     ) THEN test_container.environments ELSE excluded.environments END`)
	if err != nil {
		return fmt.Errorf("prepare upsert container: %w", err)
	}
	defer stmt.Close()

	for _, c := range containers {
		if _, err := stmt.Exec(profileID, c.Key, c.Kind, c.Summary, c.Status, c.ParentKey, c.IssueType, c.ParentSummary, encodeFixVersions(c.Labels), encodeEnvironments(c.Environments), encodeFixVersions(c.FixVersions), c.Created, c.Updated, c.Resolved, c.Description); err != nil {
			return fmt.Errorf("upsert container %s: %w", c.Key, err)
		}
	}
	if _, err := tx.Exec(
		`DELETE FROM test_container WHERE profile_id = ? AND jira_key IN (
		   SELECT entity_key FROM pending_change WHERE profile_id = ? AND entity_type = ?)`,
		profileID, profileID, entityContainerDelete,
	); err != nil {
		return fmt.Errorf("reapply pending container deletes: %w", err)
	}
	return tx.Commit()
}

// ReplaceAllContainerLinks wipes a profile's Test-to-Container memberships and
// rewrites them from the provided list, so memberships removed in Jira
// actually disappear on resync (mirrors the precondition link sweep). Members
// of a locally created container are kept, and pending membership and run
// changes are reapplied on top of the rewritten rows.
func (r *Repository) ReplaceAllContainerLinks(profileID string, links []ContainerLink) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// A locally created container's members exist nowhere but here.
	if _, err := tx.Exec(
		`DELETE FROM test_container_test WHERE profile_id = ? AND container_key NOT IN (
		   SELECT entity_key FROM pending_change WHERE profile_id = ? AND entity_type = ?)`,
		profileID, profileID, entityContainerAdd,
	); err != nil {
		return fmt.Errorf("clear container links: %w", err)
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
	if err := reapplyContainerLinkChanges(tx, profileID); err != nil {
		return err
	}
	return tx.Commit()
}

// stagedRunColumns are the test_container_test columns a pending run change
// stages its value in, by entity type. The pending row's field carries the
// column's name and its entity_key is "<execKey>:<testKey>".
var stagedRunColumns = map[string]string{
	entityTestRun:        "run_status",
	entityTestRunDefect:  "run_defects",
	entityTestRunComment: "run_comment",
}

// reapplyContainerLinkChanges puts the pending membership and run changes back
// on top of freshly synced membership rows.
func reapplyContainerLinkChanges(tx *sql.Tx, profileID string) error {
	if _, err := tx.Exec(
		`DELETE FROM test_container_test WHERE profile_id = ? AND container_key IN (
		   SELECT entity_key FROM pending_change WHERE profile_id = ? AND entity_type = ?)`,
		profileID, profileID, entityContainerDelete,
	); err != nil {
		return fmt.Errorf("reapply pending container deletes to links: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT OR IGNORE INTO test_container_test (profile_id, container_key, test_key, run_status)
		 SELECT p.profile_id, p.entity_key, m.value, ''
		 FROM pending_change p, json_each(p.after_val, '$.members') m
		 WHERE p.profile_id = ? AND p.entity_type = ? AND p.field = 'members'`,
		profileID, entityMembershipAdd,
	); err != nil {
		return fmt.Errorf("reapply pending membership adds: %w", err)
	}
	if _, err := tx.Exec(
		`DELETE FROM test_container_test WHERE profile_id = ? AND EXISTS (
		   SELECT 1 FROM pending_change p, json_each(p.after_val, '$.members') m
		   WHERE p.profile_id = test_container_test.profile_id
		     AND p.entity_type = ? AND p.field = 'members'
		     AND p.entity_key = test_container_test.container_key
		     AND m.value = test_container_test.test_key)`,
		profileID, entityMembershipRemove,
	); err != nil {
		return fmt.Errorf("reapply pending membership removes: %w", err)
	}
	for entityType, col := range stagedRunColumns {
		if _, err := tx.Exec(
			fmt.Sprintf(
				`UPDATE test_container_test SET %[1]s = p.after_val
				 FROM pending_change p
				 WHERE test_container_test.profile_id = ?
				   AND p.profile_id = test_container_test.profile_id
				   AND p.entity_type = ? AND p.field = '%[1]s'
				   AND p.entity_key = test_container_test.container_key || ':' || test_container_test.test_key`,
				col),
			profileID, entityType,
		); err != nil {
			return fmt.Errorf("reapply staged %s: %w", col, err)
		}
	}
	return nil
}
