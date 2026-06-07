package testrepo_test

import (
	"testing"

	"xray-test-manager/internal/testrepo"
)

func seedContainerCrud(t *testing.T) *testrepo.Repository {
	t.Helper()
	repo := newRepo(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1"},
	}); err != nil {
		t.Fatalf("seed tests: %v", err)
	}
	if err := repo.UpsertContainers("p1", []testrepo.Container{
		{Key: "QA-TP-1", Kind: "testplan", Summary: "Release 1.0", Status: "Open"},
	}); err != nil {
		t.Fatalf("seed container: %v", err)
	}
	return repo
}

func TestEditContainerQueuesRename(t *testing.T) {
	repo := seedContainerCrud(t)

	if err := repo.EditContainer("p1", "QA-TP-1", "Release 2.0"); err != nil {
		t.Fatalf("edit: %v", err)
	}

	plans, _ := repo.ListContainers("p1", "testplan")
	if plans[0].Summary != "Release 2.0" {
		t.Errorf("summary = %q, want Release 2.0", plans[0].Summary)
	}
	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 1 || changes[0].EntityType != "container_edit" {
		t.Fatalf("pending = %+v, want one container_edit", changes)
	}
}

func TestDiscardContainerEditRevertsSummary(t *testing.T) {
	repo := seedContainerCrud(t)
	if err := repo.EditContainer("p1", "QA-TP-1", "Renamed"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	changes, _ := repo.ListPendingChanges("p1")
	if err := repo.DiscardPendingChange("p1", changes[0].ID); err != nil {
		t.Fatalf("discard: %v", err)
	}
	plans, _ := repo.ListContainers("p1", "testplan")
	if plans[0].Summary != "Release 1.0" {
		t.Errorf("summary = %q after discard, want Release 1.0", plans[0].Summary)
	}
}

func TestDeleteContainerQueuesDeleteAndDiscardRestores(t *testing.T) {
	repo := seedContainerCrud(t)
	if err := repo.ReplaceAllContainerLinks("p1", []testrepo.ContainerLink{
		{ContainerKey: "QA-TP-1", TestKey: "QA-1"},
	}); err != nil {
		t.Fatalf("seed link: %v", err)
	}

	if err := repo.DeleteContainer("p1", "QA-TP-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	plans, _ := repo.ListContainers("p1", "testplan")
	if len(plans) != 0 {
		t.Errorf("container should be gone; got %+v", plans)
	}
	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 1 || changes[0].EntityType != "container_delete" {
		t.Fatalf("pending = %+v, want one container_delete", changes)
	}

	if err := repo.DiscardPendingChange("p1", changes[0].ID); err != nil {
		t.Fatalf("discard: %v", err)
	}
	plans, _ = repo.ListContainers("p1", "testplan")
	if len(plans) != 1 {
		t.Fatalf("container should be restored; got %+v", plans)
	}
	members, _ := repo.ListContainersForTest("p1", "QA-1")
	if len(members) != 1 {
		t.Errorf("membership should be restored; got %+v", members)
	}
}

func TestDeleteLocallyCreatedContainerCancelsCreate(t *testing.T) {
	repo := seedContainerCrud(t)
	res, err := repo.CreateContainerAllocation("p1", "QA", "testplan", "New plan", []string{"QA-1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := repo.DeleteContainer("p1", res.TempKey); err != nil {
		t.Fatalf("delete: %v", err)
	}

	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 0 {
		t.Errorf("create-then-delete should leave no pending rows; got %+v", changes)
	}
}
