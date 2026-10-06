package reports_test

import (
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/reports"
)

// hours is an estimate written the way a reader thinks of one.
func hours(n int) *int { v := n * 3600; return &v }

// logged is one worklog entry: the day it was started and how long it ran.
// Started carries Jira's own offset, which is what dates the entry.
func logged(date string, hour int, h float64) backend.Worklog {
	return backend.Worklog{Started: on(date, hour), Seconds: int(h * 3600)}
}

// timed is card() with an estimate in seconds and a worklog list, which the
// points fixtures have no use for.
func timed(key, status, statusID string, estimate *int, logs []backend.Worklog, changes ...backend.Change) backend.IssueHistory {
	h := card(key, status, statusID, "11", nil, changes...)
	h.Issue.OriginalEstimateSeconds = estimate
	h.Worklogs = logs
	return h
}

// timeDay is the series' time figures for one date.
func timeDay(t *testing.T, s reports.Series, date string) reports.Day {
	t.Helper()
	for _, d := range s.TimeDays {
		if d.Date == date {
			return d
		}
	}
	t.Fatalf("the series has no time day %s", date)
	return reports.Day{}
}

// The burndown in hours: the estimates are the scope, and the work logged
// against them is what burns it down, day by day.
func TestTimeBurndownBurnsTheLoggedWorkAgainstTheEstimates(t *testing.T) {
	s := build(t, sprint11(), afterTheSprint, time.UTC,
		timed("PLAT-1", "In Progress", "3", hours(8), []backend.Worklog{logged("2026-08-04", 10, 3)}),
		timed("PLAT-2", "To Do", "1", hours(4), nil),
	)
	// Twelve hours taken on, nothing burned on the first day.
	first := timeDay(t, s, "2026-08-03")
	if first.Scope != 12 || first.Remaining != 12 {
		t.Errorf("3 Aug = scope %v remaining %v, want 12 and 12", first.Scope, first.Remaining)
	}
	// Three hours logged on the second day leave nine.
	second := timeDay(t, s, "2026-08-04")
	if second.Remaining != 9 {
		t.Errorf("4 Aug remaining = %v, want 9", second.Remaining)
	}
	// And they stay burned: a later day with nothing logged holds the line.
	if third := timeDay(t, s, "2026-08-05"); third.Remaining != 9 {
		t.Errorf("5 Aug remaining = %v, want the 9 the day before left", third.Remaining)
	}
	// The guide runs the committed total down over the sprint's working
	// days, the same shape the points guide has.
	if first.Ideal <= 0 || first.Ideal > 12 {
		t.Errorf("3 Aug ideal = %v, want the committed total stepping down", first.Ideal)
	}
}

// Work logged before the sprint opened was not done inside it, and work
// logged after it closed is not part of what it burned.
func TestWorkOutsideTheSprintDoesNotBurnIt(t *testing.T) {
	s := build(t, sprint11(), afterTheSprint, time.UTC,
		timed("PLAT-1", "In Progress", "3", hours(8), []backend.Worklog{
			logged("2026-07-28", 10, 5), // before the sprint
			logged("2026-08-20", 10, 2), // after it closed
			logged("2026-08-05", 10, 1),
		}),
	)
	last := timeDay(t, s, "2026-08-14")
	if last.Remaining != 7 {
		t.Errorf("remaining at the close = %v, want 7: only the hour logged inside the sprint burns it", last.Remaining)
	}
}

// Scope steps the day a card joins, the way the points scope does.
func TestAnIssueAddedMidSprintRaisesTheTimeScopeThatDay(t *testing.T) {
	s := build(t, sprint11(), afterTheSprint, time.UTC,
		timed("PLAT-1", "To Do", "1", hours(8), nil),
		timed("PLAT-2", "To Do", "1", hours(5), nil,
			backend.Change{At: on("2026-08-06", 11), Field: "sprint", From: "", To: "Sprint 11"}),
	)
	if before := timeDay(t, s, "2026-08-05"); before.Scope != 8 {
		t.Errorf("5 Aug scope = %v, want the 8 hours the sprint opened with", before.Scope)
	}
	if after := timeDay(t, s, "2026-08-06"); after.Scope != 13 {
		t.Errorf("6 Aug scope = %v, want 13 once the second card joined", after.Scope)
	}
}

