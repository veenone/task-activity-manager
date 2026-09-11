package reports_test

import (
	"fmt"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/reports"
	"agile-suite/tam/internal/sprintdate"
)

// The velocity table is two rules and nothing else: which sprints are in it
// and in what order, which is VelocitySprints, and what one sprint's row
// says, which is Row. They are tested apart here because they ship apart:
// internal/sprintreport calls each of them directly, once, and there is no
// third function in between for a test to aim at.

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

// ids is the sprints a selection picked, in the order it picked them.
func ids(sprints []backend.Sprint) []int {
	out := make([]int, 0, len(sprints))
	for _, sp := range sprints {
		out = append(out, sp.ID)
	}
	return out
}

func TestVelocitySprintsCoversTheLastSixClosedSprintsOldestFirst(t *testing.T) {
	var sprints []backend.Sprint
	for n := 1; n <= 9; n++ {
		sprints = append(sprints, pastSprint(n))
	}
	// A sprint still running has no place in a table of finished work.
	live := pastSprint(10)
	live.State = "active"
	sprints = append(sprints, live)

	// Pinned against the literal the design settled on, not just against
	// the constant: comparing the length to reports.Depth alone would still
	// pass if Depth changed, and the fixed list below would then fail with
	// a message about sprint numbers rather than one saying what changed.
	if reports.Depth != 6 {
		t.Fatalf("this test's table of six sprints assumes Depth is 6; it is %d", reports.Depth)
	}
	got := ids(reports.VelocitySprints(sprints))
	want := []int{4, 5, 6, 7, 8, 9}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("the table covers %v, want %v: the last six closed sprints, oldest first, with the live one left out", got, want)
	}
}

func TestVelocitySprintsOverABoardWithNoClosedSprintIsEmpty(t *testing.T) {
	live := pastSprint(1)
	live.State = "active"
	future := pastSprint(2)
	future.State = "future"

	if got := reports.VelocitySprints([]backend.Sprint{live, future}); len(got) != 0 {
		t.Fatalf("a board that has never closed a sprint has no velocity; got %v", ids(got))
	}
}

// TestASprintTAMCannotReadTheDatesOfIsLeftOutWithoutLosingTheRest covers
// both dates and not only the end. A sprint dropped here is a sprint
// nobody fetches, so an unreadable start that got through would cost
// several pages of changelog before Build refused the series anyway;
// internal/sprintreport has the test that counts those pages.
func TestASprintTAMCannotReadTheDatesOfIsLeftOutWithoutLosingTheRest(t *testing.T) {
	noStart := pastSprint(1)
	noStart.StartDate = ""
	noEnd := pastSprint(2)
	noEnd.EndDate = "the fourteenth"
	fine := pastSprint(3)

	got := ids(reports.VelocitySprints([]backend.Sprint{noStart, noEnd, fine}))
	if len(got) != 1 || got[0] != 3 {
		t.Fatalf("the table covers %v, want only sprint 3: an unreadable date on either end drops that sprint and no other", got)
	}
}

func TestASprintClosedWithCardsStillOpenReportsLessThanItCommitted(t *testing.T) {
	row := reports.Row(build(t, pastSprint(1), afterTheSprint, time.UTC, held(1, 5, 2, points(3))...))

	if row.SprintID != 1 || row.SprintName != "Sprint 1" {
		t.Errorf("the row names sprint %d %q, want 1 \"Sprint 1\"", row.SprintID, row.SprintName)
	}
	if row.Committed != 15 || row.Completed != 6 {
		t.Errorf("five three-point cards with two finished; got committed %v completed %v", row.Committed, row.Completed)
	}
}

func TestABoardThatChangedUnitKeepsEachRowsOwnUnit(t *testing.T) {
	estimated := reports.Row(build(t, pastSprint(1), afterTheSprint, time.UTC, held(1, 4, 2, points(5))...))
	counted := reports.Row(build(t, pastSprint(2), afterTheSprint, time.UTC, held(2, 4, 2, nil)...))

	if estimated.Unit != reports.UnitPoints || estimated.UnitReason != "" {
		t.Errorf("the estimated sprint's row = %+v, want points with nothing to explain", estimated)
	}
	if counted.Unit != reports.UnitCards {
		t.Fatalf("the unestimated sprint's row is counted in %s, want %s", counted.Unit, reports.UnitCards)
	}
	if counted.UnitReason != reports.ReasonNoPointsFieldSeen {
		t.Errorf("the row that counts cards says why; got %q", counted.UnitReason)
	}
	// The reason the unit rides on the row rather than on the table: these
	// two numbers are not the same quantity, and averaging them would give
	// a board that switched halfway through the year one meaningless
	// figure a reader could not see the switch in.
	if counted.Committed != 4 || counted.Completed != 2 {
		t.Errorf("the cards row counts cards, not the points the row beside it used; got %+v", counted)
	}
}

func TestAVelocityRowSaysWhenItsHistoryWasCutShort(t *testing.T) {
	issues := held(1, 2, 1, points(3))
	issues[1].Truncated = true

	whole := reports.Row(build(t, pastSprint(1), afterTheSprint, time.UTC, held(1, 2, 1, points(3))...))
	cut := reports.Row(build(t, pastSprint(1), afterTheSprint, time.UTC, issues...))
	if whole.Truncated {
		t.Errorf("a row built on a whole history must not claim it was cut short; got %+v", whole)
	}
	if !cut.Truncated {
		t.Errorf("a row built on a partial history has to say so; got %+v", cut)
	}
}
