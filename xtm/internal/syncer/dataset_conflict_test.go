package syncer_test

import (
	"context"
	"strings"
	"testing"

	"agile-suite/xtm/internal/backend/xray"
	"agile-suite/xtm/internal/jira"
	"agile-suite/xtm/internal/syncer"
	"agile-suite/xtm/internal/testrepo"
)

// Resolving a dataset conflict, the half of #277 W1 that lives in the
// repository: taking theirs, and keeping mine without the same conflict
// coming back on the next commit.

// Keeping mine after a conflict has to re-base what the next commit compares
// against, or the same conflict is reported forever.
func TestResolvingADatasetConflictWithMineLetsTheNextCommitThrough(t *testing.T) {
	repo := datasetRepo(t)
	mine := testrepo.Dataset{
		Parameters: []testrepo.DatasetParameter{{Name: "USERNAME"}},
		Rows:       [][]string{{"alice"}, {"bob"}},
	}
	if err := repo.EditTestDataset("p1", "QA-1", mine); err != nil {
		t.Fatalf("edit dataset: %v", err)
	}
	inst := &datasetInstance{columns: []string{
		textColumn(2726, "USERNAME", "alice"),
		textColumn(2727, "REGION", "eu-west"),
	}}
	srv := inst.server(t)
	defer srv.Close()
	eng := syncer.New(xray.New(jira.NewClient(srv.URL, "tok")), repo)

	res, err := eng.CommitChanges(context.Background(), "p1", "QA")
	if err != nil {
		t.Fatalf("CommitChanges: %v", err)
	}
	if len(res.Conflicted) != 1 {
		t.Fatalf("conflicted = %+v, want one conflict", res.Conflicted)
	}
	f := res.Conflicted[0].Fields[0]
	if err := repo.ResolveConflictMerge("p1", "QA-1", res.Conflicted[0].RemoteVersion,
		[]testrepo.ConflictDecision{{
			PendingID:   f.PendingID,
			EntityType:  f.EntityType,
			EntityKey:   f.EntityKey,
			Field:       f.Field,
			Choice:      "mine",
			RemoteValue: f.Remote,
		}}); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	res, err = eng.CommitChanges(context.Background(), "p1", "QA")
	if err != nil {
		t.Fatalf("second CommitChanges: %v", err)
	}
	if len(res.Conflicted) != 0 {
		t.Fatalf("still conflicted after keeping mine: %+v", res.Conflicted)
	}
	if inst.writes != 1 {
		t.Fatalf("%d dataset writes after keeping mine, want 1", inst.writes)
	}
	if !strings.Contains(inst.wrote, `"value":"bob"`) {
		t.Errorf("write body %s does not carry my table", inst.wrote)
	}
}

// Taking theirs puts the remote table in the cache and drops the edit, so the
// panel stops showing a change that is not going anywhere.
func TestResolvingADatasetConflictWithTheirsTakesTheRemoteTable(t *testing.T) {
	repo := datasetRepo(t)
	if err := repo.EditTestDataset("p1", "QA-1", testrepo.Dataset{
		Parameters: []testrepo.DatasetParameter{{Name: "USERNAME"}},
		Rows:       [][]string{{"alice"}, {"bob"}},
	}); err != nil {
		t.Fatalf("edit dataset: %v", err)
	}
	inst := &datasetInstance{columns: []string{
		textColumn(2726, "USERNAME", "alice"),
		textColumn(2727, "REGION", "eu-west"),
	}}
	srv := inst.server(t)
	defer srv.Close()
	eng := syncer.New(xray.New(jira.NewClient(srv.URL, "tok")), repo)

	res, err := eng.CommitChanges(context.Background(), "p1", "QA")
	if err != nil {
		t.Fatalf("CommitChanges: %v", err)
	}
	if len(res.Conflicted) != 1 {
		t.Fatalf("conflicted = %+v, want one conflict", res.Conflicted)
	}
	f := res.Conflicted[0].Fields[0]
	if err := repo.ResolveConflictMerge("p1", "QA-1", "", []testrepo.ConflictDecision{{
		PendingID:   f.PendingID,
		EntityType:  f.EntityType,
		EntityKey:   f.EntityKey,
		Field:       f.Field,
		Choice:      "theirs",
		RemoteValue: f.Remote,
	}}); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if left := pendingDatasets(t, repo); len(left) != 0 {
		t.Errorf("%d pending dataset rows after taking theirs, want none", len(left))
	}
	cached, found, err := repo.GetTestDataset("p1", "QA-1", "")
	if err != nil || !found {
		t.Fatalf("cache: found=%v err=%v", found, err)
	}
	if len(cached.Parameters) != 2 || cached.Parameters[1].Name != "REGION" {
		t.Errorf("cached dataset = %+v, want the remote table", cached)
	}
}
