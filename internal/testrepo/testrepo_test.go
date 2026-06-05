package testrepo_test

import (
	"fmt"
	"path/filepath"
	"strings"
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

func TestListContainersForTestReturnsMembershipsWithRunStatus(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "Login test"},
	}); err != nil {
		t.Fatalf("upsert tests: %v", err)
	}
	if err := repo.UpsertContainers("p1", []testrepo.Container{
		{Key: "QA-TS-1", Kind: "testset", Summary: "Auth set", Status: "Open"},
		{Key: "QA-TE-1", Kind: "testexec", Summary: "Cycle 1", Status: "In Progress"},
	}); err != nil {
		t.Fatalf("upsert containers: %v", err)
	}
	if err := repo.ReplaceAllContainerLinks("p1", []testrepo.ContainerLink{
		{ContainerKey: "QA-TS-1", TestKey: "QA-1"},
		{ContainerKey: "QA-TE-1", TestKey: "QA-1", RunStatus: "PASS"},
	}); err != nil {
		t.Fatalf("replace links: %v", err)
	}

	got, err := repo.ListContainersForTest("p1", "QA-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d memberships, want 2", len(got))
	}
}

func TestListContainersForTestCarriesExecutionRunStatus(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "Login test"},
	}); err != nil {
		t.Fatalf("upsert tests: %v", err)
	}
	if err := repo.UpsertContainers("p1", []testrepo.Container{
		{Key: "QA-TE-1", Kind: "testexec", Summary: "Cycle 1", Status: "In Progress"},
	}); err != nil {
		t.Fatalf("upsert containers: %v", err)
	}
	if err := repo.ReplaceAllContainerLinks("p1", []testrepo.ContainerLink{
		{ContainerKey: "QA-TE-1", TestKey: "QA-1", RunStatus: "FAIL"},
	}); err != nil {
		t.Fatalf("replace links: %v", err)
	}

	got, _ := repo.ListContainersForTest("p1", "QA-1")
	if got[0].RunStatus != "FAIL" {
		t.Errorf("RunStatus = %q, want FAIL", got[0].RunStatus)
	}
}

func TestReplaceAllContainerLinksClearsStaleMemberships(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "Login test"},
	}); err != nil {
		t.Fatalf("upsert tests: %v", err)
	}
	if err := repo.UpsertContainers("p1", []testrepo.Container{
		{Key: "QA-TS-1", Kind: "testset", Summary: "Auth set", Status: "Open"},
	}); err != nil {
		t.Fatalf("upsert containers: %v", err)
	}
	if err := repo.ReplaceAllContainerLinks("p1", []testrepo.ContainerLink{
		{ContainerKey: "QA-TS-1", TestKey: "QA-1"},
	}); err != nil {
		t.Fatalf("first replace: %v", err)
	}

	if err := repo.ReplaceAllContainerLinks("p1", []testrepo.ContainerLink{}); err != nil {
		t.Fatalf("second replace: %v", err)
	}

	got, _ := repo.ListContainersForTest("p1", "QA-1")
	if len(got) != 0 {
		t.Errorf("got %d memberships, want 0 after clearing", len(got))
	}
}

func TestAllocateTestsAddsMembershipAndQueuesPending(t *testing.T) {
	repo := seedTestsAndContainer(t)

	result, err := repo.AllocateTests("p1", "QA-TS-1", []string{"QA-1", "QA-2"})
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	if len(result.Added) != 2 {
		t.Errorf("Added = %v, want 2", result.Added)
	}

	members, _ := repo.ListContainersForTest("p1", "QA-1")
	if len(members) != 1 || members[0].Key != "QA-TS-1" {
		t.Errorf("QA-1 memberships = %+v, want [QA-TS-1]", members)
	}

	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 1 || changes[0].EntityType != "test_membership_add" || changes[0].EntityKey != "QA-TS-1" {
		t.Fatalf("pending = %+v, want one test_membership_add for QA-TS-1", changes)
	}
}

