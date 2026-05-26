package testrepo_test

import (
	"path/filepath"
	"testing"

	"xray-test-manager/internal/store"
	"xray-test-manager/internal/testrepo"
)

func newRepo(t *testing.T) *testrepo.Repository {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return testrepo.NewRepository(st)
}

func TestUpsertTestsIsIdempotent(t *testing.T) {
	repo := newRepo(t)
	tests := []testrepo.TestCase{
		{Key: "QA-1", ID: "1001", Summary: "Login works", Status: "Open"},
		{Key: "QA-2", ID: "1002", Summary: "Logout works", Status: "Done"},
	}
	if err := repo.UpsertTests("p1", tests); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	tests[0].Summary = "Login works correctly"
	if err := repo.UpsertTests("p1", tests); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	page, err := repo.ListTests("p1", testrepo.Query{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if page.Total != 2 {
		t.Errorf("Total = %d, want 2 (re-upsert must not duplicate rows)", page.Total)
	}
}

func TestListTestsSearchMatchesSummary(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "Login works"},
		{Key: "QA-2", ID: "2", Summary: "Logout works"},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	page, err := repo.ListTests("p1", testrepo.Query{Search: "Login"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(page.Tests) != 1 {
		t.Fatalf("got %d tests, want 1", len(page.Tests))
	}
	if page.Tests[0].Key != "QA-1" {
		t.Errorf("Key = %q, want QA-1", page.Tests[0].Key)
	}
}

func TestListTestsIsProfileScoped(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "Profile one test"},
	}); err != nil {
		t.Fatalf("upsert p1: %v", err)
	}
	if err := repo.UpsertTests("p2", []testrepo.TestCase{
		{Key: "QA-9", ID: "9", Summary: "Profile two test"},
	}); err != nil {
		t.Fatalf("upsert p2: %v", err)
	}

	page, err := repo.ListTests("p1", testrepo.Query{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if page.Total != 1 {
		t.Errorf("Total = %d, want 1 (p1 must not see p2 data)", page.Total)
	}
}

func TestGetTestUnknownKeyReturnsNotFound(t *testing.T) {
	repo := newRepo(t)

	_, err := repo.GetTest("p1", "QA-404")

	if err != testrepo.ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestListTestsFilterByLeafFolderReturnsOnlyThatFolder(t *testing.T) {
	repo := seedFolders(t)

	page, err := repo.ListTests("p1", testrepo.Query{FolderID: "/Authentication/Login"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if page.Total != 1 || page.Tests[0].Key != "QA-1" {
		t.Errorf("leaf filter returned %+v, want only QA-1", page)
	}
}

func TestListTestsFilterByCategoryFolderIncludesDescendants(t *testing.T) {
	repo := seedFolders(t)

	page, err := repo.ListTests("p1", testrepo.Query{FolderID: "/Authentication"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if page.Total != 2 {
		t.Errorf("category filter Total = %d, want 2 (QA-1, QA-2 under /Authentication/*)", page.Total)
	}
}

func TestListTestPreconditionsReturnsLinkedItems(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "Login test"},
	}); err != nil {
		t.Fatalf("upsert tests: %v", err)
	}
	if err := repo.UpsertPreconditions("p1", []testrepo.Precondition{
		{Key: "QA-P-1", Summary: "User account exists"},
		{Key: "QA-P-2", Summary: "Email service is available"},
	}); err != nil {
		t.Fatalf("upsert preconditions: %v", err)
	}
	if err := repo.ReplaceAllTestPreconditions("p1", map[string][]string{
		"QA-1": {"QA-P-1", "QA-P-2"},
	}); err != nil {
		t.Fatalf("replace links: %v", err)
	}

	got, err := repo.ListTestPreconditions("p1", "QA-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(got) != 2 {
		t.Errorf("got %d preconditions, want 2", len(got))
	}
}

func TestReplaceAllTestPreconditionsClearsStaleLinks(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "Login test"},
	}); err != nil {
		t.Fatalf("upsert tests: %v", err)
	}
	if err := repo.UpsertPreconditions("p1", []testrepo.Precondition{
		{Key: "QA-P-1", Summary: "Stale"},
	}); err != nil {
		t.Fatalf("upsert preconditions: %v", err)
	}
	if err := repo.ReplaceAllTestPreconditions("p1", map[string][]string{
		"QA-1": {"QA-P-1"},
	}); err != nil {
		t.Fatalf("first replace: %v", err)
	}

	// Re-run with an empty map — links must be cleared.
	if err := repo.ReplaceAllTestPreconditions("p1", map[string][]string{}); err != nil {
		t.Fatalf("second replace: %v", err)
	}

	got, err := repo.ListTestPreconditions("p1", "QA-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d preconditions, want 0 after clearing", len(got))
	}
}

