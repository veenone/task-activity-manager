package issuerepo_test

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// A story drafted under a draft epic, and a sub-task under the story. The
// epic's create rekeys it, and the story's JSON has to name the real key
// before the next phase reads it, or Jira gets a TAM-NEW key as an Epic Link.
func TestRekeyRepointsThePlaceholderParentOfEveryDraftUnderIt(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	epic, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeEpic, Summary: "Promotions"})
	if err != nil {
		t.Fatal(err)
	}
	story, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeStory, Summary: "Promo input", ParentKey: epic})
	if err != nil {
		t.Fatal(err)
	}
	sub, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeSubtask, Summary: "Wire it", ParentKey: story})
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.Rekey(ctx, "p1", epic, "PLAT-900"); err != nil {
		t.Fatal(err)
	}

	decode := func(key string) backend.IssueDraft {
		row, ok := rowOf(t, repo, key, issuerepo.EntityIssueCreate)
		if !ok {
			t.Fatalf("no create row for %s", key)
		}
		var d backend.IssueDraft
		if err := json.Unmarshal([]byte(row.AfterVal), &d); err != nil {
			t.Fatal(err)
		}
		return d
	}
	if d := decode(story); d.ParentKey != "PLAT-900" {
		t.Errorf("the story's draft names %q, want the epic's real key", d.ParentKey)
	}
	if d := decode(sub); d.ParentKey != story {
		t.Errorf("the sub-task's parent is the story, still a draft: %q", d.ParentKey)
	}
	if iss, _ := repo.GetIssue(ctx, "p1", story); iss.ParentKey != "PLAT-900" {
		t.Errorf("the story's row names the real epic: %+v", iss)
	}
}

// A link from an issue Jira holds to a draft is journaled under the source,
// so none of Rekey's statements over the draft's own key reach it; left
// alone, its target stays a TAM-NEW key Jira can never resolve.
func TestRekeyRepointsAPendingLinkWhoseTargetIsTheDraft(t *testing.T) {
	repo, db := newRepoWithDB(t)
	ctx := context.Background()
	if err := repo.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatal(err)
	}
	temp, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "Linked to"})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.AddLink(ctx, "p1", "PLAT-412", relates(temp)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO issue_link (profile_id, from_key, to_key, link_type, direction) VALUES ('p1', 'PLAT-409', ?, 'Relates', 'outward')`, temp); err != nil {
		t.Fatal(err)
	}

	if err := repo.Rekey(ctx, "p1", temp, "PLAT-900"); err != nil {
		t.Fatal(err)
	}

	link, ok := rowOf(t, repo, "PLAT-412", issuerepo.EntityLink)
	if !ok || link.Field != "Relates|outward|PLAT-900" {
		t.Fatalf("the pending link's field names the real key: %+v", link)
	}
	var d backend.LinkDraft
	if err := json.Unmarshal([]byte(link.AfterVal), &d); err != nil || d.ToKey != "PLAT-900" || d.Type != "Relates" || d.ToSummary != "Summary of "+temp {
		t.Errorf("the pending link's JSON targets the real key and keeps the rest: %+v, %v", d, err)
	}
	var to string
	if err := db.QueryRow(`SELECT to_key FROM issue_link WHERE profile_id = 'p1' AND from_key = 'PLAT-409'`).Scan(&to); err != nil || to != "PLAT-900" {
		t.Errorf("the cached link targets the real key: %q, %v", to, err)
	}
}

func TestRekeySprintMakesADraftSprintRealEverywhereItIsNamed(t *testing.T) {
	repo, db := newRepoWithDB(t)
	ctx := context.Background()
	if err := repo.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatal(err)
	}
	s, err := repo.CreateDraftSprint(ctx, "p1", sprint15())
	if err != nil {
		t.Fatal(err)
	}
	id := strconv.Itoa(s.ID)
	if err := repo.MoveToSprint(ctx, "p1", "PLAT-409", id, "Sprint 15"); err != nil {
		t.Fatal(err)
	}
	temp, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "In the sprint", SprintID: id, SprintName: "Sprint 15"})
	if err != nil {
		t.Fatal(err)
	}

	made := backend.Sprint{ID: 88, BoardID: 1, Name: "Sprint 15", State: "future", StartDate: s.StartDate, EndDate: s.EndDate, Goal: s.Goal}
	if err := repo.RekeySprint(ctx, "p1", s.ID, made); err != nil {
		t.Fatal(err)
	}

	var draft int
	if err := db.QueryRow(`SELECT draft FROM sprint WHERE profile_id = 'p1' AND id = 88`).Scan(&draft); err != nil || draft != 0 {
		t.Errorf("the sprint row is real: draft %d, %v", draft, err)
	}
	var left int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sprint WHERE profile_id = 'p1' AND id = ?`, s.ID).Scan(&left); err != nil || left != 0 {
		t.Errorf("no row keeps the draft id: %d, %v", left, err)
	}
	if iss, _ := repo.GetIssue(ctx, "p1", "PLAT-409"); iss.SprintID != "88" || iss.SprintName != "Sprint 15" {
		t.Errorf("the moved card names the real sprint: %+v", iss)
	}
	move, ok := rowOf(t, repo, "PLAT-409", issuerepo.EntitySprintMove)
	if !ok || move.AfterVal != "88|Sprint 15" || move.BeforeVal != "12|Sprint 12" {
		t.Errorf("the journaled move targets the real sprint: %+v", move)
	}
	create, _ := rowOf(t, repo, temp, issuerepo.EntityIssueCreate)
	if !strings.Contains(create.AfterVal, `"sprintId":"88"`) {
		t.Errorf("the draft issue's JSON names the real sprint: %s", create.AfterVal)
	}
	if _, ok := rowOf(t, repo, id, issuerepo.EntitySprintCreate); ok {
		t.Error("the sprint_create row is gone")
	}
	act, _ := repo.ListActivity(ctx, "p1", "88", 0)
	if len(act) == 0 || act[0].EntityType != issuerepo.EntitySprint || act[0].Action != "create" {
		t.Errorf("the real sprint's creation is audited: %+v", act)
	}
	if err := repo.RekeySprint(ctx, "p1", s.ID, made); !errors.Is(err, issuerepo.ErrDraftSprintGone) {
		t.Errorf("a second rekey = %v", err)
	}
}

