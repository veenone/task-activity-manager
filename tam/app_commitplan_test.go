package main

import (
	"strconv"
	"testing"

	"agile-suite/tam/internal/backend"
	demobackend "agile-suite/tam/internal/backend/demo"
)

// The ticket's plan, end to end on the demo backend: a new sprint, an epic, a
// story under the epic in the sprint, and a technical task under the story,
// drafted offline and committed with one press.
func TestTheTicketsPlanCommitsInOnePressOnTheDemoBackend(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	demo := demobackend.New(p.ProjectKey)
	a.backends[p.ID] = demo
	if err := a.boards.ReplaceBoard(a.ctx, p.ID, backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	sprint, err := a.CreateSprint(p.ID, 1, "Sprint 15", "Ship promos", "2026-09-16", "2026-09-30")
	if err != nil {
		t.Fatal(err)
	}
	sid := strconv.Itoa(sprint.Sprint.ID)
	epic, err := a.CreateIssue(p.ID, backend.IssueDraft{Type: backend.TypeEpic, Summary: "Promotions"})
	if err != nil {
		t.Fatal(err)
	}
	story, err := a.CreateIssue(p.ID, backend.IssueDraft{Type: backend.TypeStory, Summary: "Promo input", ParentKey: epic, SprintID: sid, SprintName: "Sprint 15"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateIssue(p.ID, backend.IssueDraft{Type: backend.TypeSubtask, Summary: "Wire the input", ParentKey: story}); err != nil {
		t.Fatal(err)
	}

	res, err := a.CommitPendingChanges(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 0 || len(res.Held) != 0 || res.Remaining != 0 || len(res.Created) != 3 || len(res.CreatedSprints) != 1 {
		t.Fatalf("result = %+v", res)
	}
	realKeys := map[string]string{}
	for _, c := range res.Created {
		realKeys[c.TempKey] = c.Key
	}
	sprintID := strconv.Itoa(res.CreatedSprints[0].ID)
	gotStory, err := demo.GetIssue(a.ctx, realKeys[story])
	if err != nil || gotStory.ParentKey != realKeys[epic] || gotStory.SprintID != sprintID {
		t.Errorf("story in Jira = %+v, %v; want under %s in sprint %s", gotStory, err, realKeys[epic], sprintID)
	}
	for temp, key := range realKeys {
		if temp == story || temp == epic {
			continue
		}
		sub, err := demo.GetIssue(a.ctx, key)
		if err != nil || sub.ParentKey != realKeys[story] {
			t.Errorf("technical task in Jira = %+v, %v; want under %s", sub, err, realKeys[story])
		}
	}
	if open, _ := a.ListOpenSprints(p.ID); len(open) != 1 || open[0].Draft || open[0].ID != res.CreatedSprints[0].ID {
		t.Errorf("the picker offers the real sprint now: %+v", open)
	}
}