func TestAllocateTestsSkipsExistingMembers(t *testing.T) {
	repo := seedTestsAndContainer(t)
	if err := repo.ReplaceAllContainerLinks("p1", []testrepo.ContainerLink{
		{ContainerKey: "QA-TS-1", TestKey: "QA-1"},
	}); err != nil {
		t.Fatalf("seed link: %v", err)
	}

	result, err := repo.AllocateTests("p1", "QA-TS-1", []string{"QA-1", "QA-2"})
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	if len(result.Added) != 1 || result.Added[0] != "QA-2" {
		t.Errorf("Added = %v, want [QA-2]", result.Added)
	}
	if len(result.AlreadyMembers) != 1 || result.AlreadyMembers[0] != "QA-1" {
		t.Errorf("AlreadyMembers = %v, want [QA-1]", result.AlreadyMembers)
	}
}

func TestAllocateTestsCoalescesAcrossCalls(t *testing.T) {
	repo := seedTestsAndContainer(t)

	if _, err := repo.AllocateTests("p1", "QA-TS-1", []string{"QA-1"}); err != nil {
		t.Fatalf("allocate 1: %v", err)
	}
	if _, err := repo.AllocateTests("p1", "QA-TS-1", []string{"QA-2"}); err != nil {
		t.Fatalf("allocate 2: %v", err)
	}

	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 1 {
		t.Fatalf("want 1 coalesced pending row, got %d", len(changes))
	}
	if !strings.Contains(changes[0].AfterVal, "QA-1") || !strings.Contains(changes[0].AfterVal, "QA-2") {
		t.Errorf("payload = %q, want both QA-1 and QA-2", changes[0].AfterVal)
	}
}

func TestDiscardAllocationRemovesAddedMemberships(t *testing.T) {
	repo := seedTestsAndContainer(t)
	if _, err := repo.AllocateTests("p1", "QA-TS-1", []string{"QA-1"}); err != nil {
		t.Fatalf("allocate: %v", err)
	}
	changes, _ := repo.ListPendingChanges("p1")

	if err := repo.DiscardPendingChange("p1", changes[0].ID); err != nil {
		t.Fatalf("discard: %v", err)
	}

	members, _ := repo.ListContainersForTest("p1", "QA-1")
	if len(members) != 0 {
		t.Errorf("discarding the allocation should remove the membership; got %+v", members)
	}
}

func seedTestsAndContainer(t *testing.T) *testrepo.Repository {
	t.Helper()
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1"},
		{Key: "QA-2", ID: "2"},
	}); err != nil {
		t.Fatalf("seed tests: %v", err)
	}
	if err := repo.UpsertContainers("p1", []testrepo.Container{
		{Key: "QA-TS-1", Kind: "testset", Summary: "Auth set", Status: "Open"},
	}); err != nil {
		t.Fatalf("seed container: %v", err)
	}
	return repo
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

func TestEditTestStepFieldQueuesStepPendingChange(t *testing.T) {
	repo := seedTestWithSteps(t)

	if err := repo.EditTestStepField("p1", "QA-1", "s1", "action", "new action"); err != nil {
		t.Fatalf("edit step: %v", err)
	}

	steps, _ := repo.ListTestSteps("p1", "QA-1")
	if steps[0].Action != "new action" {
		t.Errorf("step action = %q, want %q", steps[0].Action, "new action")
	}

	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 1 {
		t.Fatalf("want 1 pending change, got %d", len(changes))
	}
	c := changes[0]
	if c.EntityType != "test_step" || c.EntityKey != "QA-1:s1" || c.Field != "action" {
		t.Errorf("change = %+v, want entity_type=test_step entity_key=QA-1:s1 field=action", c)
	}
}