func TestSetSyncStateReflectsCurrentRowCount(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "a"},
		{Key: "QA-2", ID: "2", Summary: "b"},
		{Key: "QA-3", ID: "3", Summary: "c"},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	if err := repo.SetSyncState("p1"); err != nil {
		t.Fatalf("set: %v", err)
	}

	state, err := repo.GetSyncState("p1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if state.TestCount != 3 {
		t.Errorf("TestCount = %d, want 3 (derived from current rows)", state.TestCount)
	}
	if state.LastSyncedAt == "" {
		t.Errorf("LastSyncedAt should be set, got empty")
	}
}

func TestEditTestFieldCreatesPendingChange(t *testing.T) {
	repo := seedTestForEditing(t)

	if err := repo.EditTestField("p1", "QA-1", "summary", "Login works (edited)"); err != nil {
		t.Fatalf("edit: %v", err)
	}

	changes, err := repo.ListPendingChanges("p1")
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("got %d pending, want 1", len(changes))
	}
	c := changes[0]
	if c.Field != "summary" {
		t.Errorf("Field = %q, want summary", c.Field)
	}
	if c.BeforeVal != "Login works" {
		t.Errorf("BeforeVal = %q, want %q", c.BeforeVal, "Login works")
	}
	if c.AfterVal != "Login works (edited)" {
		t.Errorf("AfterVal = %q, want %q", c.AfterVal, "Login works (edited)")
	}
}

func TestEditTestFieldCoalescesRepeatedEdits(t *testing.T) {
	repo := seedTestForEditing(t)

	if err := repo.EditTestField("p1", "QA-1", "summary", "first edit"); err != nil {
		t.Fatalf("first edit: %v", err)
	}
	if err := repo.EditTestField("p1", "QA-1", "summary", "second edit"); err != nil {
		t.Fatalf("second edit: %v", err)
	}

	changes, _ := repo.ListPendingChanges("p1")

	if len(changes) != 1 {
		t.Fatalf("got %d pending, want 1 (coalesced)", len(changes))
	}
	if changes[0].BeforeVal != "Login works" {
		t.Errorf("BeforeVal = %q, want original (not first edit)", changes[0].BeforeVal)
	}
	if changes[0].AfterVal != "second edit" {
		t.Errorf("AfterVal = %q, want %q", changes[0].AfterVal, "second edit")
	}
}

