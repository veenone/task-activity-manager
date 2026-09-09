package sprints_test

import (
	"testing"
	"time"

	"agile-suite/tam/internal/sprints"
)

// TestSuggestTakesTheBoardsOwnLength is the dialog's default end date: the
// board's own cadence, measured from its closed sprints, rather than a
// number this app picked.
func TestSuggestTakesTheBoardsOwnLength(t *testing.T) {
	now := time.Date(2026, 9, 9, 11, 30, 0, 0, time.Local)

	got := sprints.Suggest(now, 10, "Sprint 12")
	if got.Start != "2026-09-09" || got.End != "2026-09-19" {
		t.Errorf("dates = %q to %q, want today and today plus ten days", got.Start, got.End)
	}
	if got.Length != 10 || !got.FromHistory {
		t.Errorf("suggestion = %+v, want the board's own ten days", got)
	}
	if got.Name != "Sprint 13" {
		t.Errorf("name = %q, want the number after the board's last sprint", got.Name)
	}
}

// TestSuggestFallsBackToAFortnightAndSaysSo is what a board with no closed
// sprints gets. FromHistory is what lets the dialog focus the end date
// instead of letting a number nobody measured pass unread.
func TestSuggestFallsBackToAFortnightAndSaysSo(t *testing.T) {
	now := time.Date(2026, 9, 9, 11, 30, 0, 0, time.Local)

	got := sprints.Suggest(now, 0, "")
	if got.Length != sprints.DefaultLength || got.FromHistory {
		t.Errorf("suggestion = %+v, want a fortnight marked as invented", got)
	}
	if got.End != "2026-09-23" {
		t.Errorf("end = %q, want a fortnight after today", got.End)
	}
	if got.Name != "" {
		t.Errorf("name = %q, want nothing suggested when there is no sprint to follow", got.Name)
	}
}

// TestNextNameFollowsTheBoardsOwnNumbering covers what teams actually call
// their sprints, including the padded numbers that would otherwise lose
// their width, and the names that suggest nothing rather than a duplicate of
// the sprint before.
func TestNextNameFollowsTheBoardsOwnNumbering(t *testing.T) {
	for _, tc := range []struct{ last, want string }{
		{"Sprint 12", "Sprint 13"},
		{"Sprint 09", "Sprint 10"},
		{"Sprint 9", "Sprint 10"},
		{"PLAT 2026 Sprint 3 ", "PLAT 2026 Sprint 4"},
		{"Hardening", ""},
		{"", ""},
	} {
		if got := sprints.NextName(tc.last); got != tc.want {
			t.Errorf("NextName(%q) = %q, want %q", tc.last, got, tc.want)
		}
	}
}
