package reports_test

import (
	"reflect"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/donerule"
	"agile-suite/tam/internal/reports"
)

func TestAnIssueChangedAfterTheSprintClosedLeavesTheSeriesUnmoved(t *testing.T) {
	// The card as it stands today: closed, re-estimated, and moved on to
	// the next sprint, all of it a week after this sprint ended. A
	// forward replay seeded from those values would open the sprint with
	// eight points of already finished work that was not in the sprint at
	// all.
	moved := card("PLAT-1", "Done", "10001", "12", points(8),
		backend.Change{At: on("2026-08-20", 9), Field: "sprint", From: "11", To: "12"},
		backend.Change{At: on("2026-08-21", 9), Field: "status", From: "In Progress", To: "Done"},
		backend.Change{At: on("2026-08-22", 9), Field: "storyPoints", From: "3", To: "8"},
	)
	// The same card as it stood during the sprint, with nothing after it.
	untouched := card("PLAT-1", "In Progress", "3", "11", points(3))

	after := build(t, sprint11(), afterTheSprint, time.UTC, moved)
	before := build(t, sprint11(), afterTheSprint, time.UTC, untouched)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("what happened after the sprint closed has to leave it alone:\n after  %+v\n before %+v", after, before)
	}
	if after.Committed != 3 || after.Completed != 0 || after.CarriedOver != 3 {
		t.Errorf("the sprint committed three points and finished none of them; got %+v", after)
	}
}

func TestDayOneIsTheSprintsOwnLocalDate(t *testing.T) {
	// Nine in the morning in Sydney, which is the previous evening in
	// UTC. Bucketing in UTC would give this team a day one that began the
	// afternoon before their sprint did.
	sydney := time.FixedZone("test/+1000", 10*60*60)
	sp := sprint11()
	sp.StartDate = "2026-08-03T09:00:00.000+1000"
	sp.EndDate = "2026-08-14T17:00:00.000+1000"

	local := build(t, sp, afterTheSprint, sydney, card("PLAT-1", "To Do", "1", "11", points(1)))
	if local.Days[0].Date != "2026-08-03" {
		t.Errorf("day one is the sprint's own local date; got %s out of %v", local.Days[0].Date, dates(local))
	}
	utc := build(t, sp, afterTheSprint, time.UTC, card("PLAT-1", "To Do", "1", "11", points(1)))
	if utc.Days[0].Date != "2026-08-02" {
		t.Errorf("the same instant read in UTC really does fall on the day before, which is the whole point; got %s", utc.Days[0].Date)
	}
}

func TestACardMovingFromTwoSprintsToOneLeavesOnlyTheOneItDropped(t *testing.T) {
	// A rollover: the card sits in 12 and 13 at once, then 12 lets go of
	// it. Read as a toggle rather than as a set, that change takes the
	// card out of both.
	rollover := card("PLAT-1", "To Do", "1", "13", points(5),
		backend.Change{At: on("2026-08-05", 9), Field: "sprint", From: "12, 13", To: "13"})

	twelve := backend.Sprint{ID: 12, Name: "Sprint 12", State: "closed", StartDate: sprintStart, EndDate: sprintEnd}
	thirteen := backend.Sprint{ID: 13, Name: "Sprint 13", State: "closed", StartDate: sprintStart, EndDate: sprintEnd}

	left := build(t, twelve, afterTheSprint, time.UTC, rollover)
	if left.Committed != 5 || left.Removed != 5 {
		t.Errorf("sprint 12 started with the card and lost it; got committed %v removed %v", left.Committed, left.Removed)
	}
	if d := day(t, left, "2026-08-05"); d.Scope != 0 {
		t.Errorf("sprint 12 holds nothing after the card leaves; scope %v", d.Scope)
	}

	stayed := build(t, thirteen, afterTheSprint, time.UTC, rollover)
	if stayed.Committed != 5 || stayed.Removed != 0 {
		t.Errorf("sprint 13 never let go of the card; got committed %v removed %v", stayed.Committed, stayed.Removed)
	}
	if d := day(t, stayed, "2026-08-05"); d.Scope != 5 {
		t.Errorf("sprint 13 still holds the card after the change; scope %v", d.Scope)
	}
}