func TestEditTestFieldRevertingToOriginalRemovesPending(t *testing.T) {
	repo := seedTestForEditing(t)

	if err := repo.EditTestField("p1", "QA-1", "summary", "temporary"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if err := repo.EditTestField("p1", "QA-1", "summary", "Login works"); err != nil {
		t.Fatalf("revert: %v", err)
	}

	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 0 {
		t.Errorf("got %d pending, want 0 (reverted to original)", len(changes))
	}
}

func TestDiscardPendingChangeRevertsTheValue(t *testing.T) {
	repo := seedTestForEditing(t)

	if err := repo.EditTestField("p1", "QA-1", "summary", "edited"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 1 {
		t.Fatalf("expected 1 pending change before discard, got %d", len(changes))
	}

	if err := repo.DiscardPendingChange("p1", changes[0].ID); err != nil {
		t.Fatalf("discard: %v", err)
	}

	test, err := repo.GetTest("p1", "QA-1")
	if err != nil {
		t.Fatalf("get test: %v", err)
	}
	if test.Summary != "Login works" {
		t.Errorf("Summary = %q after discard, want %q", test.Summary, "Login works")
	}

	after, _ := repo.ListPendingChanges("p1")
	if len(after) != 0 {
		t.Errorf("got %d pending after discard, want 0", len(after))
	}
}

func TestEditTestFieldRejectsUnknownField(t *testing.T) {
	repo := seedTestForEditing(t)

	err := repo.EditTestField("p1", "QA-1", "bogus", "value")

	if err == nil {
		t.Error("editing an unknown field should error")
	}
}

func TestAuditLogRecordsEditAndDiscard(t *testing.T) {
	repo := seedTestForEditing(t)

	if err := repo.EditTestField("p1", "QA-1", "summary", "edited"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	changes, _ := repo.ListPendingChanges("p1")
	if err := repo.DiscardPendingChange("p1", changes[0].ID); err != nil {
		t.Fatalf("discard: %v", err)
	}

	entries, err := repo.ListAuditEntries("p1", 100)
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}

	var hasEdit, hasDiscard bool
	for _, e := range entries {
		if e.Action == "edit-local" {
			hasEdit = true
		}
		if e.Action == "discard-pending" {
			hasDiscard = true
		}
	}
	if !hasEdit {
		t.Error("audit log missing edit-local entry")
	}
	if !hasDiscard {
		t.Error("audit log missing discard-pending entry")
	}
}

func TestCommitPendingChangesClearsRowsAndWritesCommitAudit(t *testing.T) {
	repo := seedTestForEditing(t)

	if err := repo.EditTestField("p1", "QA-1", "summary", "edited summary"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 1 {
		t.Fatalf("expected 1 pending before commit, got %d", len(changes))
	}

	if err := repo.CommitPendingChanges("p1", []int64{changes[0].ID}); err != nil {
		t.Fatalf("commit: %v", err)
	}

	after, _ := repo.ListPendingChanges("p1")
	if len(after) != 0 {
		t.Errorf("expected 0 pending after commit, got %d", len(after))
	}

	entries, _ := repo.ListAuditEntries("p1", 100)
	var hasCommit bool
	for _, e := range entries {
		if e.Action == "commit" && e.Field == "summary" && e.AfterVal == "edited summary" {
			hasCommit = true
		}
	}
	if !hasCommit {
		t.Error("audit log missing commit entry for the committed change")
	}
}

func TestCommitPendingChangesIsIdempotentForUnknownIDs(t *testing.T) {
	repo := seedTestForEditing(t)

	// No edits made, so id=999 doesn't exist — commit should still succeed
	// without error (idempotent).
	if err := repo.CommitPendingChanges("p1", []int64{999}); err != nil {
		t.Errorf("commit of non-existent id should be a no-op, got %v", err)
	}
}

func TestUpsertTestsPreservesPendingFieldValuesOnResync(t *testing.T) {
	repo := newRepo(t)

	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "Original", Status: "Open", Priority: "Medium"},
	}); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	if err := repo.EditTestField("p1", "QA-1", "summary", "Local edit"); err != nil {
		t.Fatalf("local edit: %v", err)
	}

	// A subsequent sync brings new remote values for BOTH summary (which
	// has a pending edit) and status (which doesn't).
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "Remote changed it too", Status: "In Progress", Priority: "Medium"},
	}); err != nil {
		t.Fatalf("re-sync: %v", err)
	}

	got, err := repo.GetTest("p1", "QA-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Summary != "Local edit" {
		t.Errorf("Summary = %q after re-sync, want %q (pending edit must survive sync)",
			got.Summary, "Local edit")
	}
	if got.Status != "In Progress" {
		t.Errorf("Status = %q after re-sync, want %q (non-pending field should update)",
			got.Status, "In Progress")
	}
}

func TestUpsertTestsOverwritesAllFieldsWhenNoPending(t *testing.T) {
	repo := newRepo(t)

	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "Original", Status: "Open"},
	}); err != nil {
		t.Fatalf("initial sync: %v", err)
	}

	// No pending edits, so the next sync should overwrite freely.
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "Updated", Status: "Done"},
	}); err != nil {
		t.Fatalf("re-sync: %v", err)
	}

	got, _ := repo.GetTest("p1", "QA-1")
	if got.Summary != "Updated" || got.Status != "Done" {
		t.Errorf("got Summary=%q Status=%q; both should be the new values",
			got.Summary, got.Status)
	}
}

func TestUpsertTestsPreservesPendingForOneFieldButUpdatesAnother(t *testing.T) {
	repo := newRepo(t)

	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "Orig summary", Description: "Orig desc", Priority: "Low"},
	}); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	// User only edits summary.
	if err := repo.EditTestField("p1", "QA-1", "summary", "User summary"); err != nil {
		t.Fatalf("edit: %v", err)
	}

	// Sync changes summary, description, and priority on the remote.
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "Remote summary", Description: "Remote desc", Priority: "High"},
	}); err != nil {
		t.Fatalf("re-sync: %v", err)
	}

	got, _ := repo.GetTest("p1", "QA-1")
	if got.Summary != "User summary" {
		t.Errorf("summary should stay edited: got %q", got.Summary)
	}
	if got.Description != "Remote desc" {
		t.Errorf("description has no pending — should take remote, got %q", got.Description)
	}
	if got.Priority != "High" {
		t.Errorf("priority has no pending — should take remote, got %q", got.Priority)
	}
}

func TestBulkEditSetsFieldOnAllSelected(t *testing.T) {
	repo := seedBulkTests(t)

	result, err := repo.BulkEditTests(
		"p1",
		[]string{"QA-1", "QA-2", "QA-3"},
		testrepo.BulkEdit{Operation: "set", Field: "priority", Value: "Critical"},
	)
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}

	if len(result.Succeeded) != 3 {
		t.Errorf("succeeded = %d, want 3", len(result.Succeeded))
	}
	for _, key := range []string{"QA-1", "QA-2", "QA-3"} {
		got, _ := repo.GetTest("p1", key)
		if got.Priority != "Critical" {
			t.Errorf("%s priority = %q, want Critical", key, got.Priority)
		}
	}

	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 3 {
		t.Errorf("pending = %d, want 3 (one per test)", len(changes))
	}
}

