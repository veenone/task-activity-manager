package testrepo_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"agile-suite/xtm/internal/store"
	"agile-suite/xtm/internal/testrepo"
)

const caProfile = "p1"

func seedComponentAdmin(t *testing.T, repo *testrepo.Repository) {
	t.Helper()
	tests := []testrepo.TestCase{
		{Key: "QA-1", Summary: "a", Components: []string{"core", "API"}},
		{Key: "QA-2", Summary: "b", Components: []string{"core"}},
		{Key: "QA-3", Summary: "c", Components: []string{"Core Services"}},
		{Key: "QA-4", Summary: "d"},
	}
	if err := repo.UpsertTests(caProfile, tests); err != nil {
		t.Fatalf("upsert: %v", err)
	}
}

func componentsOf(t *testing.T, repo *testrepo.Repository, key string) []string {
	t.Helper()
	tc, err := repo.GetTest(caProfile, key)
	if err != nil {
		t.Fatalf("get %s: %v", key, err)
	}
	return tc.Components
}

func TestRenameComponentOnTestsCaseOnly(t *testing.T) {
	repo := newRepo(t)
	seedComponentAdmin(t, repo)

	n, err := repo.RenameComponentOnTests(caProfile, "core", "Core")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if n != 2 {
		t.Fatalf("changed %d tests, want 2", n)
	}
	if got := componentsOf(t, repo, "QA-1"); !reflect.DeepEqual(got, []string{"Core", "API"}) {
		t.Fatalf("QA-1 %v", got)
	}
	if got := componentsOf(t, repo, "QA-3"); !reflect.DeepEqual(got, []string{"Core Services"}) {
		t.Fatalf("QA-3 must keep its longer name, got %v", got)
	}
}

func TestRenameComponentOnTestsMergesIntoExisting(t *testing.T) {
	repo := newRepo(t)
	seedComponentAdmin(t, repo)

	if _, err := repo.RenameComponentOnTests(caProfile, "API", "core"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if got := componentsOf(t, repo, "QA-1"); !reflect.DeepEqual(got, []string{"core"}) {
		t.Fatalf("QA-1 should hold core once, got %v", got)
	}
}

func TestRemoveComponentFromTests(t *testing.T) {
	repo := newRepo(t)
	seedComponentAdmin(t, repo)

	n, err := repo.RemoveComponentFromTests(caProfile, "core")
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if n != 2 {
		t.Fatalf("changed %d, want 2", n)
	}
	if got := componentsOf(t, repo, "QA-2"); len(got) != 0 {
		t.Fatalf("QA-2 %v", got)
	}
	if got := componentsOf(t, repo, "QA-1"); !reflect.DeepEqual(got, []string{"API"}) {
		t.Fatalf("QA-1 %v", got)
	}
}

func TestComponentEditsPendingIgnoresOtherFields(t *testing.T) {
	repo := newRepo(t)
	seedComponentAdmin(t, repo)

	// An unrelated pending edit on a test that carries the component.
	if err := repo.EditTestField(caProfile, "QA-1", "summary", "changed"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	n, err := repo.ComponentEditsPending(caProfile, "core")
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if n != 0 {
		t.Fatalf("summary edit counted as a component edit: %d", n)
	}
}

func newRepoAndStore(t *testing.T) (*testrepo.Repository, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return testrepo.NewRepository(st), st
}

func TestComponentEditsPendingCountsMatchingEdits(t *testing.T) {
	repo, st := newRepoAndStore(t)
	seedComponentAdmin(t, repo)
	insert := func(key, after string) {
		t.Helper()
		_, err := st.DB().Exec(`INSERT INTO pending_change
			(profile_id, entity_type, entity_key, field, before_val, after_val, base_version, created_at)
			VALUES (?, 'test_case', ?, 'components', '', ?, '', '2026-10-09T00:00:00Z')`,
			caProfile, key, after)
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	insert("QA-2", "\nAPI\n")   // the test carries core in its synced value
	insert("QA-4", "\ncore\n")  // the queued value adds core
	insert("QA-3", "\nOther\n") // unrelated

	n, err := repo.ComponentEditsPending(caProfile, "core")
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if n != 2 {
		t.Fatalf("want 2, got %d", n)
	}
}

// A queued edit that drops a component still blocks: discarding it would
// restore the old value, naming a component Jira no longer has. The seed
// mirrors what EditTestField leaves behind: the cache holds the queued value
// and before_val holds Jira's.
func TestComponentEditsPendingCountsEditsThatDropTheComponent(t *testing.T) {
	repo, st := newRepoAndStore(t)
	seedComponentAdmin(t, repo)
	db := st.DB()
	if _, err := db.Exec(`UPDATE test_case SET components = ? WHERE profile_id = ? AND jira_key = 'QA-1'`,
		"\nAPI\n", caProfile); err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO pending_change
		(profile_id, entity_type, entity_key, field, before_val, after_val, base_version, created_at)
		VALUES (?, 'test_case', 'QA-1', 'components', ?, ?, '', '2026-10-09T00:00:00Z')`,
		caProfile, "\ncore\nAPI\n", "\nAPI\n"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	n, err := repo.ComponentEditsPending(caProfile, "core")
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1, got %d", n)
	}
}
