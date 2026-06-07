package testrepo

import "fmt"

// profileScopedTables lists every table whose rows belong to a single profile,
// so a profile delete can remove all of its cached data (FR-5.3).
var profileScopedTables = []string{
	"sync_state",
	"test_folder",
	"test_case",
	"precondition",
	"test_precondition",
	"pending_change",
	"audit_log",
	"test_step",
	"test_container",
	"test_container_test",
	"saved_view",
	"custom_field",
	"test_custom_field",
	"sync_log",
	"test_review",
}

// PurgeProfile deletes every cached row belonging to a profile (FR-5.3) so its
// data doesn't linger after the profile is removed.
func (r *Repository) PurgeProfile(profileID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, table := range profileScopedTables {
		if _, err := tx.Exec(
			fmt.Sprintf("DELETE FROM %s WHERE profile_id = ?", table), profileID,
		); err != nil {
			return fmt.Errorf("purge %s: %w", table, err)
		}
	}
	return tx.Commit()
}
