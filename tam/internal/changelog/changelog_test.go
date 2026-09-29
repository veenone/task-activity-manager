package changelog

import (
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
)

func history(key string, changes ...backend.Change) backend.IssueHistory {
	return backend.IssueHistory{Issue: backend.Issue{Key: key}, Changes: changes}
}

// Jira's timestamps are not RFC 3339, which is the whole reason Parse reads
// them through sprintdate rather than time.Parse.
func TestParseReadsJiraTimestamps(t *testing.T) {
	got, err := Parse(history("TAM-1",
		backend.Change{At: "2026-03-04T09:00:00.000+0000", Field: FieldStatus, From: "To Do", To: "In Progress", FromID: "1", ToID: "3"},
	))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d changes, want 1", len(got))
	}
	if got[0].Field != FieldStatus || got[0].To != "In Progress" || got[0].ToID != "3" {
		t.Errorf("fields did not survive the read: %+v", got[0])
	}
	if got[0].At.IsZero() {
		t.Error("the timestamp was not read")
	}
}

// The order is not a promise any backend makes, and a change replayed on the
// wrong side of a boundary is a wrong report with nothing saying so.
func TestParseSortsOldestFirst(t *testing.T) {
	got, err := Parse(history("TAM-2",
		backend.Change{At: "2026-03-06T09:00:00.000+0000", Field: FieldStatus, To: "Done"},
		backend.Change{At: "2026-03-04T09:00:00.000+0000", Field: FieldStatus, To: "In Progress"},
	))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got[0].To != "In Progress" || got[1].To != "Done" {
		t.Errorf("not sorted oldest first: %q then %q", got[0].To, got[1].To)
	}
}

// A report missing one change is wrong without saying so, so an unreadable
// timestamp refuses rather than being skipped.
func TestParseRefusesAnUnreadableTimestamp(t *testing.T) {
	_, err := Parse(history("TAM-3", backend.Change{At: "not a date", Field: FieldStatus}))
	if err == nil {
		t.Fatal("want an error for an unreadable timestamp, got nil")
	}
}

func TestPoints(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		want  *float64
	}{
		{"blank is no estimate", "", nil},
		{"text is no estimate", "sizeable", nil},
		{"a whole number", "5", ptr(5)},
		{"a fraction", "2.5", ptr(2.5)},
		{"surrounded by spaces", "  8  ", ptr(8)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Points(tc.value)
			switch {
			case tc.want == nil && got != nil:
				t.Errorf("got %v, want no estimate", *got)
			case tc.want != nil && got == nil:
				t.Errorf("got no estimate, want %v", *tc.want)
			case tc.want != nil && *got != *tc.want:
				t.Errorf("got %v, want %v", *got, *tc.want)
			}
		})
	}
}

// Civil returns a UTC midnight so that adding a day can never land on the
// same date twice or skip one, which stepping a local midnight through a
// spring forward can.
func TestCivilIsUTCMidnightOfTheLocalDate(t *testing.T) {
	loc, err := time.LoadLocation("Australia/Sydney")
	if err != nil {
		t.Skipf("no tzdata for the test: %v", err)
	}
	// 13:00 UTC is already the next day in Sydney.
	got := Civil(time.Date(2026, 3, 4, 13, 0, 0, 0, time.UTC), loc)
	want := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %s, want %s", got, want)
	}
	if got.Location() != time.UTC {
		t.Errorf("got location %s, want UTC", got.Location())
	}
}

func TestDaysBetweenIsSignedAndWholeDays(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 3, d, 0, 0, 0, 0, time.UTC) }
	if got := DaysBetween(day(2), day(9)); got != 7 {
		t.Errorf("forwards: got %d, want 7", got)
	}
	if got := DaysBetween(day(9), day(2)); got != -7 {
		t.Errorf("backwards: got %d, want -7", got)
	}
	if got := DaysBetween(day(4), day(4)); got != 0 {
		t.Errorf("same day: got %d, want 0", got)
	}
}

// Monday to Friday, inclusive of both ends, and not configurable.
func TestWorkingDays(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 3, d, 0, 0, 0, 0, time.UTC) }
	// 2026-03-02 is a Monday.
	if got := WorkingDays(day(2), day(6)); got != 5 {
		t.Errorf("one full week: got %d, want 5", got)
	}
	if got := WorkingDays(day(2), day(8)); got != 5 {
		t.Errorf("a week plus its weekend: got %d, want 5", got)
	}
	if got := WorkingDays(day(7), day(8)); got != 0 {
		t.Errorf("a weekend alone: got %d, want 0", got)
	}
	if got := WorkingDays(day(2), day(2)); got != 1 {
		t.Errorf("one working day: got %d, want 1", got)
	}
}

func TestIsWorkingDay(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 3, d, 0, 0, 0, 0, time.UTC) }
	if !IsWorkingDay(day(6)) {
		t.Error("Friday should be a working day")
	}
	if IsWorkingDay(day(7)) || IsWorkingDay(day(8)) {
		t.Error("the weekend should not be")
	}
}

func ptr(f float64) *float64 { return &f }