// A sprint nobody estimated in time has no time burndown at all. A flat
// line at zero would say the team finished everything on the first day.
func TestASprintWithNoEstimatesHasNoTimeBurndown(t *testing.T) {
	s := build(t, sprint11(), afterTheSprint, time.UTC,
		timed("PLAT-1", "To Do", "1", nil, []backend.Worklog{logged("2026-08-04", 10, 2)}),
	)
	if len(s.TimeDays) != 0 {
		t.Errorf("time days = %d, want none for a sprint with no estimate on any card", len(s.TimeDays))
	}
}

// Remaining stops at zero. A team that logs more hours than it estimated
// has overrun, which the line says by reaching the floor and staying
// there, not by burning through it into negative work.
func TestRemainingDoesNotGoBelowZero(t *testing.T) {
	s := build(t, sprint11(), afterTheSprint, time.UTC,
		timed("PLAT-1", "In Progress", "3", hours(2), []backend.Worklog{logged("2026-08-04", 10, 6)}),
	)
	if d := timeDay(t, s, "2026-08-04"); d.Remaining != 0 {
		t.Errorf("remaining = %v, want 0 rather than negative work", d.Remaining)
	}
}

// family is one card whose figures live on its sub-tasks: nothing of its
// own, everything in the aggregates, and its children's worklogs beside
// it the way the fetch attaches them.
func family(key string, famEstimate int, logs []backend.Worklog, changes ...backend.Change) backend.IssueHistory {
	h := card(key, "In Progress", "3", "11", nil, changes...)
	h.Issue.AggregateEstimateSeconds = hours(famEstimate)
	h.Issue.AggregateTimeSpentSeconds = hours(0)
	h.SubtaskWorklogs = logs
	return h
}

// The issue #142 was raised for, as a sprint: every estimate is on the
// sub-tasks, and the sub-tasks are not in the sprint. Counting only what
// the cards carry themselves drew no line at all.
func TestASprintEstimatedThroughSubtasksBurnsTheFamily(t *testing.T) {
	s := build(t, sprint11(), afterTheSprint, time.UTC,
		family("PLAT-1", 40, []backend.Worklog{logged("2026-08-04", 10, 4)}),
	)
	if len(s.TimeDays) == 0 {
		t.Fatal("no hours burndown for a sprint estimated through its sub-tasks")
	}
	first := timeDay(t, s, "2026-08-03")
	if first.Scope != 40 {
		t.Errorf("3 Aug scope = %v, want the family's 40h", first.Scope)
	}
	if second := timeDay(t, s, "2026-08-04"); second.Remaining != 36 {
		t.Errorf("4 Aug remaining = %v, want the sub-task's four hours burned", second.Remaining)
	}
}

// A sprint holding a parent and its sub-tasks counts the family once:
// the children are cards in their own right, so the parent contributes
// what it carries itself, which is usually nothing.
func TestAParentAndItsChildrenInOneSprintCountOnce(t *testing.T) {
	parent := family("PLAT-1", 40, []backend.Worklog{logged("2026-08-04", 10, 4)})
	child := timed("PLAT-2", "In Progress", "3", hours(40), []backend.Worklog{logged("2026-08-04", 10, 4)})
	child.Issue.ParentKey = "PLAT-1"
	s := build(t, sprint11(), afterTheSprint, time.UTC, parent, child)

	first := timeDay(t, s, "2026-08-03")
	if first.Scope != 40 {
		t.Errorf("scope = %v, want the family counted once", first.Scope)
	}
	if second := timeDay(t, s, "2026-08-04"); second.Remaining != 36 {
		t.Errorf("remaining = %v, want the four hours burned once", second.Remaining)
	}
}