func TestAnEstimateChangedInsideTheSprintTakesEffectThatDay(t *testing.T) {
	s := build(t, sprint11(), afterTheSprint, time.UTC,
		card("PLAT-1", "To Do", "1", "11", points(8),
			backend.Change{At: on("2026-08-05", 14), Field: "storyPoints", From: "3", To: "8"}),
	)
	if s.Committed != 3 {
		t.Errorf("the sprint was committed at the estimate the card carried then; got %v", s.Committed)
	}
	if d := day(t, s, "2026-08-04"); d.Scope != 3 {
		t.Errorf("the day before the re-estimate still reads three; got %v", d.Scope)
	}
	if d := day(t, s, "2026-08-05"); d.Scope != 8 {
		t.Errorf("the re-estimate lands on the day it happened; got %v", d.Scope)
	}
}

func TestAnEstimateChangedWhileTheCardWasOutsideTakesEffectWhenItReturns(t *testing.T) {
	s := build(t, sprint11(), afterTheSprint, time.UTC,
		card("PLAT-1", "To Do", "1", "11", points(8),
			backend.Change{At: on("2026-08-04", 9), Field: "sprint", From: "Sprint 11", To: ""},
			backend.Change{At: on("2026-08-06", 9), Field: "storyPoints", From: "3", To: "8"},
			backend.Change{At: on("2026-08-10", 9), Field: "sprint", From: "", To: "Sprint 11"}),
	)
	if s.Committed != 3 || s.Removed != 3 {
		t.Errorf("the sprint started with three points and lost them; got committed %v removed %v", s.Committed, s.Removed)
	}
	if d := day(t, s, "2026-08-06"); d.Scope != 0 {
		t.Errorf("a re-estimate outside the sprint moves nothing inside it; scope %v", d.Scope)
	}
	if s.Added != 8 {
		t.Errorf("the card came back carrying its new estimate; added %v, want 8", s.Added)
	}
	if d := day(t, s, "2026-08-10"); d.Scope != 8 {
		t.Errorf("the day it returned holds the new estimate; scope %v", d.Scope)
	}
}

func TestACardFinishedBeforeTheSprintStartedIsCommittedWithNothingToBurn(t *testing.T) {
	s := build(t, sprint11(), afterTheSprint, time.UTC,
		card("PLAT-1", "Done", "10001", "11", points(8),
			backend.Change{At: on("2026-08-01", 9), Field: "status", From: "In Progress", To: "Done"}),
		card("PLAT-2", "To Do", "1", "11", points(2)),
	)
	if s.Committed != 10 {
		t.Errorf("work already finished was still part of what the sprint took on; committed %v, want 10", s.Committed)
	}
	if d := day(t, s, "2026-08-03"); d.Remaining != 2 || d.Completed != 8 {
		t.Errorf("there is nothing left to burn on the finished card; day one %+v", d)
	}
	if s.Completed != 8 || s.CarriedOver != 2 {
		t.Errorf("eight points finished and two carried over; got %+v", s)
	}
}

func TestDaysBeforeTheEarliestEvidenceRepeatDayOnesValue(t *testing.T) {
	// A start date dragged back after the sprint had already begun: the
	// changelog knows nothing at all about the first week, so the walk
	// has nothing to move and every one of those days reads the same.
	s := build(t, sprint11(), afterTheSprint, time.UTC,
		card("PLAT-1", "To Do", "1", "11", points(3)),
		card("PLAT-2", "To Do", "1", "11", points(5),
			backend.Change{At: on("2026-08-10", 9), Field: "sprint", From: "", To: "Sprint 11"}),
	)
	if s.Committed != 3 {
		t.Errorf("committed is measured at the start date the sprint carries now; got %v", s.Committed)
	}
	for _, date := range []string{"2026-08-03", "2026-08-05", "2026-08-07", "2026-08-09"} {
		if d := day(t, s, date); d.Scope != 3 {
			t.Errorf("%s has no evidence of its own and reads day one's value; scope %v", date, d.Scope)
		}
	}
	if d := day(t, s, "2026-08-10"); d.Scope != 8 {
		t.Errorf("the first day the changelog knows about is the first day that moves; scope %v", d.Scope)
	}
	if s.Added != 5 {
		t.Errorf("the card that arrived mid-sprint is the sprint's added scope; got %v", s.Added)
	}
}

