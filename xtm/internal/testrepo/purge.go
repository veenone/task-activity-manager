package testrepo

import "fmt"

// profileScopedTables lists every table whose rows belong to a single profile,
// so a profile delete can remove all of its cached data (FR-5.3). It must name
// every table in the schema carrying a profile_id; the instruction gate fails
// when one is missing, and purge_test.go proves the rows actually go.
var profileScopedTables = []string{
	"sync_state",
	"test_folder",
	"test_case",
	"precondition",
	"test_precondition",
	"pending_change",
	"audit_log",
	"test_step",
	"test_dataset",
	"dataset_catalog_state",
	"dataset_catalog_test",
	"dataset_catalog_definition",
	"dataset_catalog_usage",
	"dataset_catalog_exec_plan",
	"test_container",
	"test_container_test",
	"saved_view",
	"custom_field",
	"test_custom_field",
	"sync_log",
	"test_review",

	// Requirements and the canonical set built from them.
	"requirement",
	"test_requirement",
	"requirement_source",
	"requirement_link",
	"canonical_requirement",
	"canonical_requirement_member",
	"canonical_version",
	"change_request",
	"cr_member_decision",

	// Duplicate scans and the pairs a reviewer dismissed.
	"duplicate_ignore",
	"duplicate_step_scan",
	"precondition_duplicate_ignore",

	// Bugs and their links to tests.
	"bug",
	"test_bug",

	// Execution data pulled back from the backend.
	"external_test",
	"test_run",
	"exec_plan",

	// Coverage parameters, their values and what they publish to.
	"coverage_project",
	"coverage_param_group",
	"coverage_parameter",
	"coverage_param_value",
	"coverage_value_test",
	"coverage_group_publication",

	"project_field_option",

	// external_ref maps this profile's local entities to the keys a publish
	// gave them on another instance. It goes with the profile: every local id
	// it points at is deleted here, so a kept row maps nothing to something,
	// and a profile recreated against the same project would inherit a
	// stranger's mappings for ids it reuses.
	"external_ref",
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

// CleanSampleContainers deletes the sample Test Sets / Plans / Executions that
// SeedSampleContainers created — their keys are "<projectKey>-SET-n",
// "-PLAN-n", "-EXEC-n", which real Jira issue keys ("<projectKey>-<number>")
// never match — plus their Test links. Real synced data is left untouched.
// Returns the number of containers removed.
func (r *Repository) CleanSampleContainers(profileID, projectKey string) (int, error) {
	if projectKey == "" {
		projectKey = "SAMPLE"
	}
	tx, err := r.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	removed := 0
	for _, infix := range []string{"-SET-%", "-PLAN-%", "-EXEC-%"} {
		pattern := projectKey + infix
		if _, err := tx.Exec(
			`DELETE FROM test_container_test WHERE profile_id = ? AND container_key LIKE ?`,
			profileID, pattern,
		); err != nil {
			return 0, fmt.Errorf("clean sample links: %w", err)
		}
		res, err := tx.Exec(
			`DELETE FROM test_container WHERE profile_id = ? AND jira_key LIKE ?`,
			profileID, pattern,
		)
		if err != nil {
			return 0, fmt.Errorf("clean sample containers: %w", err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			removed += int(n)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit clean sample data: %w", err)
	}
	return removed, nil
}
