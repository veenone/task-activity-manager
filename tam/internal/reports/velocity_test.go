package reports_test

import (
	"fmt"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/reports"
	"agile-suite/tam/internal/sprintdate"
)

// firstMonday is where the run of fortnightly sprints below starts, so
// every one of them opens on a Monday and closes on the second Friday.
var firstMonday = time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)

// pastSprint is sprint n of a fortnightly run, closed, with its dates
// written through sprintdate so the fixtures carry the same format Jira
// sends rather than one this package invented.
func pastSprint(n int) backend.Sprint {
	start := firstMonday.AddDate(0, 0, 14*(n-1))
	return backend.Sprint{
		ID:        n,
		Name:      fmt.Sprintf("Sprint %d", n),
		State:     "closed",
		StartDate: sprintdate.Format(start),
		EndDate:   sprintdate.Format(start.AddDate(0, 0, 11)),
	}
}

// held is one sprint's issues, all of them in it from the start, with the
// first finished and the rest still open.
func held(sprintID, count, finished int, estimate *float64) []backend.IssueHistory {
	out := make([]backend.IssueHistory, 0, count)
	for i := 0; i < count; i++ {
		status, statusID := "To Do", "1"
		if i < finished {
			status, statusID = "Done", "10001"
		}
		out = append(out, card(fmt.Sprintf("PLAT-%d%02d", sprintID, i), status, statusID, fmt.Sprint(sprintID), estimate))
	}
	return out
}

func TestVelocityCoversTheLastSixClosedSprintsOldestFirst(t *testing.T) {
	var sprints []backend.Sprint
	histories := map[int][]backend.IssueHistory{}
	for n := 1; n <= 9; n++ {
		sprints = append(sprints, pastSprint(n))
		histories[n] = held(n, n, 1, points(2))
	}
	// A sprint still running has no place in a table of finished work,
	// and its issues are here to prove that is what excludes it.
	live := pastSprint(10)
	live.State = "active"
	sprints = append(sprints, live)
	histories[10] = held(10, 3, 0, points(2))

	// Pinned against the literal the design settled on, not just against
	// the constant: comparing len(rows) to reports.Depth alone would still
	// pass if Depth changed, and the fixed-length table below would then
	// panic indexing past a shorter rows instead of failing with a message
	// that says what changed.
	if reports.Depth != 6 {
		t.Fatalf("this test's table of six sprints assumes Depth is 6; it is %d", reports.Depth)
	}
	rows := reports.Velocity(sprints, doneRule(), histories, afterTheSprint, time.UTC)
	if len(rows) != reports.Depth {
		t.Fatalf("nine closed sprints give the last %d; got %d", reports.Depth, len(rows))
	}
	for i, want := range []int{4, 5, 6, 7, 8, 9} {
		if rows[i].SprintID != want {
			t.Errorf("row %d is sprint %d, want %d (the rows read oldest first)", i, rows[i].SprintID, want)
		}
	}
	if rows[0].Committed != 8 || rows[0].Completed != 2 {
		t.Errorf("sprint 4 held four two-point cards and finished one; got committed %v completed %v", rows[0].Committed, rows[0].Completed)
	}
}

func TestVelocityOverABoardWithNoClosedSprintIsEmpty(t *testing.T) {
	live := pastSprint(1)
	live.State = "active"
	future := pastSprint(2)
	future.State = "future"
	rows := reports.Velocity([]backend.Sprint{live, future}, doneRule(),
		map[int][]backend.IssueHistory{1: held(1, 2, 0, points(3)), 2: nil}, afterTheSprint, time.UTC)
	if len(rows) != 0 {
		t.Fatalf("a board that has never closed a sprint has no velocity; got %d rows", len(rows))
	}
}

func TestASprintClosedWithCardsStillOpenReportsLessThanItCommitted(t *testing.T) {
	rows := reports.Velocity([]backend.Sprint{pastSprint(1)}, doneRule(),
		map[int][]backend.IssueHistory{1: held(1, 5, 2, points(3))}, afterTheSprint, time.UTC)
	if len(rows) != 1 {
		t.Fatalf("one closed sprint is one row; got %d", len(rows))
	}
	if rows[0].Committed != 15 || rows[0].Completed != 6 {
		t.Errorf("five three-point cards with two finished; got committed %v completed %v", rows[0].Committed, rows[0].Completed)
	}
}

func TestABoardThatChangedUnitKeepsEachRowsOwnUnit(t *testing.T) {
	sprints := []backend.Sprint{pastSprint(1), pastSprint(2), pastSprint(3)}
	histories := map[int][]backend.IssueHistory{
		1: held(1, 4, 2, points(5)),
		2: held(2, 4, 2, points(5)),
		3: held(3, 4, 2, nil),
	}
	rows := reports.Velocity(sprints, doneRule(), histories, afterTheSprint, time.UTC)
	if len(rows) != 3 {
		t.Fatalf("three closed sprints are three rows; got %d", len(rows))
	}
	for i, want := range []string{reports.UnitPoints, reports.UnitPoints, reports.UnitCards} {
		if rows[i].Unit != want {
			t.Fatalf("row %d (sprint %d) is counted in %s, want %s", i, rows[i].SprintID, rows[i].Unit, want)
		}
	}
	if rows[2].UnitReason != reports.ReasonNoPointsFieldSeen {
		t.Errorf("the row that counts cards says why; got %q", rows[2].UnitReason)
	}
	if rows[2].Committed != 4 || rows[2].Completed != 2 {
		t.Errorf("the cards row counts cards, not the points the rows above it used; got %+v", rows[2])
	}
}

func TestASprintWhoseHistoryWasNeverFetchedIsNotARow(t *testing.T) {
	sprints := []backend.Sprint{pastSprint(1), pastSprint(2)}
	rows := reports.Velocity(sprints, doneRule(),
		map[int][]backend.IssueHistory{2: held(2, 1, 1, points(3))}, afterTheSprint, time.UTC)
	if len(rows) != 1 || rows[0].SprintID != 2 {
		t.Fatalf("a sprint nobody fetched a changelog for is left out rather than shown as a zero; got %+v", rows)
	}
}

func TestASprintTAMCannotReadTheDatesOfIsLeftOutWithoutLosingTheRest(t *testing.T) {
	broken := pastSprint(1)
	broken.StartDate = ""
	rows := reports.Velocity([]backend.Sprint{broken, pastSprint(2)}, doneRule(),
		map[int][]backend.IssueHistory{1: held(1, 2, 1, points(3)), 2: held(2, 2, 1, points(3))},
		afterTheSprint, time.UTC)
	if len(rows) != 1 || rows[0].SprintID != 2 {
		t.Fatalf("one unreadable sprint must not take the readable ones with it; got %+v", rows)
	}
}

func TestAVelocityRowSaysWhenItsHistoryWasCutShort(t *testing.T) {
	issues := held(1, 2, 1, points(3))
	issues[1].Truncated = true
	rows := reports.Velocity([]backend.Sprint{pastSprint(1)}, doneRule(),
		map[int][]backend.IssueHistory{1: issues}, afterTheSprint, time.UTC)
	if len(rows) != 1 || !rows[0].Truncated {
		t.Fatalf("a row built on a partial history has to say so; got %+v", rows)
	}
}
