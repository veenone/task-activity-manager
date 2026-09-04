package testrepo_test

import (
	"testing"

	"agile-suite/xtm/internal/testrepo"
)

func seedFolderTree(t *testing.T) *testrepo.Repository {
	t.Helper()
	repo := newRepo(t)
	if err := repo.UpsertFolders("p1", []testrepo.Folder{
		{ID: "/Auth", ParentID: "", Name: "Auth"},
		{ID: "/Auth/Login", ParentID: "/Auth", Name: "Login"},
	}); err != nil {
		t.Fatalf("seed folders: %v", err)
	}
	return repo
}

func TestCreateFolderAddsNodeAndQueues(t *testing.T) {
	repo := seedFolderTree(t)

	f, err := repo.CreateFolder("p1", "/Auth", "Logout")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if f.ID != "/Auth/Logout" {
		t.Errorf("ID = %q, want /Auth/Logout", f.ID)
	}

	folders, _ := repo.ListFolders("p1")
	var found bool
	for _, x := range folders {
		if x.ID == "/Auth/Logout" {
			found = true
		}
	}
	if !found {
		t.Error("new folder not in list")
	}

	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 1 || changes[0].EntityType != "folder_create" || changes[0].EntityKey != "/Auth/Logout" {
		t.Fatalf("pending = %+v, want one folder_create for /Auth/Logout", changes)
	}
}

func TestRenameFolderCascadesToDescendantsAndTests(t *testing.T) {
	repo := seedFolderTree(t)
	if err := repo.UpsertFolders("p1", []testrepo.Folder{
		{ID: "/Auth/Login/Mobile", ParentID: "/Auth/Login", Name: "Mobile"},
	}); err != nil {
		t.Fatalf("seed deep: %v", err)
	}
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", FolderID: "/Auth/Login"},
		{Key: "QA-2", ID: "2", FolderID: "/Auth/Login/Mobile"},
	}); err != nil {
		t.Fatalf("seed tests: %v", err)
	}

	if err := repo.RenameFolder("p1", "/Auth/Login", "Signin"); err != nil {
		t.Fatalf("rename: %v", err)
	}

	folders, _ := repo.ListFolders("p1")
	have := map[string]bool{}
	for _, x := range folders {
		have[x.ID] = true
	}
	if !have["/Auth/Signin"] || !have["/Auth/Signin/Mobile"] || have["/Auth/Login"] {
		t.Errorf("folders after rename = %v, want /Auth/Signin + /Auth/Signin/Mobile", folders)
	}

	qa1, _ := repo.GetTest("p1", "QA-1")
	qa2, _ := repo.GetTest("p1", "QA-2")
	if qa1.FolderID != "/Auth/Signin" {
		t.Errorf("QA-1 folder = %q, want /Auth/Signin", qa1.FolderID)
	}
	if qa2.FolderID != "/Auth/Signin/Mobile" {
		t.Errorf("QA-2 folder = %q, want /Auth/Signin/Mobile", qa2.FolderID)
	}
}

func TestDiscardRenameFolderRestoresPaths(t *testing.T) {
	repo := seedFolderTree(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", FolderID: "/Auth/Login"},
	}); err != nil {
		t.Fatalf("seed test: %v", err)
	}
	if err := repo.RenameFolder("p1", "/Auth/Login", "Signin"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	changes, _ := repo.ListPendingChanges("p1")

	if err := repo.DiscardPendingChange("p1", changes[0].ID); err != nil {
		t.Fatalf("discard: %v", err)
	}

	qa1, _ := repo.GetTest("p1", "QA-1")
	if qa1.FolderID != "/Auth/Login" {
		t.Errorf("QA-1 folder = %q after discard, want /Auth/Login", qa1.FolderID)
	}
}

func TestDeleteFolderRejectsNonEmpty(t *testing.T) {
	repo := seedFolderTree(t)
	// /Auth has a subfolder /Auth/Login.
	if err := repo.DeleteFolder("p1", "/Auth"); err == nil {
		t.Error("deleting a folder with subfolders should error")
	}
}

func TestDeleteEmptyFolderQueuesAndDiscardRestores(t *testing.T) {
	repo := seedFolderTree(t)

	if err := repo.DeleteFolder("p1", "/Auth/Login"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	folders, _ := repo.ListFolders("p1")
	for _, x := range folders {
		if x.ID == "/Auth/Login" {
			t.Error("folder should be gone after delete")
		}
	}
	changes, _ := repo.ListPendingChanges("p1")
	if len(changes) != 1 || changes[0].EntityType != "folder_delete" {
		t.Fatalf("pending = %+v, want one folder_delete", changes)
	}

	if err := repo.DiscardPendingChange("p1", changes[0].ID); err != nil {
		t.Fatalf("discard: %v", err)
	}
	folders, _ = repo.ListFolders("p1")
	var restored bool
	for _, x := range folders {
		if x.ID == "/Auth/Login" {
			restored = true
		}
	}
	if !restored {
		t.Error("discarding the delete should restore the folder")
	}
}
