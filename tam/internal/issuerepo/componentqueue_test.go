package issuerepo_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

func TestQueueComponentDeleteJournalsAndDropsAnEdit(t *testing.T) {
	repo := newRepo(t)
	seedCatalog(t, repo)
	ctx := context.Background()
	_ = repo.EditComponent(ctx, "p1", 10000, issuerepo.ComponentFields{Name: "Login"})
	if err := repo.QueueComponentDelete(ctx, "p1", 10000, 10001); err != nil {
		t.Fatal(err)
	}
	if rows := pendingOf(t, repo, issuerepo.EntityComponentEdit); len(rows) != 0 {
		t.Fatalf("edit kept %+v", rows)
	}
	if _, ok := componentNamed(t, repo, "Auth"); !ok {
		t.Fatal("the dropped edit did not put the name back")
	}
	rows := pendingOf(t, repo, issuerepo.EntityComponentDelete)
	if len(rows) != 1 || rows[0].EntityKey != "10000" {
		t.Fatalf("journal %+v", rows)
	}
	var d issuerepo.ComponentDelete
	if err := json.Unmarshal([]byte(rows[0].AfterVal), &d); err != nil || d.Name != "Auth" || d.MoveTo != 10001 || d.MoveToName != "Board" {
		t.Fatalf("payload %+v %v", d, err)
	}
	// A queued delete refuses an edit and a second delete.
	if err := repo.EditComponent(ctx, "p1", 10000, issuerepo.ComponentFields{Name: "Login"}); err == nil {
		t.Error("edit of a component queued for delete: no error")
	}
	if err := repo.QueueComponentDelete(ctx, "p1", 10000, 0); err == nil {
		t.Error("second delete: no error")
	}
}

func TestQueueComponentDeleteRefusesItselfAsTarget(t *testing.T) {
	repo := newRepo(t)
	seedCatalog(t, repo)
	if err := repo.QueueComponentDelete(context.Background(), "p1", 10000, 10000); err == nil {
		t.Fatal("moving issues to the component being deleted: no error")
	}
	if rows := pendingOf(t, repo, issuerepo.EntityComponentDelete); len(rows) != 0 {
		t.Fatalf("journal %+v", rows)
	}
}

func TestQueueComponentDeleteOfADraftDiscardsIt(t *testing.T) {
	repo := newRepo(t)
	seedCatalog(t, repo)
	ctx := context.Background()
	made, _ := repo.CreateDraftComponent(ctx, "p1", "PLAT", issuerepo.ComponentFields{Name: "Reports"})
	if err := repo.QueueComponentDelete(ctx, "p1", made.ID, 0); err != nil {
		t.Fatal(err)
	}
	if _, ok := componentNamed(t, repo, "Reports"); ok {
		t.Fatal("draft kept")
	}
	if rows := pendingOf(t, repo, issuerepo.EntityComponentCreate); len(rows) != 0 {
		t.Fatalf("create row kept %+v", rows)
	}
}

func TestRekeyComponentRepointsIssuesAndTargets(t *testing.T) {
	repo := newRepo(t)
	seedCatalog(t, repo)
	seedComponentIssues(t, repo)
	ctx := context.Background()
	made, _ := repo.CreateDraftComponent(ctx, "p1", "PLAT", issuerepo.ComponentFields{Name: "Reports"})
	draft := strconv.Itoa(made.ID)
	if err := repo.EditField(ctx, "p1", "PLAT-2", "components", draft); err != nil {
		t.Fatal(err)
	}
	if err := repo.QueueComponentDelete(ctx, "p1", 10001, made.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.RekeyComponent(ctx, "p1", made.ID, backend.Component{ID: 10300, Name: "Reports", AssigneeType: "PROJECT_DEFAULT"}); err != nil {
		t.Fatal(err)
	}
	c, ok := componentNamed(t, repo, "Reports")
	if !ok || c.ID != 10300 || c.Draft {
		t.Fatalf("row %+v %v", c, ok)
	}
	two, _ := repo.GetIssue(ctx, "p1", "PLAT-2")
	if !reflect.DeepEqual(two.Components, []int{10300}) {
		t.Fatalf("PLAT-2 components %v", two.Components)
	}
	edits := pendingOf(t, repo, issuerepo.EntityIssue)
	if len(edits) != 1 || edits[0].AfterVal != "10300" {
		t.Fatalf("edits %+v", edits)
	}
	var d issuerepo.ComponentDelete
	_ = json.Unmarshal([]byte(pendingOf(t, repo, issuerepo.EntityComponentDelete)[0].AfterVal), &d)
	if d.MoveTo != 10300 {
		t.Fatalf("delete target %+v", d)
	}
	if rows := pendingOf(t, repo, issuerepo.EntityComponentCreate); len(rows) != 0 {
		t.Fatalf("create row kept %+v", rows)
	}
	if err := repo.RekeyComponent(ctx, "p1", made.ID, backend.Component{ID: 10301}); !errors.Is(err, issuerepo.ErrDraftComponentGone) {
		t.Fatalf("second rekey: %v", err)
	}
}

func TestApplyComponentDeleteMovesIssuesAndDropsTheRow(t *testing.T) {
	repo := newRepo(t)
	seedCatalog(t, repo)
	seedComponentIssues(t, repo)
	ctx := context.Background()
	if err := repo.QueueComponentDelete(ctx, "p1", 10000, 10001); err != nil {
		t.Fatal(err)
	}
	row := pendingOf(t, repo, issuerepo.EntityComponentDelete)[0]
	if err := repo.ApplyComponentCommitted(ctx, "p1", row); err != nil {
		t.Fatal(err)
	}
	if _, ok := componentNamed(t, repo, "Auth"); ok {
		t.Fatal("row kept")
	}
	one, _ := repo.GetIssue(ctx, "p1", "PLAT-1")
	if !reflect.DeepEqual(one.Components, []int{10001}) {
		t.Fatalf("PLAT-1 components %v", one.Components)
	}
	if rows := pendingOf(t, repo, issuerepo.EntityComponentDelete); len(rows) != 0 {
		t.Fatalf("journal kept %+v", rows)
	}
}

func TestComponentIssuesAndCounts(t *testing.T) {
	repo := newRepo(t)
	seedCatalog(t, repo)
	seedComponentIssues(t, repo)
	ctx := context.Background()
	issues, err := repo.ComponentIssues(ctx, "p1", 10000)
	if err != nil || len(issues) != 1 || issues[0].Key != "PLAT-1" {
		t.Fatalf("issues %+v %v", issues, err)
	}
	none, err := repo.ComponentIssues(ctx, "p1", 10001)
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("empty component %+v %v", none, err)
	}
	counts, err := repo.ComponentCounts(ctx, "p1")
	if err != nil || counts[10000] != 1 || counts[10001] != 0 {
		t.Fatalf("counts %v %v", counts, err)
	}
}