func TestBulkEditAddLabelIsNoopWhenAlreadyPresent(t *testing.T) {
	repo := seedBulkTests(t)
	// QA-1 already has the label; QA-2 doesn't.
	if err := repo.EditTestField("p1", "QA-1", "labels", "smoke regression"); err != nil {
		t.Fatalf("seed labels: %v", err)
	}

	result, err := repo.BulkEditTests(
		"p1",
		[]string{"QA-1", "QA-2"},
		testrepo.BulkEdit{Operation: "add_label", Value: "smoke"},
	)
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}

	// Both succeed: QA-1 is a no-op (already present), QA-2 gets the label.
	if len(result.Succeeded) != 2 {
		t.Errorf("succeeded = %d, want 2", len(result.Succeeded))
	}

	qa1, _ := repo.GetTest("p1", "QA-1")
	qa2, _ := repo.GetTest("p1", "QA-2")
	if !containsString(qa1.Labels, "smoke") {
		t.Errorf("QA-1 should still have smoke: %v", qa1.Labels)
	}
	if !containsString(qa2.Labels, "smoke") {
		t.Errorf("QA-2 should have smoke after add: %v", qa2.Labels)
	}
}

func TestBulkEditRemoveLabelRemovesWhenPresent(t *testing.T) {
	repo := seedBulkTests(t)
	if err := repo.EditTestField("p1", "QA-1", "labels", "smoke regression"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := repo.BulkEditTests(
		"p1",
		[]string{"QA-1", "QA-2"},
		testrepo.BulkEdit{Operation: "remove_label", Value: "smoke"},
	); err != nil {
		t.Fatalf("bulk: %v", err)
	}

	qa1, _ := repo.GetTest("p1", "QA-1")
	if containsString(qa1.Labels, "smoke") {
		t.Errorf("QA-1 should no longer have smoke: %v", qa1.Labels)
	}
}

func TestBulkEditReportsFailureForUnknownTest(t *testing.T) {
	repo := seedBulkTests(t)

	result, err := repo.BulkEditTests(
		"p1",
		[]string{"QA-1", "QA-NONEXIST"},
		testrepo.BulkEdit{Operation: "set", Field: "summary", Value: "New"},
	)
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}

	if len(result.Succeeded) != 1 || result.Succeeded[0] != "QA-1" {
		t.Errorf("succeeded = %v, want [QA-1]", result.Succeeded)
	}
	if len(result.Failed) != 1 || result.Failed[0].TestKey != "QA-NONEXIST" {
		t.Errorf("failed = %v, want one entry for QA-NONEXIST", result.Failed)
	}
}

func TestBulkEditAppendIsRejectedForNonDescriptionFields(t *testing.T) {
	repo := seedBulkTests(t)

	result, err := repo.BulkEditTests(
		"p1",
		[]string{"QA-1"},
		testrepo.BulkEdit{Operation: "append", Field: "summary", Value: "extra"},
	)
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if len(result.Failed) != 1 {
		t.Errorf("append-on-summary should fail; got %+v", result)
	}
}

func seedBulkTests(t *testing.T) *testrepo.Repository {
	t.Helper()
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "First", Priority: "Low"},
		{Key: "QA-2", ID: "2", Summary: "Second", Priority: "Low"},
		{Key: "QA-3", ID: "3", Summary: "Third", Priority: "Low"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return repo
}

func containsString(s []string, want string) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}

// seedFolders populates a fresh repo with three tests across two folder
// branches so the FolderID filter tests can exercise leaf and ancestor
// selections.
func seedFolders(t *testing.T) *testrepo.Repository {
	t.Helper()
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "Login test", FolderID: "/Authentication/Login"},
		{Key: "QA-2", ID: "2", Summary: "Logout test", FolderID: "/Authentication/Logout"},
		{Key: "QA-3", ID: "3", Summary: "Search test", FolderID: "/Browse/Search"},
	}); err != nil {
		t.Fatalf("seed upsert: %v", err)
	}
	return repo
}

// seedTestForEditing creates one Test with known values so editing tests can
// assert on the BeforeVal / AfterVal transitions.
func seedTestForEditing(t *testing.T) *testrepo.Repository {
	t.Helper()
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{
			Key:         "QA-1",
			ID:          "1",
			Summary:     "Login works",
			Description: "original",
			Priority:    "Medium",
		},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return repo
}
