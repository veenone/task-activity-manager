package boardrepo_test

import (
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
	"context"
	"testing"
)

func TestSubtaskDraftsFollowParentBoardAndSprint(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	parent := card("P-1", "To Do", "1")
	parent.Type, parent.ParentKey, parent.Assignee, parent.SprintID = "story", "P-EPIC", "Parent owner", "12"
	child := backend.Issue{Key: "TAM-NEW-1", Type: "subtask", ParentKey: parent.Key, Draft: true, Assignee: "Child owner"}
	src := newIssues(parent).withDrafts(child, backend.Issue{Key: "TAM-NEW-2", Type: "subtask", ParentKey: "P-404", Draft: true})
	for _, id := range []int{1, 2} {
		keys := map[string][]string{}
		if id == 1 {
			keys = map[string][]string{"": {parent.Key}, "12": {parent.Key}}
		}
		if err := r.ReplaceBoard(ctx, "p1", backend.Board{ID: id, Type: "scrum"}, sampleColumns(), []backend.Sprint{{ID: 12, BoardID: id, Name: "Sprint 12", State: "active"}, {ID: 13, BoardID: id, Name: "Sprint 13", State: "future"}}, keys); err != nil {
			t.Fatal(err)
		}
		for _, sprint := range []string{"12", "13"} {
			for _, lane := range []string{boardrepo.SwimlaneNone, boardrepo.SwimlaneEpic, boardrepo.SwimlaneAssignee} {
				view, err := r.Board(ctx, src, "p1", id, sprint, lane)
				if err != nil {
					t.Fatal(err)
				}
				var count int
				for _, l := range view.Lanes {
					for _, cell := range l.Cells {
						for _, c := range cell {
							if c.Type == "subtask" {
								count++
								if c.Key != child.Key || c.SprintID != "12" || c.Assignee != "Child owner" {
									t.Fatalf("wrong child: %+v", c)
								}
								if lane == boardrepo.SwimlaneEpic && l.ID != "P-EPIC" {
									t.Fatalf("wrong epic lane: %+v", l)
								}
							}
						}
					}
				}
				want := 0
				if id == 1 && sprint == "12" {
					want = 1
				}
				if count != want {
					t.Fatalf("board %d sprint %s has %d drafts, want %d", id, sprint, count, want)
				}
			}
		}
		details, err := r.BoardSprintDetails(ctx, src, "p1", id)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range details {
			var count int
			for _, c := range d.Issues {
				if c.Type == "subtask" {
					count++
				}
			}
			want := 0
			if id == 1 && d.ID == 12 {
				want = 1
			}
			if count != want {
				t.Fatalf("board %d sprint detail %d has %d drafts, want %d", id, d.ID, count, want)
			}
		}
	}
}