func TestDiscardStepPendingChangeRevertsStepField(t *testing.T) {
	repo := seedTestWithSteps(t)

	if err := repo.EditTestStepField("p1", "QA-1", "s1", "expected", "new expected"); err != nil {
		t.Fatalf("edit step: %v", err)
	}
	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 1 {
		t.Fatalf("want 1 pending, got %d", len(changes))
	}

	if err := repo.DiscardPendingChange("p1", changes[0].ID); err != nil {
		t.Fatalf("discard: %v", err)
	}

	steps, _ := repo.ListTestSteps("p1", "QA-1")
	if steps[0].Expected != "old expected" {
		t.Errorf("step expected = %q after discard, want %q (reverted)", steps[0].Expected, "old expected")
	}
}

func TestDeleteTestStepHidesStepAndQueuesDelete(t *testing.T) {
	repo := seedTestWithSteps(t)

	if err := repo.DeleteTestStep("p1", "QA-1", "s1"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	steps, _ := repo.ListTestSteps("p1", "QA-1")
	if len(steps) != 0 {
		t.Errorf("expected step hidden locally; got %+v", steps)
	}

	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 1 {
		t.Fatalf("want 1 pending change, got %d", len(changes))
	}
	if changes[0].EntityType != "test_step_delete" || changes[0].EntityKey != "QA-1:s1" {
		t.Errorf("change = %+v, want entity_type=test_step_delete entity_key=QA-1:s1", changes[0])
	}
}

func TestDiscardStepDeleteRestoresStepFromSnapshot(t *testing.T) {
	repo := seedTestWithSteps(t)

	if err := repo.DeleteTestStep("p1", "QA-1", "s1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	changes, _ := repo.ListPendingChanges("p1")

	if err := repo.DiscardPendingChange("p1", changes[0].ID); err != nil {
		t.Fatalf("discard: %v", err)
	}

	steps, _ := repo.ListTestSteps("p1", "QA-1")
	if len(steps) != 1 {
		t.Fatalf("want step restored; got %+v", steps)
	}
	got := steps[0]
	if got.XrayID != "s1" || got.Action != "old action" || got.Expected != "old expected" {
		t.Errorf("restored step = %+v, want {XrayID:s1 Action:old action Expected:old expected ...}", got)
	}
}

func TestAddTestStepAppendsStepAndQueuesAdd(t *testing.T) {
	repo := seedTestWithSteps(t)

	added, err := repo.AddTestStep("p1", "QA-1", "new action", "new data", "new expected")
	if err != nil {
		t.Fatalf("add step: %v", err)
	}

	if added.Index != 2 {
		t.Errorf("new step Index = %d, want 2 (appended after the seeded step)", added.Index)
	}

	steps, _ := repo.ListTestSteps("p1", "QA-1")
	if len(steps) != 2 {
		t.Fatalf("want 2 steps after add, got %d", len(steps))
	}

	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 1 {
		t.Fatalf("want 1 pending change, got %d", len(changes))
	}
	c := changes[0]
	if c.EntityType != "test_step_add" || c.EntityKey != "QA-1:"+added.XrayID || c.Field != "step" {
		t.Errorf("change = %+v, want entity_type=test_step_add entity_key=QA-1:%s field=step", c, added.XrayID)
	}
}

func TestEditNewStepFoldsIntoAddInsteadOfNewPending(t *testing.T) {
	repo := seedTestWithSteps(t)
	added, err := repo.AddTestStep("p1", "QA-1", "", "", "")
	if err != nil {
		t.Fatalf("add step: %v", err)
	}

	if err := repo.EditTestStepField("p1", "QA-1", added.XrayID, "action", "typed action"); err != nil {
		t.Fatalf("edit new step: %v", err)
	}

	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 1 {
		t.Fatalf("want 1 pending change (edit folded into add), got %d", len(changes))
	}
	if changes[0].EntityType != "test_step_add" {
		t.Errorf("pending entity_type = %q, want test_step_add (no standalone edit row)", changes[0].EntityType)
	}
	if !strings.Contains(changes[0].AfterVal, "typed action") {
		t.Errorf("add payload = %q, want it to carry the edited action", changes[0].AfterVal)
	}
}

func TestDeleteNewStepCancelsAdd(t *testing.T) {
	repo := seedTestWithSteps(t)
	added, err := repo.AddTestStep("p1", "QA-1", "x", "", "")
	if err != nil {
		t.Fatalf("add step: %v", err)
	}

	if err := repo.DeleteTestStep("p1", "QA-1", added.XrayID); err != nil {
		t.Fatalf("delete new step: %v", err)
	}

	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 0 {
		t.Errorf("add-then-delete should leave no pending rows; got %+v", changes)
	}

	steps, _ := repo.ListTestSteps("p1", "QA-1")
	if len(steps) != 1 {
		t.Errorf("want only the seeded step left, got %d", len(steps))
	}
}

func TestDiscardAddRemovesNewStep(t *testing.T) {
	repo := seedTestWithSteps(t)
	if _, err := repo.AddTestStep("p1", "QA-1", "x", "", ""); err != nil {
		t.Fatalf("add step: %v", err)
	}
	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 1 {
		t.Fatalf("want 1 pending, got %d", len(changes))
	}

	if err := repo.DiscardPendingChange("p1", changes[0].ID); err != nil {
		t.Fatalf("discard add: %v", err)
	}

	steps, _ := repo.ListTestSteps("p1", "QA-1")
	if len(steps) != 1 {
		t.Errorf("discarding the add should remove the new step; got %d steps", len(steps))
	}
}

func TestDeleteCommittedStepDropsSupersededEditRows(t *testing.T) {
	repo := seedTestWithSteps(t)

	if err := repo.EditTestStepField("p1", "QA-1", "s1", "action", "edited then deleted"); err != nil {
		t.Fatalf("edit step: %v", err)
	}
	if err := repo.DeleteTestStep("p1", "QA-1", "s1"); err != nil {
		t.Fatalf("delete step: %v", err)
	}

	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 1 {
		t.Fatalf("want 1 pending row after edit-then-delete, got %d (%+v)", len(changes), changes)
	}
	if changes[0].EntityType != "test_step_delete" {
		t.Errorf("remaining pending = %q, want test_step_delete (the edit row is superseded)", changes[0].EntityType)
	}
}

func TestReorderTestStepsUpdatesIndicesAndQueuesOrder(t *testing.T) {
	repo := seedTestWithThreeSteps(t)

	if err := repo.ReorderTestSteps("p1", "QA-1", []string{"s3", "s1", "s2"}); err != nil {
		t.Fatalf("reorder: %v", err)
	}

	steps, _ := repo.ListTestSteps("p1", "QA-1")
	got := []string{steps[0].XrayID, steps[1].XrayID, steps[2].XrayID}
	want := []string{"s3", "s1", "s2"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}

	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 1 {
		t.Fatalf("want 1 pending change, got %d", len(changes))
	}
	c := changes[0]
	if c.EntityType != "test_step_order" || c.EntityKey != "QA-1" || c.Field != "order" {
		t.Errorf("change = %+v, want entity_type=test_step_order entity_key=QA-1 field=order", c)
	}
	if !strings.Contains(c.AfterVal, "s3") {
		t.Errorf("after_val = %q, want it to carry the new order", c.AfterVal)
	}
}

func TestReorderToOriginalOrderDropsPending(t *testing.T) {
	repo := seedTestWithThreeSteps(t)

	if err := repo.ReorderTestSteps("p1", "QA-1", []string{"s3", "s1", "s2"}); err != nil {
		t.Fatalf("reorder: %v", err)
	}
	if err := repo.ReorderTestSteps("p1", "QA-1", []string{"s1", "s2", "s3"}); err != nil {
		t.Fatalf("reorder back: %v", err)
	}

	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 0 {
		t.Errorf("reordering back to original should drop the pending row; got %+v", changes)
	}
}

func TestDiscardReorderRestoresOriginalOrder(t *testing.T) {
	repo := seedTestWithThreeSteps(t)

	if err := repo.ReorderTestSteps("p1", "QA-1", []string{"s3", "s2", "s1"}); err != nil {
		t.Fatalf("reorder: %v", err)
	}
	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 1 {
		t.Fatalf("want 1 pending, got %d", len(changes))
	}

	if err := repo.DiscardPendingChange("p1", changes[0].ID); err != nil {
		t.Fatalf("discard: %v", err)
	}

	steps, _ := repo.ListTestSteps("p1", "QA-1")
	got := []string{steps[0].XrayID, steps[1].XrayID, steps[2].XrayID}
	want := []string{"s1", "s2", "s3"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order after discard = %v, want original %v", got, want)
		}
	}
}

func TestReorderRejectsMismatchedSet(t *testing.T) {
	repo := seedTestWithThreeSteps(t)

	err := repo.ReorderTestSteps("p1", "QA-1", []string{"s1", "s2"})
	if err == nil {
		t.Error("reorder with a step missing from the set should error")
	}
}

func seedTestWithThreeSteps(t *testing.T) *testrepo.Repository {
	t.Helper()
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "X", Status: "Open"},
	}); err != nil {
		t.Fatalf("seed test: %v", err)
	}
	if err := repo.SetTestSteps("p1", "QA-1", []testrepo.Step{
		{XrayID: "s1", Index: 1, Action: "first"},
		{XrayID: "s2", Index: 2, Action: "second"},
		{XrayID: "s3", Index: 3, Action: "third"},
	}); err != nil {
		t.Fatalf("seed steps: %v", err)
	}
	return repo
}

