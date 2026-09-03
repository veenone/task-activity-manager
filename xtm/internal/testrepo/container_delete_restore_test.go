package testrepo

import "testing"

// TestContainerDeleteRestoreCarriesParent verifies that deleting a sub-task Test
// Execution and then discarding the pending delete restores its parent_key /
// issue_type, not just kind/summary/status.
func TestContainerDeleteRestoreCarriesParent(t *testing.T) {
	r := newTestRepo(t) // shared helper in sankey_crossproject_test.go
	const p = "p1"

	in := []Container{
		{Key: "DEMO-STE-1", Kind: "testexec", Summary: "Sub", Status: "Open",
			ParentKey: "DEMO-S-1", IssueType: "Sub Test Execution"},
	}
	if err := r.UpsertContainers(p, in); err != nil {
		t.Fatalf("UpsertContainers: %v", err)
	}

	if err := r.DeleteContainer(p, "DEMO-STE-1"); err != nil {
		t.Fatalf("DeleteContainer: %v", err)
	}

	// Find the pending container_delete row so we can discard it (the restore
	// path).
	var changeID int64
	if err := r.db.QueryRow(
		`SELECT id FROM pending_change
		 WHERE profile_id = ? AND entity_type = ? AND entity_key = ?`,
		p, entityContainerDelete, "DEMO-STE-1",
	).Scan(&changeID); err != nil {
		t.Fatalf("locate pending container_delete: %v", err)
	}

	if err := r.DiscardPendingChange(p, changeID); err != nil {
		t.Fatalf("DiscardPendingChange: %v", err)
	}

	got, err := r.ListContainers(p, "testexec")
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	var restored *Container
	for i := range got {
		if got[i].Key == "DEMO-STE-1" {
			restored = &got[i]
			break
		}
	}
	if restored == nil {
		t.Fatalf("container DEMO-STE-1 not restored")
	}
	if restored.ParentKey != "DEMO-S-1" || restored.IssueType != "Sub Test Execution" {
		t.Errorf("restored sub-task exec lost parent/issuetype: %+v", *restored)
	}
}
