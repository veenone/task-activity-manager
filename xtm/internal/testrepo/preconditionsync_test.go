package testrepo_test

import (
	"reflect"
	"testing"

	"agile-suite/xtm/internal/testrepo"
)

// jiraPre and jiraPreLinks are what Jira holds in these tests.
var jiraPre = []testrepo.Precondition{
	{Key: "QA-P1", Summary: "Logged in", Type: "Manual"},
	{Key: "QA-P2", Summary: "Clean database", Type: "Manual"},
	{Key: "QA-P3", Summary: "Feature flag on", Type: "Manual"},
}

var jiraPreLinks = map[string][]string{
	"QA-1": {"QA-P1", "QA-P2"},
	"QA-2": {"QA-P1"},
}

// syncPre replays one precondition sync pass the way the engine runs it:
// upsert, mark at a new generation, sweep.
func syncPre(t *testing.T, repo *testrepo.Repository, gen int64, links map[string][]string) {
	t.Helper()
	if err := repo.UpsertPreconditions("p1", jiraPre); err != nil {
		t.Fatalf("upsert preconditions: %v", err)
	}
	if err := repo.MarkTestPreconditions("p1", gen, links); err != nil {
		t.Fatalf("mark links: %v", err)
	}
	if _, err := repo.SweepTestPreconditions("p1", gen); err != nil {
		t.Fatalf("sweep links: %v", err)
	}
}

func seedPreRepo(t *testing.T) *testrepo.Repository {
	t.Helper()
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "Login"},
		{Key: "QA-2", ID: "2", Summary: "Logout"},
	}); err != nil {
		t.Fatalf("seed tests: %v", err)
	}
	syncPre(t, repo, 1, jiraPreLinks)
	return repo
}

func testPreKeys(t *testing.T, repo *testrepo.Repository, testKey string) []string {
	t.Helper()
	pre, err := repo.ListTestPreconditions("p1", testKey)
	if err != nil {
		t.Fatalf("list preconditions of %s: %v", testKey, err)
	}
	keys := []string{}
	for _, p := range pre {
		keys = append(keys, p.Key)
	}
	return keys
}

func preByKey(t *testing.T, repo *testrepo.Repository) map[string]testrepo.PreconditionUsage {
	t.Helper()
	all, err := repo.ListPreconditionsWithUsage("p1")
	if err != nil {
		t.Fatalf("list preconditions: %v", err)
	}
	out := map[string]testrepo.PreconditionUsage{}
	for _, p := range all {
		out[p.Key] = p
	}
	return out
}

func TestSyncKeepsPendingPreconditionEdit(t *testing.T) {
	repo := seedPreRepo(t)
	if err := repo.EditPreconditionField("p1", "QA-P2", "summary", "Empty database"); err != nil {
		t.Fatalf("edit: %v", err)
	}

	syncPre(t, repo, 2, jiraPreLinks)

	all := preByKey(t, repo)
	if got := all["QA-P2"].Summary; got != "Empty database" {
		t.Errorf("QA-P2 summary = %q, want the pending edit", got)
	}
	if got := all["QA-P1"].Summary; got != "Logged in" {
		t.Errorf("QA-P1 summary = %q, want the synced value", got)
	}
}

func TestSyncKeepsPendingPreconditionDelete(t *testing.T) {
	repo := seedPreRepo(t)
	if err := repo.DeletePrecondition("p1", "QA-P1"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	syncPre(t, repo, 2, jiraPreLinks)

	if _, ok := preByKey(t, repo)["QA-P1"]; ok {
		t.Error("QA-P1 came back although its delete is still pending")
	}
	tests, err := repo.ListTestsForPrecondition("p1", "QA-P1")
	if err != nil {
		t.Fatalf("list tests: %v", err)
	}
	if len(tests) != 0 {
		t.Errorf("QA-P1 still has %d test links", len(tests))
	}
	if got := testPreKeys(t, repo, "QA-1"); !reflect.DeepEqual(got, []string{"QA-P2"}) {
		t.Errorf("QA-1 preconditions = %v, want [QA-P2]", got)
	}
}

func TestSyncKeepsPendingPreconditionAssociation(t *testing.T) {
	repo := seedPreRepo(t)
	// QA-1 drops QA-P2 and gains QA-P3.
	if err := repo.SetTestPreconditions("p1", "QA-1", []string{"QA-P1", "QA-P3"}); err != nil {
		t.Fatalf("set: %v", err)
	}

	// Jira also dropped QA-2's link, which the sweep must still remove.
	syncPre(t, repo, 2, map[string][]string{"QA-1": {"QA-P1", "QA-P2"}})

	if got := testPreKeys(t, repo, "QA-1"); !reflect.DeepEqual(got, []string{"QA-P1", "QA-P3"}) {
		t.Errorf("QA-1 preconditions = %v, want [QA-P1 QA-P3]", got)
	}
	if got := testPreKeys(t, repo, "QA-2"); len(got) != 0 {
		t.Errorf("QA-2 preconditions = %v, want none after the sweep", got)
	}
}