func TestEditTestStepFieldRejectsUnknownField(t *testing.T) {
	repo := seedTestWithSteps(t)

	err := repo.EditTestStepField("p1", "QA-1", "s1", "summary", "x")
	if err == nil {
		t.Errorf("expected error for unknown step field, got nil")
	}
}

func seedTestWithSteps(t *testing.T) *testrepo.Repository {
	t.Helper()
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "X", Status: "Open"},
	}); err != nil {
		t.Fatalf("seed test: %v", err)
	}
	if err := repo.SetTestSteps("p1", "QA-1", []testrepo.Step{
		{XrayID: "s1", Index: 1, Action: "old action", Data: "", Expected: "old expected"},
	}); err != nil {
		t.Fatalf("seed steps: %v", err)
	}
	return repo
}

func TestSetTestStepsThenListReturnsThemInIndexOrder(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "X"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	in := []testrepo.Step{
		{XrayID: "s3", Index: 3, Action: "third", Expected: "ok"},
		{XrayID: "s1", Index: 1, Action: "first", Expected: "ok"},
		{XrayID: "s2", Index: 2, Action: "second", Expected: "ok"},
	}
	if err := repo.SetTestSteps("p1", "QA-1", in); err != nil {
		t.Fatalf("set: %v", err)
	}

	got, err := repo.ListTestSteps("p1", "QA-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	for i, want := range []int{1, 2, 3} {
		if got[i].Index != want {
			t.Errorf("got[%d].Index = %d, want %d (steps not ordered)", i, got[i].Index, want)
		}
	}
}

