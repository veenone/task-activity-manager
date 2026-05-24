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
