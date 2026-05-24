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