func TestSetTestStepsReplacesPreviousList(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "X"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := repo.SetTestSteps("p1", "QA-1", []testrepo.Step{
		{XrayID: "old1", Index: 1, Action: "old A"},
		{XrayID: "old2", Index: 2, Action: "old B"},
	}); err != nil {
		t.Fatalf("set old: %v", err)
	}
	if err := repo.SetTestSteps("p1", "QA-1", []testrepo.Step{
		{XrayID: "new1", Index: 1, Action: "new A"},
	}); err != nil {
		t.Fatalf("set new: %v", err)
	}

	got, _ := repo.ListTestSteps("p1", "QA-1")
	if len(got) != 1 || got[0].XrayID != "new1" {
		t.Errorf("after replace got %+v, want [{XrayID:new1...}]", got)
	}
}

func TestListMatchingKeysReturnsAllKeysAcrossPages(t *testing.T) {
	repo := newRepo(t)
	cases := []testrepo.TestCase{}
	for i := 1; i <= 150; i++ {
		cases = append(cases, testrepo.TestCase{
			Key:    fmt.Sprintf("QA-%d", i),
			ID:     fmt.Sprintf("%d", i),
			Status: "Open",
		})
	}
	if err := repo.UpsertTests("p1", cases); err != nil {
		t.Fatalf("seed: %v", err)
	}

	keys, err := repo.ListMatchingKeys("p1", testrepo.Query{})
	if err != nil {
		t.Fatalf("list matching: %v", err)
	}
	if len(keys) != 150 {
		t.Errorf("len(keys) = %d, want 150", len(keys))
	}
}

