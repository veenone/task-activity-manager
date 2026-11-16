package testrepo_test

import (
	"reflect"
	"testing"

	"agile-suite/xtm/internal/testrepo"
)

// jiraReqs and jiraLinks are what Jira holds in these tests: the same data
// seedReqRepo caches, so replaying them stands in for a sync that found
// nothing new.
var jiraReqs = []testrepo.Requirement{
	{Key: "PRD-10", ProjectKey: "PRD", IssueType: "Story", Summary: "Auth works"},
	{Key: "PRD-11", ProjectKey: "PRD", IssueType: "Story", Summary: "Session policy"},
	{Key: "PRD-12", ProjectKey: "PRD", IssueType: "Epic", Summary: "Untested"},
}

var jiraLinks = []testrepo.RequirementLink{
	{TestKey: "QA-1", RequirementKey: "PRD-10", LinkID: "100"},
	{TestKey: "QA-2", RequirementKey: "PRD-10", LinkID: "101"},
	{TestKey: "QA-3", RequirementKey: "PRD-11", LinkID: "102"},
}

func resync(t *testing.T, repo *testrepo.Repository) {
	t.Helper()
	if err := repo.ReplaceAllRequirements("p1", jiraReqs); err != nil {
		t.Fatalf("replace requirements: %v", err)
	}
	if err := repo.ReplaceAllRequirementLinks("p1", jiraLinks); err != nil {
		t.Fatalf("replace links: %v", err)
	}
}

func coverageByKey(t *testing.T, repo *testrepo.Repository) map[string]testrepo.RequirementCoverage {
	t.Helper()
	cov, err := repo.ListRequirementsWithCoverage("p1")
	if err != nil {
		t.Fatalf("coverage: %v", err)
	}
	out := map[string]testrepo.RequirementCoverage{}
	for _, c := range cov {
		out[c.Key] = c
	}
	return out
}

func testReqKeys(t *testing.T, repo *testrepo.Repository, testKey string) []string {
	t.Helper()
	reqs, err := repo.GetTestRequirements("p1", testKey)
	if err != nil {
		t.Fatalf("get test requirements: %v", err)
	}
	keys := []string{}
	for _, rq := range reqs {
		keys = append(keys, rq.Key)
	}
	return keys
}

func TestSyncKeepsLocallyCreatedRequirement(t *testing.T) {
	repo := seedReqRepo(t)
	key, err := repo.CreateRequirement("p1", "PRD", "Story", "Draft", "", "", "", "", nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.SetTestRequirements("p1", "QA-3", []string{"PRD-11", key}); err != nil {
		t.Fatalf("link: %v", err)
	}

	resync(t, repo)

	got, ok := coverageByKey(t, repo)[key]
	if !ok {
		t.Fatalf("%s is gone from the cache after a sync", key)
	}
	if got.Summary != "Draft" || got.TestCount != 1 {
		t.Errorf("%s = summary %q, %d tests; want Draft, 1", key, got.Summary, got.TestCount)
	}
}

func TestSyncKeepsPendingRequirementDelete(t *testing.T) {
	repo := seedReqRepo(t)
	if err := repo.DeleteRequirement("p1", "PRD-10"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	resync(t, repo)

	if _, ok := coverageByKey(t, repo)["PRD-10"]; ok {
		t.Error("PRD-10 came back although its delete is still pending")
	}
	if keys := testReqKeys(t, repo, "QA-1"); len(keys) != 0 {
		t.Errorf("QA-1 still covers %v", keys)
	}
	tests, err := repo.ListTestsForRequirement("p1", "PRD-10")
	if err != nil {
		t.Fatalf("list tests: %v", err)
	}
	if len(tests) != 0 {
		t.Errorf("PRD-10 still has %d coverage links", len(tests))
	}
}

func TestSyncKeepsPendingRequirementEdit(t *testing.T) {
	repo := seedReqRepo(t)
	if err := repo.EditRequirementField("p1", "PRD-11", "summary", "Session rules"); err != nil {
		t.Fatalf("edit: %v", err)
	}

	resync(t, repo)

	cov := coverageByKey(t, repo)
	if got := cov["PRD-11"].Summary; got != "Session rules" {
		t.Errorf("PRD-11 summary = %q, want the pending edit", got)
	}
	if got := cov["PRD-12"].Summary; got != "Untested" {
		t.Errorf("PRD-12 summary = %q, want the synced value", got)
	}
}

func TestSyncKeepsPendingCoverageEdit(t *testing.T) {
	repo, st := newRepoAndStore(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "Login"},
		{Key: "QA-2", ID: "2", Summary: "Logout"},
		{Key: "QA-3", ID: "3", Summary: "Reset"},
	}); err != nil {
		t.Fatalf("seed tests: %v", err)
	}
	resync(t, repo)
	// QA-1 keeps PRD-10 and gains PRD-12; QA-3 drops its only link.
	if err := repo.SetTestRequirements("p1", "QA-1", []string{"PRD-10", "PRD-12"}); err != nil {
		t.Fatalf("set QA-1: %v", err)
	}
	if err := repo.SetTestRequirements("p1", "QA-3", nil); err != nil {
		t.Fatalf("set QA-3: %v", err)
	}

	resync(t, repo)

	if got := testReqKeys(t, repo, "QA-1"); !reflect.DeepEqual(got, []string{"PRD-10", "PRD-12"}) {
		t.Errorf("QA-1 covers %v, want [PRD-10 PRD-12]", got)
	}
	if got := testReqKeys(t, repo, "QA-3"); len(got) != 0 {
		t.Errorf("QA-3 covers %v, want nothing", got)
	}
	if got := testReqKeys(t, repo, "QA-2"); !reflect.DeepEqual(got, []string{"PRD-10"}) {
		t.Errorf("QA-2 covers %v, want the synced [PRD-10]", got)
	}

	linkIDs := map[string]string{}
	rows, err := st.DB().Query(
		`SELECT requirement_key, link_id FROM test_requirement
		 WHERE profile_id = 'p1' AND test_key = 'QA-1'`)
	if err != nil {
		t.Fatalf("read link ids: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var k, id string
		if err := rows.Scan(&k, &id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		linkIDs[k] = id
	}
	if want := map[string]string{"PRD-10": "100", "PRD-12": ""}; !reflect.DeepEqual(linkIDs, want) {
		t.Errorf("QA-1 link ids = %v, want %v", linkIDs, want)
	}
}
