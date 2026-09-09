package boardrepo_test

import (
	"context"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
)

// closed is one finished sprint of board 1, its dates written the way Jira's
// Agile API writes them.
func closed(id int, name, start, end string) backend.Sprint {
	return backend.Sprint{ID: id, BoardID: 1, Name: name, State: "closed", StartDate: start, EndDate: end}
}

// seedSprints puts a board's sprints in the cache through the one write that
// exists for them.
func seedSprints(t *testing.T, r *boardrepo.Repository, sprints []backend.Sprint) {
	t.Helper()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	if err := r.ReplaceBoard(context.Background(), "p1", board, oneColumn(), sprints, nil); err != nil {
		t.Fatalf("seed sprints: %v", err)
	}
}

// TestSprintLengthIsTheMedianOfTheLastThreeClosedSprints uses five closed
// sprints, of which the three most recent ran 10, 14 and 14 days: the median
// is 14, and the two ancient one-week sprints have no say in it. A team that
// changed its cadence gets the cadence it changed to.
func TestSprintLengthIsTheMedianOfTheLastThreeClosedSprints(t *testing.T) {
	r, _ := newRepo(t)
	seedSprints(t, r, []backend.Sprint{
		closed(7, "Sprint 7", "2026-05-04T09:00:00.000+0000", "2026-05-11T09:00:00.000+0000"),
		closed(8, "Sprint 8", "2026-05-11T09:00:00.000+0000", "2026-05-18T09:00:00.000+0000"),
		closed(9, "Sprint 9", "2026-06-01T09:00:00.000+0000", "2026-06-11T09:00:00.000+0000"),
		closed(10, "Sprint 10", "2026-06-15T09:00:00.000+0000", "2026-06-29T09:00:00.000+0000"),
		closed(11, "Sprint 11", "2026-07-06T09:00:00.000+0000", "2026-07-20T09:00:00.000+0000"),
	})

	days, err := r.SprintLength(context.Background(), "p1", 1)
	if err != nil {
		t.Fatalf("sprint length: %v", err)
	}
	if days != 14 {
		t.Errorf("sprint length = %d, want 14, the median of the last three", days)
	}
}

// TestSprintLengthIgnoresASprintMissingADate keeps a half-recorded sprint
// from being read as a sprint of no length at all, which would drag the
// median down to something no team ever ran.
func TestSprintLengthIgnoresASprintMissingADate(t *testing.T) {
	r, _ := newRepo(t)
	seedSprints(t, r, []backend.Sprint{
		closed(9, "Sprint 9", "2026-06-01T09:00:00Z", ""),
		closed(10, "Sprint 10", "2026-06-15T09:00:00Z", "2026-06-29T09:00:00Z"),
		closed(11, "Sprint 11", "2026-07-06T09:00:00Z", "2026-07-20T09:00:00Z"),
	})

	days, err := r.SprintLength(context.Background(), "p1", 1)
	if err != nil {
		t.Fatalf("sprint length: %v", err)
	}
	if days != 14 {
		t.Errorf("sprint length = %d, want the two readable sprints' 14", days)
	}
}

// TestSprintLengthOfABoardWithNoHistoryIsZero pins the contract the caller
// depends on: zero is not a length and not an error, it is "this board has
// nothing to suggest from", and the sprints service is what decides that
// means a fortnight. An active or future sprint is not history: its dates
// are a plan, and one of them is often not set at all.
func TestSprintLengthOfABoardWithNoHistoryIsZero(t *testing.T) {
	r, _ := newRepo(t)
	seedSprints(t, r, []backend.Sprint{
		{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active", StartDate: "2026-08-18T09:00:00Z", EndDate: "2026-09-01T09:00:00Z"},
		{ID: 13, BoardID: 1, Name: "Sprint 13", State: "future", StartDate: "2026-09-01T09:00:00Z", EndDate: "2026-09-15T09:00:00Z"},
	})

	days, err := r.SprintLength(context.Background(), "p1", 1)
	if err != nil {
		t.Fatalf("sprint length: %v", err)
	}
	if days != 0 {
		t.Errorf("sprint length = %d, want 0 for a board with no closed sprints", days)
	}
	empty, err := r.SprintLength(context.Background(), "p1", 99)
	if err != nil || empty != 0 {
		t.Errorf("sprint length of an unknown board = %d, %v, want 0 and no error", empty, err)
	}
}

// TestSprintLengthIsPerBoard keeps one team's cadence out of another's
// dialog: the tables are keyed by board, and so is this read.
func TestSprintLengthIsPerBoard(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	seedSprints(t, r, []backend.Sprint{
		closed(11, "Sprint 11", "2026-07-06T09:00:00Z", "2026-07-20T09:00:00Z"),
	})
	other := backend.Board{ID: 2, Name: "PLAT Kanban", Type: backend.BoardTypeKanban}
	weekly := []backend.Sprint{{ID: 21, BoardID: 2, Name: "Week 21", State: "closed", StartDate: "2026-07-06T09:00:00Z", EndDate: "2026-07-13T09:00:00Z"}}
	if err := r.ReplaceBoard(ctx, "p1", other, oneColumn(), weekly, nil); err != nil {
		t.Fatalf("seed the other board: %v", err)
	}

	first, err := r.SprintLength(ctx, "p1", 1)
	if err != nil {
		t.Fatalf("board 1: %v", err)
	}
	second, err := r.SprintLength(ctx, "p1", 2)
	if err != nil {
		t.Fatalf("board 2: %v", err)
	}
	if first != 14 || second != 7 {
		t.Errorf("lengths = %d and %d, want each board's own 14 and 7", first, second)
	}
}