func TestListMatchingKeysAppliesStatusFilter(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Status: "Open"},
		{Key: "QA-2", ID: "2", Status: "Done"},
		{Key: "QA-3", ID: "3", Status: "Open"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	keys, err := repo.ListMatchingKeys("p1", testrepo.Query{Status: "Open"})
	if err != nil {
		t.Fatalf("list matching: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("len(keys) = %d, want 2", len(keys))
	}
}

func TestListMatchingKeysFolderFilterIncludesDescendants(t *testing.T) {
	repo := seedFolders(t)

	keys, err := repo.ListMatchingKeys("p1", testrepo.Query{FolderID: "/Authentication"})
	if err != nil {
		t.Fatalf("list matching: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 matches under /Authentication (Login + Logout); got %d (%v)", len(keys), keys)
	}
}

func TestTransitionTestRecordsStatusPendingChange(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "X", Status: "Open"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := repo.TransitionTest("p1", "QA-1", "In Progress"); err != nil {
		t.Fatalf("transition: %v", err)
	}

	got, err := repo.GetTest("p1", "QA-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != "In Progress" {
		t.Errorf("Status = %q, want In Progress", got.Status)
	}

	changes, err := repo.ListPendingChanges("p1")
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("want 1 pending change, got %d", len(changes))
	}
	c := changes[0]
	if c.Field != "status" || c.BeforeVal != "Open" || c.AfterVal != "In Progress" {
		t.Errorf("change = %+v, want field=status before=Open after=In Progress", c)
	}
}

func TestTransitionTestToCurrentStatusIsNoop(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "X", Status: "Open"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := repo.TransitionTest("p1", "QA-1", "Open"); err != nil {
		t.Fatalf("transition: %v", err)
	}

	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 0 {
		t.Errorf("transition to current status should not create a pending row; got %+v", changes)
	}
}

func TestUpsertTestsPreservesPendingStatusOnResync(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "X", Status: "Open"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := repo.TransitionTest("p1", "QA-1", "In Progress"); err != nil {
		t.Fatalf("transition: %v", err)
	}

	// Re-sync with a different remote status — the local pending transition
	// must not be clobbered.
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "X", Status: "Approved"},
	}); err != nil {
		t.Fatalf("resync: %v", err)
	}

	got, _ := repo.GetTest("p1", "QA-1")
	if got.Status != "In Progress" {
		t.Errorf("status = %q after resync, want In Progress (pending preserved)", got.Status)
	}
}

func TestDiscardStatusPendingChangeRevertsStatus(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "X", Status: "Open"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := repo.TransitionTest("p1", "QA-1", "Done"); err != nil {
		t.Fatalf("transition: %v", err)
	}
	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 1 {
		t.Fatalf("want 1 pending, got %d", len(changes))
	}

	if err := repo.DiscardPendingChange("p1", changes[0].ID); err != nil {
		t.Fatalf("discard: %v", err)
	}

	got, _ := repo.GetTest("p1", "QA-1")
	if got.Status != "Open" {
		t.Errorf("status = %q after discard, want Open (reverted)", got.Status)
	}
}