func TestACardThatLeftAndCameBackIsChargedOncePerDirection(t *testing.T) {
	s := build(t, sprint11(), afterTheSprint, time.UTC,
		card("PLAT-1", "To Do", "1", "11", points(2),
			backend.Change{At: on("2026-08-04", 9), Field: "sprint", From: "Sprint 11", To: ""},
			backend.Change{At: on("2026-08-06", 9), Field: "sprint", From: "", To: "Sprint 11"},
			backend.Change{At: on("2026-08-10", 9), Field: "sprint", From: "Sprint 11", To: ""},
			backend.Change{At: on("2026-08-12", 9), Field: "sprint", From: "", To: "Sprint 11"}),
	)
	if s.Added != 2 || s.Removed != 2 {
		t.Errorf("two round trips are one departure and one arrival, not two of each; added %v removed %v", s.Added, s.Removed)
	}
	if s.CarriedOver != 2 {
		t.Errorf("the card ended the sprint inside it and unfinished; carried over %v", s.CarriedOver)
	}
}

func TestTheGuideLineRunsDownOverWorkingDaysOnly(t *testing.T) {
	s := build(t, sprint11(), afterTheSprint, time.UTC, card("PLAT-1", "To Do", "1", "11", points(10)))
	want := map[string]float64{
		"2026-08-03": 9, // Monday, one of ten working days spent
		"2026-08-07": 5, // Friday
		"2026-08-08": 5, // Saturday, the line holds
		"2026-08-09": 5, // Sunday
		"2026-08-10": 4, // Monday again
		"2026-08-14": 0, // the last working day
	}
	for date, ideal := range want {
		if d := day(t, s, date); d.Ideal != ideal {
			t.Errorf("%s: guide %v, want %v", date, d.Ideal, ideal)
		}
	}
	if len(s.Days) != 12 {
		t.Errorf("the sprint runs twelve calendar days; got %d (%v)", len(s.Days), dates(s))
	}
}

func TestALiveSprintStopsAtTodayAndKeepsItsGuideAimedAtTheEnd(t *testing.T) {
	sp := sprint11()
	sp.State = "active"
	now := time.Date(2026, 8, 6, 15, 0, 0, 0, time.UTC)
	s := build(t, sp, now, time.UTC, card("PLAT-1", "To Do", "1", "11", points(10)))
	if last := s.Days[len(s.Days)-1].Date; last != "2026-08-06" {
		t.Errorf("a sprint still running is not drawn past today; last day %s", last)
	}
	if d := day(t, s, "2026-08-06"); d.Ideal != 6 {
		t.Errorf("the guide is still aimed at the day the sprint is due to finish; got %v, want 6", d.Ideal)
	}
}

func TestAStatusNoCardStillSitsInReadsAsUnfinished(t *testing.T) {
	// PLAT-1 passed through Done and came back out, and it is the only
	// card in the sprint, so nothing here carries the id that would say
	// what "Done" means on this board. The walk keeps its work on the
	// burndown for those days rather than retiring it on a guess, which
	// is the limit doneNames documents.
	s := build(t, sprint11(), afterTheSprint, time.UTC,
		card("PLAT-1", "In Progress", "3", "11", points(4),
			backend.Change{At: on("2026-08-05", 9), Field: "status", From: "In Progress", To: "Done"},
			backend.Change{At: on("2026-08-07", 9), Field: "status", From: "Done", To: "In Progress"}),
	)
	if d := day(t, s, "2026-08-06"); d.Completed != 0 || d.Remaining != 4 {
		t.Errorf("an untranslatable status is not counted as finished; %+v", d)
	}
	if s.CarriedOver != 4 {
		t.Errorf("the card ended the sprint unfinished either way; carried over %v", s.CarriedOver)
	}
}

func TestACardDoneInsideTheSprintAndArchivedAfterItClosedReportsCompletedNotCarried(t *testing.T) {
	// The reviewer's own case. The card moves to a done status inside the
	// sprint and on to an archived one a fortnight after the sprint closed,
	// and nothing currently sits in the done status any more, so the name
	// based fallback alone has nothing to translate it with. The changelog
	// carries the status id on both sides of the change, which is what
	// lets the card be read as done without needing another card to still
	// be sitting there.
	done := donerule.Done([]backend.BoardColumn{
		{Name: "To Do", StatusIDs: []string{"1"}},
		{Name: "In Progress", StatusIDs: []string{"3"}},
		{Name: "Done", StatusIDs: []string{"10001", "10002"}},
	})
	c := card("PLAT-1", "Archived", "20000", "11", points(4),
		backend.Change{At: on("2026-08-06", 9), Field: "status", From: "To Do", To: "Done", FromID: "1", ToID: "10001"},
		backend.Change{At: on("2026-08-25", 9), Field: "status", From: "Done", To: "Archived", FromID: "10001", ToID: "20000"},
	)
	s, err := reports.Build(sprint11(), done, []backend.IssueHistory{c}, afterTheSprint, time.UTC)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if s.Committed != 4 || s.Completed != 4 || s.CarriedOver != 0 {
		t.Errorf("a card done inside the sprint and archived afterwards still finished it; got %+v", s)
	}
}

