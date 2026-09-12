package reports_test

import (
	"errors"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/reports"
)

func TestClosedSprintUsesActualCompletion(t *testing.T) {
	for _, tc := range []struct {
		name, completed, day, changed string
		want                          float64
	}{
		{"late", on("2026-08-17", 12), "2026-08-17", "2026-08-16", 5},
		{"early", on("2026-08-10", 12), "2026-08-10", "2026-08-12", 0},
		{"missing falls back", "", "2026-08-14", "2026-08-16", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sp := sprint11()
			sp.CompleteDate = tc.completed
			issue := card("P-1", "Done", "10001", "11", points(5), backend.Change{
				At: on(tc.changed, 12), Field: "status", From: "In Progress", To: "Done", FromID: "3", ToID: "10001",
			})
			s := build(t, sp, afterTheSprint, time.UTC, issue)
			if s.Completed != tc.want || s.CarriedOver != 5-tc.want {
				t.Fatalf("completed/carried = %v/%v", s.Completed, s.CarriedOver)
			}
			if got := s.Days[len(s.Days)-1].Date; got != tc.day {
				t.Fatalf("last day = %s, want %s", got, tc.day)
			}
		})
	}
}

func TestInvalidActualCompletionIsNotSilentlyReplaced(t *testing.T) {
	for _, date := range []string{"invalid", on("2026-08-01", 12)} {
		sp := sprint11()
		sp.CompleteDate = date
		if _, err := reports.Build(sp, doneRule(), nil, afterTheSprint, time.UTC); !errors.Is(err, reports.ErrNoDates) {
			t.Fatalf("date %q: error = %v", date, err)
		}
	}
}

func TestVelocityUsesActualCompletionOrderAndDates(t *testing.T) {
	a, b := sprint11(), sprint11()
	a.ID, b.ID = 1, 2
	a.CompleteDate = on("2026-08-20", 12)
	b.CompleteDate = on("2026-08-18", 12)
	a.EndDate = ""
	got := reports.VelocitySprints([]backend.Sprint{a, b})
	if len(got) != 2 || got[0].ID != 2 || got[1].ID != 1 {
		t.Fatalf("sprints = %+v", got)
	}
	build(t, a, afterTheSprint, time.UTC)
}

func TestActiveSprintIgnoresCompletionDate(t *testing.T) {
	sp := sprint11()
	sp.State, sp.CompleteDate = "active", "invalid"
	s := build(t, sp, afterTheSprint, time.UTC)
	if got := s.Days[len(s.Days)-1].Date; got != "2026-09-01" {
		t.Fatalf("last day = %s", got)
	}
}

func TestOverdueActiveSprintIncludesWorkThroughNow(t *testing.T) {
	sp := sprint11()
	sp.State = "active"
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	issue := card("P-1", "In Progress", "3", "11", points(5),
		backend.Change{At: on("2026-08-17", 10), Field: "status", From: "In Progress", To: "Done", FromID: "3", ToID: "10001"},
		backend.Change{At: on("2026-08-18", 15), Field: "status", From: "Done", To: "In Progress", FromID: "10001", ToID: "3"},
	)
	s := build(t, sp, now, time.UTC, issue)
	if s.Completed != 5 || s.CarriedOver != 0 {
		t.Fatalf("completed/remaining = %v/%v", s.Completed, s.CarriedOver)
	}
	last := s.Days[len(s.Days)-1]
	if last.Date != "2026-08-18" || last.Ideal != 0 {
		t.Fatalf("last day = %+v", last)
	}
}
