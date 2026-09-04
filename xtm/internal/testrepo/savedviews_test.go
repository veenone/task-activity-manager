package testrepo_test

import (
	"path/filepath"
	"testing"

	"agile-suite/xtm/internal/store"
	"agile-suite/xtm/internal/testrepo"
)

func newViewRepo(t *testing.T) *testrepo.Repository {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return testrepo.NewRepository(st)
}

func TestCreateAndListSavedViews(t *testing.T) {
	repo := newViewRepo(t)

	v, err := repo.CreateSavedView("p1", "Open smoke tests", `{"status":"Open"}`)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if v.ID == "" || v.Name != "Open smoke tests" {
		t.Errorf("view = %+v, want a non-empty id and the given name", v)
	}

	views, err := repo.ListSavedViews("p1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(views) != 1 || views[0].Query != `{"status":"Open"}` {
		t.Errorf("views = %+v, want one with the stored query", views)
	}
}

func TestDeleteSavedViewRemovesIt(t *testing.T) {
	repo := newViewRepo(t)
	v, err := repo.CreateSavedView("p1", "X", "{}")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := repo.DeleteSavedView("p1", v.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	views, _ := repo.ListSavedViews("p1")
	if len(views) != 0 {
		t.Errorf("want no views after delete, got %d", len(views))
	}
}

func TestSavedViewsAreProfileScoped(t *testing.T) {
	repo := newViewRepo(t)
	if _, err := repo.CreateSavedView("p1", "A", "{}"); err != nil {
		t.Fatalf("create p1: %v", err)
	}

	views, _ := repo.ListSavedViews("p2")
	if len(views) != 0 {
		t.Errorf("p2 should see no views from p1, got %d", len(views))
	}
}