func TestGetStatisticsCountsByStatusAndPriority(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Status: "Open", Priority: "High", Labels: []string{"smoke"}},
		{Key: "QA-2", ID: "2", Status: "Open", Priority: "Low", Labels: []string{"smoke", "api"}},
		{Key: "QA-3", ID: "3", Status: "Done", Priority: "High"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	stats, err := repo.GetStatistics("p1")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}

	if stats.Total != 3 {
		t.Errorf("Total = %d, want 3", stats.Total)
	}
	if stats.ByStatus[0].Label != "Open" || stats.ByStatus[0].Count != 2 {
		t.Errorf("top status = %+v, want {Open 2}", stats.ByStatus[0])
	}
}

func TestGetStatisticsTalliesLabelsAcrossTests(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Labels: []string{"smoke"}},
		{Key: "QA-2", ID: "2", Labels: []string{"smoke", "api"}},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	stats, err := repo.GetStatistics("p1")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}

	if stats.ByLabel[0].Label != "smoke" || stats.ByLabel[0].Count != 2 {
		t.Errorf("top label = %+v, want {smoke 2}", stats.ByLabel[0])
	}
}

func TestGetStatisticsIncludesPendingCount(t *testing.T) {
	repo := seedTestForEditing(t)
	if err := repo.EditTestField("p1", "QA-1", "summary", "edited"); err != nil {
		t.Fatalf("edit: %v", err)
	}

	stats, err := repo.GetStatistics("p1")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.PendingChanges != 1 {
		t.Errorf("PendingChanges = %d, want 1", stats.PendingChanges)
	}
}

func TestGetStatisticsRollsUpExecutionCoverage(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1"},
		{Key: "QA-2", ID: "2"},
	}); err != nil {
		t.Fatalf("seed tests: %v", err)
	}
	if err := repo.UpsertContainers("p1", []testrepo.Container{
		{Key: "QA-TE-1", Kind: "testexec", Summary: "Cycle 1", Status: "Open"},
	}); err != nil {
		t.Fatalf("seed containers: %v", err)
	}
	if err := repo.ReplaceAllContainerLinks("p1", []testrepo.ContainerLink{
		{ContainerKey: "QA-TE-1", TestKey: "QA-1", RunStatus: "PASS"},
		{ContainerKey: "QA-TE-1", TestKey: "QA-2", RunStatus: "FAIL"},
	}); err != nil {
		t.Fatalf("seed links: %v", err)
	}

	stats, err := repo.GetStatistics("p1")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.ExecutedTests != 2 {
		t.Errorf("ExecutedTests = %d, want 2", stats.ExecutedTests)
	}
	if len(stats.ByRunStatus) != 2 {
		t.Fatalf("ByRunStatus buckets = %d, want 2 (PASS, FAIL)", len(stats.ByRunStatus))
	}
}

func TestGetStatisticsIgnoresNonExecutionMembershipsForCoverage(t *testing.T) {
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1"},
	}); err != nil {
		t.Fatalf("seed tests: %v", err)
	}
	if err := repo.UpsertContainers("p1", []testrepo.Container{
		{Key: "QA-TS-1", Kind: "testset", Summary: "Set", Status: "Open"},
	}); err != nil {
		t.Fatalf("seed containers: %v", err)
	}
	if err := repo.ReplaceAllContainerLinks("p1", []testrepo.ContainerLink{
		{ContainerKey: "QA-TS-1", TestKey: "QA-1"},
	}); err != nil {
		t.Fatalf("seed links: %v", err)
	}

	stats, _ := repo.GetStatistics("p1")
	if stats.ExecutedTests != 0 {
		t.Errorf("ExecutedTests = %d, want 0 (a Test Set is not an execution)", stats.ExecutedTests)
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
