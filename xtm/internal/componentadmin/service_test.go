package componentadmin

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/jira"
)

type fakeRepo struct {
	options []string
	pending map[string]int
	renamed [][2]string
	removed []string
}

func (f *fakeRepo) ReplaceProjectFieldOptions(_, _, field string, values []string) error {
	if field == "component" {
		f.options = values
	}
	return nil
}
func (f *fakeRepo) ComponentEditsPending(_, name string) (int, error) { return f.pending[name], nil }
func (f *fakeRepo) RenameComponentOnTests(_, o, n string) (int, error) {
	f.renamed = append(f.renamed, [2]string{o, n})
	return 1, nil
}
func (f *fakeRepo) RemoveComponentFromTests(_, name string) (int, error) {
	f.removed = append(f.removed, name)
	return 1, nil
}

type fakeBackend struct {
	backend.ComponentManager
	list    []backend.Component
	err     error
	deleted []string
}

func (f *fakeBackend) ProjectComponentDetails(context.Context, string) ([]backend.Component, error) {
	return f.list, nil
}
func (f *fakeBackend) ProjectComponents(context.Context, string) ([]string, error) {
	out := []string{}
	for _, c := range f.list {
		out = append(out, c.Name)
	}
	return out, nil
}
func (f *fakeBackend) CreateComponent(_ context.Context, _ string, in backend.ComponentInput) (backend.Component, error) {
	if f.err != nil {
		return backend.Component{}, f.err
	}
	c := backend.Component{ID: "new", Name: in.Name}
	f.list = append(f.list, c)
	return c, nil
}
func (f *fakeBackend) UpdateComponent(_ context.Context, id string, in backend.ComponentInput) (backend.Component, error) {
	for i := range f.list {
		if f.list[i].ID == id {
			f.list[i].Name = in.Name
			return f.list[i], nil
		}
	}
	return backend.Component{}, &jira.HTTPError{Code: 404, Status: "404 Not Found", Message: "gone"}
}
func (f *fakeBackend) DeleteComponent(_ context.Context, id, _ string) error {
	f.deleted = append(f.deleted, id)
	out := f.list[:0]
	for _, c := range f.list {
		if c.ID != id {
			out = append(out, c)
		}
	}
	f.list = out
	return nil
}

func newSvc(repo *fakeRepo, b *fakeBackend) *Service { return New(repo, b, "p1", "QA") }

func TestCreateRefreshesCache(t *testing.T) {
	repo := &fakeRepo{}
	b := &fakeBackend{list: []backend.Component{{ID: "1", Name: "Core"}}}
	if _, err := newSvc(repo, b).Create(context.Background(), backend.ComponentInput{Name: " API "}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if !reflect.DeepEqual(repo.options, []string{"Core", "API"}) {
		t.Fatalf("cache %v", repo.options)
	}
}

func TestCreateRejectsBlankName(t *testing.T) {
	_, err := newSvc(&fakeRepo{}, &fakeBackend{}).Create(context.Background(), backend.ComponentInput{Name: "  "})
	if err == nil || err.Error() != "A component needs a name." {
		t.Fatalf("err %v", err)
	}
}

func TestForbiddenBecomesReadableError(t *testing.T) {
	b := &fakeBackend{err: &jira.HTTPError{Code: http.StatusForbidden, Status: "403 Forbidden"}}
	_, err := newSvc(&fakeRepo{}, b).Create(context.Background(), backend.ComponentInput{Name: "X"})
	if !errors.Is(err, ErrForbidden) || err.Error() != "You need project admin rights in Jira to change components." {
		t.Fatalf("err %v", err)
	}
}

func TestRenameRewritesTestsAndRefuses(t *testing.T) {
	repo := &fakeRepo{pending: map[string]int{}}
	b := &fakeBackend{list: []backend.Component{{ID: "1", Name: "core"}}}
	svc := newSvc(repo, b)
	if _, err := svc.Update(context.Background(), "1", backend.ComponentInput{Name: "Core"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if !reflect.DeepEqual(repo.renamed, [][2]string{{"core", "Core"}}) {
		t.Fatalf("renamed %v", repo.renamed)
	}

	repo.pending["Core"] = 2
	_, err := svc.Update(context.Background(), "1", backend.ComponentInput{Name: "Kernel"})
	want := `Commit or discard the 2 pending component edits on tests that use "Core" first.`
	if err == nil || err.Error() != want {
		t.Fatalf("err %v", err)
	}
	if b.list[0].Name != "Core" {
		t.Fatal("Jira was called despite the guard")
	}
}

func TestUpdateWithoutRenameSkipsGuardAndRewrite(t *testing.T) {
	repo := &fakeRepo{pending: map[string]int{"Core": 3}}
	b := &fakeBackend{list: []backend.Component{{ID: "1", Name: "Core"}}}
	if _, err := newSvc(repo, b).Update(context.Background(), "1", backend.ComponentInput{Name: "Core", Description: "x"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(repo.renamed) != 0 {
		t.Fatalf("renamed %v", repo.renamed)
	}
}

func TestDeleteMovesOrRemoves(t *testing.T) {
	repo := &fakeRepo{pending: map[string]int{}}
	b := &fakeBackend{list: []backend.Component{{ID: "1", Name: "Old"}, {ID: "2", Name: "New"}}}
	svc := newSvc(repo, b)
	if err := svc.Delete(context.Background(), "1", "2"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !reflect.DeepEqual(repo.renamed, [][2]string{{"Old", "New"}}) {
		t.Fatalf("move should rename on tests, got %v", repo.renamed)
	}
	if err := svc.Delete(context.Background(), "2", ""); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !reflect.DeepEqual(repo.removed, []string{"New"}) {
		t.Fatalf("removed %v", repo.removed)
	}
	if !reflect.DeepEqual(repo.options, []string{}) {
		t.Fatalf("cache %v", repo.options)
	}
}

func TestUpdateOfMissingComponentRefreshesCache(t *testing.T) {
	repo := &fakeRepo{pending: map[string]int{}}
	b := &fakeBackend{list: []backend.Component{{ID: "2", Name: "Other"}}}
	_, err := newSvc(repo, b).Update(context.Background(), "1", backend.ComponentInput{Name: "X"})
	if err == nil {
		t.Fatal("want error")
	}
	if !reflect.DeepEqual(repo.options, []string{"Other"}) {
		t.Fatalf("cache not refreshed after 404: %v", repo.options)
	}
}
