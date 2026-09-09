package sprintdate_test

import (
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/sprintdate"
)

// TestParseReadsEveryShapeASprintDateArrivesIn covers the three the app
// actually meets: Jira's Agile datetime, which time.RFC3339 cannot read
// because its offset has no colon, the RFC 3339 the demo dataset carries,
// and the bare date a date input produces.
func TestParseReadsEveryShapeASprintDateArrivesIn(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  time.Time
	}{
		{"2026-09-09T09:00:00.000+0000", time.Date(2026, 9, 9, 9, 0, 0, 0, time.UTC)},
		{"2026-08-04T09:00:00Z", time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)},
		{"2026-09-09", time.Date(2026, 9, 9, 9, 0, 0, 0, time.Local)},
	} {
		got, err := sprintdate.Parse(tc.value)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.value, err)
		}
		if !got.Equal(tc.want) {
			t.Errorf("parse %q = %s, want %s", tc.value, got, tc.want)
		}
	}
}

// TestParseRefusesWhatItCannotRead is the whole point of parsing rather than
// concatenating: a value that is not a date is refused here, where the
// message can name it, instead of reaching Jira as a body it rejects.
func TestParseRefusesWhatItCannotRead(t *testing.T) {
	for _, value := range []string{"", "   ", "next tuesday", "09/09/2026", "2026-13-01"} {
		if _, err := sprintdate.Parse(value); err == nil {
			t.Errorf("parse %q = no error, want a refusal", value)
		}
	}
	_, err := sprintdate.Parse("next tuesday")
	if err == nil || !strings.Contains(err.Error(), "next tuesday") {
		t.Errorf("err = %v, want it to quote the value it could not read", err)
	}
}

// TestFormatRoundTripsThroughTheAgileShape proves the format Parse reads
// back is the one Format writes, offset and all, so a date that came from
// Jira and goes back to it is unchanged by the trip.
func TestFormatRoundTripsThroughTheAgileShape(t *testing.T) {
	const value = "2026-09-09T09:00:00.000+0000"
	parsed, err := sprintdate.Parse(value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := sprintdate.Format(parsed.UTC()); got != value {
		t.Errorf("format = %q, want %q", got, value)
	}
	if got := sprintdate.FormatDay(parsed.UTC()); got != "2026-09-09" {
		t.Errorf("format day = %q, want the bare date", got)
	}
}

// TestABareDateKeepsTheMachinesOwnOffset is the rule the plan states: a date
// input says nothing about a time zone, so the machine's own is what the
// sprint is started in, rather than UTC standing in for it silently.
func TestABareDateKeepsTheMachinesOwnOffset(t *testing.T) {
	got, err := sprintdate.Parse("2026-09-09")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := time.Date(2026, 9, 9, 9, 0, 0, 0, time.Local)
	if !got.Equal(want) || got.Location() != time.Local {
		t.Errorf("parse = %s (%s), want %s in the local zone", got, got.Location(), want)
	}
}
