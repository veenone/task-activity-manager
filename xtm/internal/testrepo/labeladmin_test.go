package testrepo_test

import (
	"reflect"
	"testing"
)

func TestListTestsWithLabelMatchesWholeCaseSensitiveLabels(t *testing.T) {
	repo := newRepo(t)
	seedLabelTests(t, repo)

	got, err := repo.ListTestsWithLabel(lblProfile, "smoke")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"QA-1", "QA-2"}) {
		t.Fatalf("got %v", got)
	}
	if got, _ := repo.ListTestsWithLabel(lblProfile, "smo"); len(got) != 0 {
		t.Fatalf("partial label matched %v", got)
	}
}

func TestRenameLabelQueuesOneChangePerCarryingTest(t *testing.T) {
	repo := newRepo(t)
	seedLabelTests(t, repo)

	res, err := repo.RenameLabel(lblProfile, "smoke", "sanity")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if !reflect.DeepEqual(res.Succeeded, []string{"QA-1", "QA-2"}) {
		t.Fatalf("succeeded %v", res.Succeeded)
	}
	if got := labelsOf(t, repo, "QA-1"); !reflect.DeepEqual(got, []string{"login", "sanity"}) {
		t.Fatalf("QA-1 %v", got)
	}
	if got := labelsOf(t, repo, "QA-3"); !reflect.DeepEqual(got, []string{"Smoke"}) {
		t.Fatalf("QA-3 touched: %v", got)
	}
	pending, _ := repo.ListPendingChanges(lblProfile)
	if len(pending) != 2 {
		t.Fatalf("want 2 pending changes, got %d", len(pending))
	}
}

func TestMergeLabelDoesNotDuplicate(t *testing.T) {
	repo := newRepo(t)
	seedLabelTests(t, repo)

	if _, err := repo.RenameLabel(lblProfile, "login", "smoke"); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if got := labelsOf(t, repo, "QA-1"); !reflect.DeepEqual(got, []string{"smoke"}) {
		t.Fatalf("QA-1 %v", got)
	}
}

func TestDeleteLabelRemovesItFromCarryingTests(t *testing.T) {
	repo := newRepo(t)
	seedLabelTests(t, repo)

	res, err := repo.DeleteLabel(lblProfile, "smoke")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(res.Succeeded) != 2 {
		t.Fatalf("succeeded %v", res.Succeeded)
	}
	if got := labelsOf(t, repo, "QA-2"); len(got) != 0 {
		t.Fatalf("QA-2 %v", got)
	}
}

func TestRenameAndDeleteRefusedWhilePendingEditsNameTheLabel(t *testing.T) {
	repo := newRepo(t)
	seedLabelTests(t, repo)
	if err := repo.EditTestField(lblProfile, "QA-2", "labels", "smoke extra"); err != nil {
		t.Fatalf("edit: %v", err)
	}

	n, err := repo.LabelEditsPending(lblProfile, "smoke")
	if err != nil || n != 1 {
		t.Fatalf("pending %d, %v", n, err)
	}
	if n, _ := repo.LabelEditsPending(lblProfile, "login"); n != 0 {
		t.Fatalf("login pending %d", n)
	}

	want := `Commit or discard the 1 pending label edit on tests that use "smoke" first.`
	if _, err := repo.RenameLabel(lblProfile, "smoke", "sanity"); err == nil || err.Error() != want {
		t.Fatalf("rename err %v", err)
	}
	if _, err := repo.DeleteLabel(lblProfile, "smoke"); err == nil || err.Error() != want {
		t.Fatalf("delete err %v", err)
	}
	if got := labelsOf(t, repo, "QA-1"); !reflect.DeepEqual(got, []string{"smoke", "login"}) {
		t.Fatalf("QA-1 changed despite the guard: %v", got)
	}
}

func TestRenameLabelRefusesAnInvalidName(t *testing.T) {
	repo := newRepo(t)
	seedLabelTests(t, repo)

	_, err := repo.RenameLabel(lblProfile, "smoke", "two words")
	if err == nil || err.Error() != `label "two words" contains whitespace` {
		t.Fatalf("err %v", err)
	}
	if _, err := repo.RenameLabel(lblProfile, "smoke", "smoke"); err == nil {
		t.Fatal("renaming a label to itself should be refused")
	}
}
