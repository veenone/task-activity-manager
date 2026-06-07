package testrepo

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// containerDeleteSnapshot captures a Container plus its memberships so a
// discarded delete can restore it.
type containerDeleteSnapshot struct {
	Kind    string   `json:"kind"`
	Summary string   `json:"summary"`
	Status  string   `json:"status"`
	Members []string `json:"members"`
}

// EditContainer renames a Test Set / Plan / Execution (edits its summary) and
// queues the change for commit. Reverting to the original summary drops the
// pending change.
func (r *Repository) EditContainer(profileID, key, summary string) error {
	if strings.TrimSpace(summary) == "" {
		return fmt.Errorf("a name is required")
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var current string
	err = tx.QueryRow(
		`SELECT summary FROM test_container WHERE profile_id = ? AND jira_key = ?`,
		profileID, key,
	).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("container %s not found", key)
	}
	if err != nil {
		return fmt.Errorf("read container: %w", err)
	}
	if current == summary {
		return nil
	}

	if _, err := tx.Exec(
		`UPDATE test_container SET summary = ? WHERE profile_id = ? AND jira_key = ?`,
		summary, profileID, key,
	); err != nil {
		return fmt.Errorf("update container: %w", err)
	}

	// If the container is itself a not-yet-committed create, fold the rename
	// into the create payload instead of queuing a separate edit.
	folded, err := foldContainerRenameIntoAdd(tx, profileID, key, summary)
	if err != nil {
		return err
	}
	if !folded {
		if err := upsertPendingChange(
			tx, profileID, entityContainerEdit, key, "summary", current, summary, "",
		); err != nil {
			return err
		}
	}
	if err := writeAudit(
		tx, profileID, entityContainerEdit, key, "rename-container-local", "summary", current, summary, "",
	); err != nil {
		return err
	}
	return tx.Commit()
}

// foldContainerRenameIntoAdd rewrites the summary inside a pending
// test_container_add row, returning false when no such row exists.
func foldContainerRenameIntoAdd(tx *sql.Tx, profileID, key, summary string) (bool, error) {
	var afterVal string
	err := tx.QueryRow(
		`SELECT after_val FROM pending_change
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = 'container'`,
		profileID, entityContainerAdd, key,
	).Scan(&afterVal)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read pending container add: %w", err)
	}
	var payload map[string]any
	if uErr := json.Unmarshal([]byte(afterVal), &payload); uErr != nil {
		return false, fmt.Errorf("decode container add: %w", uErr)
	}
	payload["summary"] = summary
	encoded, mErr := json.Marshal(payload)
	if mErr != nil {
		return false, fmt.Errorf("encode container add: %w", mErr)
	}
	if _, uErr := tx.Exec(
		`UPDATE pending_change SET after_val = ?
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = 'container'`,
		string(encoded), profileID, entityContainerAdd, key,
	); uErr != nil {
		return false, fmt.Errorf("update container add: %w", uErr)
	}
	return true, nil
}

// DeleteContainer removes a Test Set / Plan / Execution and its memberships and
// queues the deletion for commit. Deleting a container that was only created
// locally cancels the create instead of queuing a remote delete.
func (r *Repository) DeleteContainer(profileID, key string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var snap containerDeleteSnapshot
	err = tx.QueryRow(
		`SELECT kind, summary, status FROM test_container WHERE profile_id = ? AND jira_key = ?`,
		profileID, key,
	).Scan(&snap.Kind, &snap.Summary, &snap.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("container %s not found", key)
	}
	if err != nil {
		return fmt.Errorf("read container: %w", err)
	}
	snap.Members, err = containerMembers(tx, profileID, key)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(
		`DELETE FROM test_container WHERE profile_id = ? AND jira_key = ?`, profileID, key,
	); err != nil {
		return fmt.Errorf("delete container: %w", err)
	}
	if _, err := tx.Exec(
		`DELETE FROM test_container_test WHERE profile_id = ? AND container_key = ?`, profileID, key,
	); err != nil {
		return fmt.Errorf("delete container memberships: %w", err)
	}

	// If this container was only ever created locally, cancel the create and
	// its membership/edit pending rows instead of queuing a remote delete.
	var addID int64
	addErr := tx.QueryRow(
		`SELECT id FROM pending_change
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ? AND field = 'container'`,
		profileID, entityContainerAdd, key,
	).Scan(&addID)
	if addErr == nil {
		for _, et := range []string{entityContainerAdd, entityMembershipAdd, entityMembershipRemove, entityContainerEdit} {
			if _, err := tx.Exec(
				`DELETE FROM pending_change WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
				profileID, et, key,
			); err != nil {
				return fmt.Errorf("cancel container create rows: %w", err)
			}
		}
		if err := writeAudit(
			tx, profileID, entityContainerAdd, key, "container-create-cancelled", "container", snap.Summary, "", "",
		); err != nil {
			return err
		}
		return tx.Commit()
	}
	if !errors.Is(addErr, sql.ErrNoRows) {
		return fmt.Errorf("probe pending container add: %w", addErr)
	}

	// Committed container: drop any superseded edit/membership rows, then queue
	// the delete with a snapshot for discard.
	for _, et := range []string{entityContainerEdit, entityMembershipAdd, entityMembershipRemove} {
		if _, err := tx.Exec(
			`DELETE FROM pending_change WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
			profileID, et, key,
		); err != nil {
			return fmt.Errorf("clear superseded container rows: %w", err)
		}
	}
	encoded, _ := json.Marshal(snap)
	if err := upsertPendingChange(
		tx, profileID, entityContainerDelete, key, "container", string(encoded), "1", "",
	); err != nil {
		return err
	}
	if err := writeAudit(
		tx, profileID, entityContainerDelete, key, "delete-container-local", "container", string(encoded), "", "",
	); err != nil {
		return err
	}
	return tx.Commit()
}

// containerMembers returns the Test keys linked to a Container.
func containerMembers(tx *sql.Tx, profileID, key string) ([]string, error) {
	rows, err := tx.Query(
		`SELECT test_key FROM test_container_test WHERE profile_id = ? AND container_key = ?`,
		profileID, key,
	)
	if err != nil {
		return nil, fmt.Errorf("read container members: %w", err)
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
