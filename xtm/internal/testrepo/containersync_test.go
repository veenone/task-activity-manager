package testrepo_test

import (
	"reflect"
	"sort"
	"testing"

	"agile-suite/xtm/internal/testrepo"
)

// jiraContainers and jiraContainerLinks are what Jira holds in these tests:
// the execution seedReqRepo caches, plus a Test Set.
var jiraContainers = []testrepo.Container{
	{Key: "QA-TE-1", Kind: "testexec", Summary: "Cycle 1"},
	{Key: "QA-TS-1", Kind: "testset", Summary: "Smoke"},
}

var jiraContainerLinks = []testrepo.ContainerLink{
	{ContainerKey: "QA-TE-1", TestKey: "QA-1", RunStatus: "PASS"},
	{ContainerKey: "QA-TE-1", TestKey: "QA-2", RunStatus: "FAIL"},
	{ContainerKey: "QA-TE-1", TestKey: "QA-3", RunStatus: "EXECUTING"},
	{ContainerKey: "QA-TS-1", TestKey: "QA-1"},
	{ContainerKey: "QA-TS-1", TestKey: "QA-2"},
}

func syncContainers(t *testing.T, repo *testrepo.Repository) {
	t.Helper()
	if err := repo.UpsertContainers("p1", jiraContainers); err != nil {
		t.Fatalf("upsert containers: %v", err)
	}
	if err := repo.ReplaceAllContainerLinks("p1", jiraContainerLinks); err != nil {
		t.Fatalf("replace container links: %v", err)
	}
}

func seedContainerRepo(t *testing.T) *testrepo.Repository {
	t.Helper()
	repo := seedReqRepo(t)
	syncContainers(t, repo)
	return repo
}

// containersOf returns the keys of the containers a Test is a member of.
func containersOf(t *testing.T, repo *testrepo.Repository, testKey string) []string {
	t.Helper()
	ms, err := repo.ListContainersForTest("p1", testKey)
	if err != nil {
		t.Fatalf("containers of %s: %v", testKey, err)
	}
	keys := []string{}
	for _, m := range ms {
		keys = append(keys, m.Key)
	}
	sort.Strings(keys)
	return keys
}

func containerSummaries(t *testing.T, repo *testrepo.Repository, kind string) map[string]string {
	t.Helper()
	cs, err := repo.ListContainers("p1", kind)
	if err != nil {
		t.Fatalf("list %s: %v", kind, err)
	}
	out := map[string]string{}
	for _, c := range cs {
		out[c.Key] = c.Summary
	}
	return out
}

func TestSyncKeepsLocalContainerMembers(t *testing.T) {
	repo := seedContainerRepo(t)
	res, err := repo.CreateContainerAllocation("p1", "QA", "testset", "Regression", []string{"QA-3"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	syncContainers(t, repo)

	if got, want := containersOf(t, repo, "QA-3"), []string{"QA-TE-1", res.TempKey}; !reflect.DeepEqual(got, want) {
		t.Errorf("QA-3 is in %v, want %v", got, want)
	}
}

func TestSyncKeepsPendingMembershipEdits(t *testing.T) {
	repo := seedContainerRepo(t)
	if _, err := repo.AllocateTests("p1", "QA-TS-1", []string{"QA-3"}); err != nil {
		t.Fatalf("allocate: %v", err)
	}
	if _, err := repo.DeallocateTests("p1", "QA-TS-1", []string{"QA-1"}); err != nil {
		t.Fatalf("deallocate: %v", err)
	}

	syncContainers(t, repo)

	if got, want := containersOf(t, repo, "QA-3"), []string{"QA-TE-1", "QA-TS-1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("QA-3 is in %v, want %v", got, want)
	}
	if got, want := containersOf(t, repo, "QA-1"), []string{"QA-TE-1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("QA-1 is in %v, want %v", got, want)
	}
	if got, want := containersOf(t, repo, "QA-2"), []string{"QA-TE-1", "QA-TS-1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("QA-2 is in %v, want the synced %v", got, want)
	}
}

func TestSyncKeepsPendingContainerDelete(t *testing.T) {
	repo := seedContainerRepo(t)
	if err := repo.DeleteContainer("p1", "QA-TS-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	syncContainers(t, repo)

	if _, ok := containerSummaries(t, repo, "testset")["QA-TS-1"]; ok {
		t.Error("QA-TS-1 came back although its delete is still pending")
	}
	if got, want := containersOf(t, repo, "QA-1"), []string{"QA-TE-1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("QA-1 is in %v, want %v", got, want)
	}
}

func TestSyncKeepsPendingContainerRename(t *testing.T) {
	repo := seedContainerRepo(t)
	if err := repo.EditContainer("p1", "QA-TS-1", "Smoke suite"); err != nil {
		t.Fatalf("rename: %v", err)
	}

	syncContainers(t, repo)

	if got := containerSummaries(t, repo, "testset")["QA-TS-1"]; got != "Smoke suite" {
		t.Errorf("QA-TS-1 summary = %q, want the pending rename", got)
	}
	if got := containerSummaries(t, repo, "testexec")["QA-TE-1"]; got != "Cycle 1" {
		t.Errorf("QA-TE-1 summary = %q, want the synced value", got)
	}
}

func TestSyncKeepsStagedRunResults(t *testing.T) {
	repo := seedContainerRepo(t)
	if err := repo.SetTestRunStatus("p1", "QA-TE-1", "QA-3", "FAIL"); err != nil {
		t.Fatalf("status: %v", err)
	}
	if err := repo.AddTestRunDefect("p1", "QA-TE-1", "QA-3", "BUG-7"); err != nil {
		t.Fatalf("defect: %v", err)
	}
	if err := repo.SetTestRunComment("p1", "QA-TE-1", "QA-3", "Times out"); err != nil {
		t.Fatalf("comment: %v", err)
	}

	syncContainers(t, repo)

	members, err := repo.GetExecutionMembersWithRuns("p1", "QA-TE-1")
	if err != nil {
		t.Fatalf("members: %v", err)
	}
	got := map[string]testrepo.ExecMemberRun{}
	for _, m := range members {
		got[m.TestKey] = m
	}
	staged := got["QA-3"]
	if staged.RunStatus != "FAIL" || !reflect.DeepEqual(staged.Defects, []string{"BUG-7"}) || staged.Comment != "Times out" {
		t.Errorf("QA-3 run = %s %v %q, want FAIL [BUG-7] \"Times out\"",
			staged.RunStatus, staged.Defects, staged.Comment)
	}
	if got["QA-1"].RunStatus != "PASS" {
		t.Errorf("QA-1 run = %s, want the synced PASS", got["QA-1"].RunStatus)
	}
}
