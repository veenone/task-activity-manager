package main

import (
	"context"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
)

func TestListOpenSprintsAnswersAcrossTheProfilesBoards(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	ctx := context.Background()
	scrum := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	// The future sprint carries the lower id on purpose, so plain id order and
	// the order this asks for disagree: without that, the assertion below
	// would pass on a read that had never heard of a sprint's state.
	sprints := []backend.Sprint{
		{ID: 11, BoardID: 1, Name: "Sprint 11", State: "closed", StartDate: "2026-08-04T09:00:00Z"},
		{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active", StartDate: "2026-08-18T09:00:00Z"},
		{ID: 9, BoardID: 1, Name: "Sprint 9", State: "future", StartDate: "2026-09-01T09:00:00Z"},
	}
	if err := a.boards.ReplaceBoard(ctx, p.ID, scrum, nil, sprints, nil); err != nil {
		t.Fatalf("seed board: %v", err)
	}

	open, err := a.ListOpenSprints(p.ID)
	if err != nil {
		t.Fatalf("ListOpenSprints: %v", err)
	}
	if len(open) != 2 || open[0].Name != "Sprint 12" || open[1].Name != "Sprint 9" {
		t.Fatalf("the two open sprints, active first: %+v", open)
	}
	if open[0].BoardName != "PLAT Scrum" {
		t.Errorf("each choice names its board: %+v", open[0])
	}
}

func TestListOpenSprintsOfAProfileWithNoBoardsIsAnEmptyList(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)

	open, err := a.ListOpenSprints(p.ID)
	if err != nil {
		t.Fatalf("ListOpenSprints: %v", err)
	}
	if open == nil || len(open) != 0 {
		t.Errorf("an unrefreshed profile has no sprints and no error: %+v", open)
	}
}

func TestListOpenSprintsNeedsAProfile(t *testing.T) {
	a := newTestApp(t)
	if _, err := a.ListOpenSprints(""); err == nil || !strings.Contains(err.Error(), "no profile selected") {
		t.Errorf("err = %v", err)
	}
}
