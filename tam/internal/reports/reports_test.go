package reports_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/donerule"
	"agile-suite/tam/internal/reports"
)

// The sprint every test in this package reconstructs unless it says
// otherwise: Monday 3 August to Friday 14 August 2026, which is ten
// working days with two weekends inside it.
//
// Every timestamp here is written the way Jira writes one, with an offset
// carrying no colon. That is not decoration. time.RFC3339 rejects this
// shape outright, so a fixture written with a Z would pass against a
// parser that could never read a real instance's changelog, which is the
// mistake internal/sprintdate's own comment says this repo already made
// once.
const (
	sprintStart = "2026-08-03T09:00:00.000+0000"
	sprintEnd   = "2026-08-14T09:00:00.000+0000"
)

// afterTheSprint is the clock for a closed sprint: any moment past the
// end, since the walk stops at the sprint's own end well before it.
var afterTheSprint = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

func sprint11() backend.Sprint {
	return backend.Sprint{ID: 11, BoardID: 1, Name: "Sprint 11", State: "closed", StartDate: sprintStart, EndDate: sprintEnd}
}

// on is a changelog timestamp in Jira's own format, for a date written as
// 2026-08-05 and an hour of the day.
func on(date string, hour int) string {
	return fmt.Sprintf("%sT%02d:00:00.000+0000", date, hour)
}

// doneRule is the board's rule, built through donerule from a three column
// board, so these tests exercise the same package the sprint completion
// does rather than a second definition of finished written for the test.
func doneRule() func(string) bool {
	return donerule.Done([]backend.BoardColumn{
		{Name: "To Do", StatusIDs: []string{"1"}},
		{Name: "In Progress", StatusIDs: []string{"3"}},
		{Name: "Done", StatusIDs: []string{"10001"}},
	})
}

func points(v float64) *float64 { return &v }

// card builds one issue's history: the row as the cache holds it today,
// then the changes that got it there.
func card(key, status, statusID, sprintID string, estimate *float64, changes ...backend.Change) backend.IssueHistory {
	return backend.IssueHistory{
		Issue: backend.Issue{
			Key: key, Status: status, StatusID: statusID, SprintID: sprintID, StoryPoints: estimate,
		},
		Changes: changes,
	}
}

func build(t *testing.T, sprint backend.Sprint, now time.Time, loc *time.Location, issues ...backend.IssueHistory) reports.Series {
	t.Helper()
	s, err := reports.Build(sprint, doneRule(), issues, now, loc)
	if err != nil {
		t.Fatalf("build sprint %d: %v", sprint.ID, err)
	}
	return s
}

// day is the series' figures for one date, which is how a test names the
// day it means instead of counting positions in a slice.
func day(t *testing.T, s reports.Series, date string) reports.Day {
	t.Helper()
	for _, d := range s.Days {
		if d.Date == date {
			return d
		}
	}
	t.Fatalf("the series has no day %s; it runs %v", date, dates(s))
	return reports.Day{}
}

func dates(s reports.Series) []string {
	out := make([]string, 0, len(s.Days))
	for _, d := range s.Days {
		out = append(out, d.Date)
	}
	return out
}

func TestASprintWithNoDatesCannotBeReportedOn(t *testing.T) {
	sp := sprint11()
	sp.StartDate = ""
	if _, err := reports.Build(sp, doneRule(), nil, afterTheSprint, time.UTC); err == nil {
		t.Error("a sprint with no start date has nothing to reconstruct")
	}
	sp = sprint11()
	sp.EndDate = ""
	if _, err := reports.Build(sp, doneRule(), nil, afterTheSprint, time.UTC); err == nil {
		t.Error("a sprint with no end date has nothing to reconstruct")
	}
	sp = sprint11()
	sp.StartDate, sp.EndDate = sprintEnd, sprintStart
	if _, err := reports.Build(sp, doneRule(), nil, afterTheSprint, time.UTC); err == nil {
		t.Error("a sprint that ends before it starts has no days to walk")
	}
}