// A boards refresh between the create and the rekey can already have cached
// Jira's new sprint, on the draft's board and on every other board whose
// filter reaches it. The draft's board copy makes way for the rekeyed row;
// another board's copy is that board's, and stays.
func TestRekeySprintReplacesOnlyTheDraftBoardsCopyOfTheNewSprint(t *testing.T) {
	repo, db := newRepoWithDB(t)
	ctx := context.Background()
	s, err := repo.CreateDraftSprint(ctx, "p1", sprint15())
	if err != nil {
		t.Fatal(err)
	}
	for _, board := range []int{1, 2} {
		if _, err := db.Exec(`INSERT INTO sprint (profile_id, id, board_id, name, state) VALUES ('p1', 88, ?, 'Sprint 15 (refreshed)', 'future')`, board); err != nil {
			t.Fatal(err)
		}
	}

	made := backend.Sprint{ID: 88, BoardID: 1, Name: "Sprint 15", State: "future"}
	if err := repo.RekeySprint(ctx, "p1", s.ID, made); err != nil {
		t.Fatal(err)
	}

	names := map[int]string{}
	rows, err := db.Query(`SELECT board_id, name FROM sprint WHERE profile_id = 'p1' AND id = 88`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var board int
		var name string
		if err := rows.Scan(&board, &name); err != nil {
			t.Fatal(err)
		}
		names[board] = name
	}
	if len(names) != 2 || names[1] != "Sprint 15" || names[2] != "Sprint 15 (refreshed)" {
		t.Errorf("board 1 holds the rekeyed row and board 2 keeps its copy: %v", names)
	}
}

func TestMarkSprintCreatedWithoutRekeyLeavesNothingToCreateTwice(t *testing.T) {
	repo, db := newRepoWithDB(t)
	ctx := context.Background()
	s, err := repo.CreateDraftSprint(ctx, "p1", sprint15())
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkSprintCreatedWithoutRekey(ctx, "p1", s.ID, 88); err != nil {
		t.Fatal(err)
	}
	if rows, _ := repo.ListPendingChanges(ctx, "p1"); len(rows) != 0 {
		t.Errorf("nothing left for a retry to create: %+v", rows)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sprint WHERE profile_id = 'p1' AND id = ?`, s.ID).Scan(&n); err != nil || n != 0 {
		t.Errorf("the draft row is gone: %d, %v", n, err)
	}
	act, _ := repo.ListActivity(ctx, "p1", strconv.Itoa(s.ID), 0)
	if len(act) == 0 || !strings.Contains(act[0].Note, "88") {
		t.Errorf("the trail says where the sprint went: %+v", act)
	}
}
