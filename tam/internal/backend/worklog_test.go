package backend_test

import (
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
)

// TestNewWorklogDraftKeepsTheLoggersOwnDay is the trap this feature exists
// around. Jira dates a worklog by the offset on started, so an entry logged
// at 01:00 in a +07:00 zone and sent as the same instant in UTC arrives
// stamped 18:00 the day before, and every Jira report that groups by day
// files the work under the wrong day.
func TestNewWorklogDraftKeepsTheLoggersOwnDay(t *testing.T) {
	jakarta := time.FixedZone("WIB", 7*60*60)
	at := time.Date(2026, 9, 29, 1, 0, 0, 0, jakarta)

	d, err := backend.NewWorklogDraft("2h 30m", "Pairing", at)
	if err != nil {
		t.Fatalf("new draft: %v", err)
	}
	if d.Started != "2026-09-29T01:00:00.000+0700" {
		t.Errorf("started = %q, want the local clock and the +0700 offset that dates it 29 Sep", d.Started)
	}
	// The same instant as UTC is 2026-09-28T18:00, so a stamp carrying Z or
	// +0000 has moved the work to the day before.
	if strings.Contains(d.Started, "Z") || strings.Contains(d.Started, "+0000") {
		t.Errorf("started = %q, want the logger's own offset rather than UTC", d.Started)
	}
	if d.TimeSpent != "2h 30m" || d.Comment != "Pairing" {
		t.Errorf("draft = %+v, want the typed duration and comment carried as typed", d)
	}
	if d.Seconds != 9000 {
		t.Errorf("seconds = %d, want 9000 for 2h 30m", d.Seconds)
	}
}

// TestNewWorklogDraftKeepsAWesternOffsetToo is the same check the other way
// round: 23:00 in a -05:00 zone is the next day in UTC.
func TestNewWorklogDraftKeepsAWesternOffsetToo(t *testing.T) {
	newYork := time.FixedZone("EST", -5*60*60)
	d, err := backend.NewWorklogDraft("1h", "", time.Date(2026, 9, 29, 23, 0, 0, 0, newYork))
	if err != nil {
		t.Fatalf("new draft: %v", err)
	}
	if d.Started != "2026-09-29T23:00:00.000-0500" {
		t.Errorf("started = %q, want 29 Sep at -0500 rather than 30 Sep in UTC", d.Started)
	}
}

func TestParseWorkSecondsReadsJirasOwnPhrasings(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"2h 30m", 9000},
		{"90m", 5400},
		{"1d", 8 * 3600},
		{"1w", 5 * 8 * 3600},
		{"1.5h", 5400},
		{"2H 30M", 9000},
		{"45", 2700},
		{"  3h  ", 3 * 3600},
		{"1d 2h 30m", 8*3600 + 9000},
	}
	for _, c := range cases {
		got, err := backend.ParseWorkSeconds(c.in)
		if err != nil {
			t.Errorf("ParseWorkSeconds(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseWorkSeconds(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// TestParseWorkSecondsRefusesWhatJiraWouldRefuse: Jira answers a 400 for
// these, and a 400 arrives at Commit, hours after the typing. The reason has
// to name the value and say what a good one looks like.
func TestParseWorkSecondsRefusesWhatJiraWouldRefuse(t *testing.T) {
	for _, in := range []string{"", "   ", "banana", "2 hrs", "0", "0m", "0h 0m", "-2h", "2x", "h", "2h30", "2..5h"} {
		got, err := backend.ParseWorkSeconds(in)
		if err == nil {
			t.Errorf("ParseWorkSeconds(%q) = %d, want a refusal", in, got)
			continue
		}
		if !strings.Contains(err.Error(), "2h 30m") {
			t.Errorf("ParseWorkSeconds(%q) error = %q, want it to show a duration that works", in, err)
		}
	}
}

// TestNewWorklogDraftRefusesAZeroDuration: the refusal is the same one
// ParseWorkSeconds gives, so the form and the journal cannot disagree about
// what a duration is.
func TestNewWorklogDraftRefusesAZeroDuration(t *testing.T) {
	_, err := backend.NewWorklogDraft("0m", "Nothing", time.Now())
	if err == nil {
		t.Fatal("a worklog of zero was accepted")
	}
	if _, direct := backend.ParseWorkSeconds("0m"); direct == nil || direct.Error() != err.Error() {
		t.Errorf("draft error = %q, want the same sentence ParseWorkSeconds gives: %v", err, direct)
	}
}