func TestABuildWithoutAZoneOrARuleForFinishedRefuses(t *testing.T) {
	if _, err := reports.Build(sprint11(), doneRule(), nil, afterTheSprint, nil); err == nil {
		t.Error("days cannot be bucketed without the zone to bucket them in")
	}
	if _, err := reports.Build(sprint11(), nil, nil, afterTheSprint, time.UTC); err == nil {
		t.Error("nothing can be burned down without a rule for what finished")
	}
}

func TestAChangelogTimestampTAMCannotReadFailsTheWholeBuild(t *testing.T) {
	c := card("PLAT-1", "To Do", "1", "11", points(3),
		backend.Change{At: "last Tuesday", Field: "status", From: "To Do", To: "In Progress"})
	_, err := reports.Build(sprint11(), doneRule(), []backend.IssueHistory{c}, afterTheSprint, time.UTC)
	if err == nil {
		t.Fatal("a report quietly missing one change is wrong without saying so, so the build has to fail")
	}
	if !strings.Contains(err.Error(), "PLAT-1") {
		t.Errorf("the failure has to name the issue it could not read; got %q", err)
	}
}

func TestPointsWinAsSoonAsOneCardCarriesAnEstimate(t *testing.T) {
	s := build(t, sprint11(), afterTheSprint, time.UTC,
		card("PLAT-1", "To Do", "1", "11", points(5)),
		card("PLAT-2", "To Do", "1", "11", nil),
	)
	if s.Unit != reports.UnitPoints || s.UnitReason != "" {
		t.Fatalf("one estimated card makes this a points sprint; got %q / %q", s.Unit, s.UnitReason)
	}
	if s.Committed != 5 {
		t.Errorf("the unestimated card contributes nothing in points mode; committed %v, want 5", s.Committed)
	}
}

func TestASprintWhereNothingIsEstimatedCountsCardsAndSaysWhich(t *testing.T) {
	// The estimate arrives a week after the sprint closed, so the rewind
	// takes it back off every day of the sprint, and the field's presence
	// is still in evidence on the row.
	late := card("PLAT-1", "Done", "10001", "11", points(5),
		backend.Change{At: on("2026-08-21", 9), Field: "storyPoints", From: "", To: "5"})
	s := build(t, sprint11(), afterTheSprint, time.UTC, late, card("PLAT-2", "To Do", "1", "11", nil))
	if s.Unit != reports.UnitCards || s.UnitReason != reports.ReasonNothingEstimated {
		t.Fatalf("nothing was estimated while the sprint ran, and the field plainly exists; got %q / %q", s.Unit, s.UnitReason)
	}
	if s.Committed != 2 {
		t.Errorf("in cards mode every card counts one; committed %v, want 2", s.Committed)
	}
}

func TestASprintWithNoSignOfTheFieldAtAllSaysSoSeparately(t *testing.T) {
	s := build(t, sprint11(), afterTheSprint, time.UTC,
		card("PLAT-1", "Done", "10001", "11", nil),
		card("PLAT-2", "To Do", "1", "11", nil,
			backend.Change{At: on("2026-08-05", 9), Field: "status", From: "To Do", To: "In Progress"},
			backend.Change{At: on("2026-08-06", 9), Field: "status", From: "In Progress", To: "To Do"}),
	)
	if s.Unit != reports.UnitCards || s.UnitReason != reports.ReasonNoPointsFieldSeen {
		t.Fatalf("nothing here mentions story points at all; got %q / %q", s.Unit, s.UnitReason)
	}
}

func TestAnIssueWhoseHistoryWasCutShortIsNamed(t *testing.T) {
	cut := card("PLAT-9", "To Do", "1", "11", points(1))
	cut.Truncated = true
	whole := card("PLAT-1", "To Do", "1", "11", points(1))
	s := build(t, sprint11(), afterTheSprint, time.UTC, whole, cut)
	if len(s.Truncated) != 1 || s.Truncated[0] != "PLAT-9" {
		t.Errorf("a report holding a partial history has to name it; got %v", s.Truncated)
	}
}
