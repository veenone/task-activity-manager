package reports_test

import (
	"context"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	demobackend "agile-suite/tam/internal/backend/demo"
	"agile-suite/tam/internal/donerule"
	"agile-suite/tam/internal/reports"
)

// The demo dataset is the one fixture in this package nobody wrote for
// this package, and that is the point of running against it. Its curated
// changelog names sprints the way a real instance does, by name rather
// than by id, so a membership test that only matched ids would find no
// card ever in Sprint 11 and draw a flat line at zero here while every
// hand written fixture above still passed.
//
// It is also the offline walk-through the design asks for: this is what a
// user opening Reports on the demo profile sees.
func TestTheDemoProfilesOneClosedSprintReconstructs(t *testing.T) {
	ctx := context.Background()
	b := demobackend.New("PLAT")

	boards, err := b.Boards(ctx, "PLAT")
	if err != nil {
		t.Fatalf("demo boards: %v", err)
	}
	var scrum backend.Board
	for _, bd := range boards {
		if bd.Type == "scrum" {
			scrum = bd
			break
		}
	}
	if scrum.ID == 0 {
		t.Fatal("the demo profile has a scrum board and this needs it")
	}
	cols, err := b.BoardColumns(ctx, scrum.ID)
	if err != nil {
		t.Fatalf("demo columns: %v", err)
	}
	done := donerule.Done(cols)
	if done == nil {
		t.Fatal("the demo board's last column collects statuses, so it has a rule")
	}

	sprints, err := b.BoardSprints(ctx, scrum.ID)
	if err != nil {
		t.Fatalf("demo sprints: %v", err)
	}
	var closed backend.Sprint
	for _, sp := range sprints {
		if sp.ID == 11 {
			closed = sp
		}
	}
	if closed.ID != 11 {
		t.Fatal("Sprint 11 is the demo's one closed sprint")
	}

	issues, _, err := b.SearchIssuesWithHistory(ctx, "sprint = 11", 0, 50)
	if err != nil {
		t.Fatalf("demo history: %v", err)
	}
	for _, h := range issues {
		if h.Issue.Key == "PLAT-398" {
			t.Fatal("PLAT-398 left Sprint 11 for good, so the search cannot see it; that blind spot is what the report has to report around")
		}
	}

	s, err := reports.Build(closed, done, issues, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), time.UTC)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if s.Unit != reports.UnitPoints {
		t.Errorf("the demo's cards are estimated; unit %q", s.Unit)
	}
	if len(s.Days) == 0 || s.Days[0].Date != "2026-08-04" || s.Days[len(s.Days)-1].Date != "2026-08-18" {
		t.Fatalf("the series runs the sprint's own local days; got %v", dates(s))
	}
	if s.Days[0].Scope != s.Committed {
		t.Errorf("nothing in this sprint's history happens after its opening instant on day one, so day one's scope is what was committed; %v against %v", s.Days[0].Scope, s.Committed)
	}

	// Only the four curated cards have a changelog, and the figures below
	// are theirs. The rest of the sprint is dataset filler with no
	// history at all, which is why this reads the movement between days
	// rather than the totals: the filler sits in every total unchanged
	// and would only pin this test to a number nobody chose on purpose.
	//
	// PLAT-401 arrives on the 5th carrying the two points it had then,
	// PLAT-347 on the 6th with one, PLAT-385 leaves on the 8th and comes
	// back on the 11th, and PLAT-401 is re-estimated from two to three on
	// the 9th.
	for _, step := range []struct {
		date  string
		moved float64
	}{
		{"2026-08-05", 2},
		{"2026-08-06", 1},
		{"2026-08-07", 0},
		{"2026-08-08", -2},
		{"2026-08-09", 1},
		{"2026-08-11", 2},
	} {
		before := day(t, s, dayBefore(t, s, step.date)).Scope
		if got := day(t, s, step.date).Scope - before; got != step.moved {
			t.Errorf("%s: scope moved by %v, want %v", step.date, got, step.moved)
		}
	}
	// PLAT-385 finishes on the 15th, and it is the only card that changes
	// status inside the sprint.
	if got := day(t, s, "2026-08-15").Completed - day(t, s, "2026-08-14").Completed; got != 2 {
		t.Errorf("PLAT-385 finished on the 15th; completed moved by %v, want 2", got)
	}
	if s.Added != 5 {
		t.Errorf("PLAT-347's one point, PLAT-401's two, and PLAT-385's two on its return; added %v, want 5", s.Added)
	}
	if s.Removed != 2 {
		t.Errorf("PLAT-385 is the only card the search can see leaving, and only because it came back; removed %v, want 2", s.Removed)
	}
}

// dayBefore is the date of the day in front of this one in the series, so
// a test can read a movement rather than a total.
func dayBefore(t *testing.T, s reports.Series, date string) string {
	t.Helper()
	for i, d := range s.Days {
		if d.Date == date {
			if i == 0 {
				t.Fatalf("%s is day one and has no day before it", date)
			}
			return s.Days[i-1].Date
		}
	}
	t.Fatalf("the series has no day %s; it runs %v", date, dates(s))
	return ""
}