func TestACardWhoseCachedSprintFieldNamesALaterSprintIsStillCommittedHere(t *testing.T) {
	// parseSprint keeps only the last entry of Jira's Sprint array, so a
	// card sitting in both sprint 12 and sprint 13 at once, ordinary during
	// a rollover, has a cached row that names 13 even while Jira still
	// counts it in 12. The card came back from a "sprint = 12" search, so
	// it is in sprint 12 by construction; nothing here ever changes that
	// membership inside the window, which is what makes the card's row
	// value irrelevant to whether it should count.
	twelve := backend.Sprint{ID: 12, Name: "Sprint 12", State: "closed", StartDate: sprintStart, EndDate: sprintEnd}
	s := build(t, twelve, afterTheSprint, time.UTC, card("PLAT-1", "To Do", "1", "13", points(5)))
	if s.Committed != 5 {
		t.Errorf("a card whose row names a later sprint is still committed to this one; got %v", s.Committed)
	}
}

func TestAChangeDatedExactlyAtTheSprintsStartIsNotUndone(t *testing.T) {
	// A single change landing on the sprint's opening instant, moving the
	// card into a done status. rewound only undoes a change dated strictly
	// after start; if that were "at or after" instead, this one would be
	// undone too and the card would open the sprint in In Progress rather
	// than Done, reading day one as unfinished instead of finished. Only
	// one card is in the sprint, so nothing besides this one change
	// decides which it is. Until now the only fixture that ever put a
	// change on this exact boundary was the demo dataset's, by accident of
	// which date its author picked, so nothing here failed on purpose if
	// the boundary moved.
	done := donerule.Done([]backend.BoardColumn{
		{Name: "To Do", StatusIDs: []string{"1"}},
		{Name: "In Progress", StatusIDs: []string{"3"}},
		{Name: "Done", StatusIDs: []string{"10001"}},
	})
	c := card("PLAT-1", "Done", "10001", "11", points(4),
		backend.Change{At: sprintStart, Field: "status", From: "In Progress", To: "Done", FromID: "3", ToID: "10001"},
	)
	s, err := reports.Build(sprint11(), done, []backend.IssueHistory{c}, afterTheSprint, time.UTC)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if d := day(t, s, "2026-08-03"); d.Completed != 4 || d.Remaining != 0 {
		t.Errorf("day one opens already in the change's \"to\" state, Done, not undone back to In Progress; %+v", d)
	}
}

func TestALateSecondLocalMorningChangeInAPositiveOffsetZoneLandsOnDayTwo(t *testing.T) {
	// Every other test in this file runs in UTC, where local midnight and
	// UTC midnight are the same instant, so a walker that bucketed in UTC
	// regardless of loc would still pass every one of them. This is the one
	// test that cannot: 09:00 on the sprint's second local morning is
	// 23:00 the previous day in UTC, so bucketing in UTC would apply this
	// change a full day early.
	sydney := time.FixedZone("test/+1000", 10*60*60)
	sp := sprint11()
	sp.StartDate = "2026-08-03T09:00:00.000+1000"
	sp.EndDate = "2026-08-14T17:00:00.000+1000"

	done := donerule.Done([]backend.BoardColumn{
		{Name: "To Do", StatusIDs: []string{"1"}},
		{Name: "Done", StatusIDs: []string{"10001"}},
	})
	c := card("PLAT-1", "Done", "10001", "11", points(4),
		backend.Change{At: "2026-08-04T09:00:00.000+1000", Field: "status", From: "To Do", To: "Done", FromID: "1", ToID: "10001"},
	)
	s, err := reports.Build(sp, done, []backend.IssueHistory{c}, afterTheSprint, sydney)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if d := day(t, s, "2026-08-03"); d.Completed != 0 {
		t.Errorf("day one, before the change, is not yet finished; completed %v", d.Completed)
	}
	if d := day(t, s, "2026-08-04"); d.Completed != 4 {
		t.Errorf("the change happened on day two's local morning and has to land there; completed %v", d.Completed)
	}
}
