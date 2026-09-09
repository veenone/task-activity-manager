// Package sprintdate turns a sprint date between the shapes TAM meets it
// in: the datetime Jira's Agile API sends and expects, the bare date an
// HTML date input produces, and Go's own time.
//
// Two packages need the same reading. boardrepo measures how long a board's
// past sprints ran, from the dates the sync cached; the sprints service
// sends a start and an end to Jira. A parser used from two places is a
// module of its own rather than a copy in each, which is how the two would
// come to disagree about what "2026-09-09" means.
package sprintdate

import (
	"fmt"
	"strings"
	"time"
)

// Agile is the datetime format Jira's Agile API sends and expects
// (2026-09-09T09:00:00.000+0000). It is not RFC 3339: the offset carries no
// colon, so time.RFC3339 neither parses nor produces it.
const Agile = "2006-01-02T15:04:05.000-0700"

// Day is the bare date an HTML date input produces and shows.
const Day = "2006-01-02"

// dayHour is the time of day a bare date is read at. A date input has no
// time of day and Jira insists on one, so both ends of a sprint take nine in
// the morning of the machine's own zone: the working-day reading of "this
// sprint ends on the 23rd", rather than a midnight that ends it before the
// day it names has begun.
const dayHour = 9

// layouts are what Parse accepts, in the order it tries them: Jira's Agile
// datetime, both RFC 3339 shapes, which is what the demo backend and any
// hand-written fixture carry, and the bare date.
var layouts = []string{Agile, time.RFC3339Nano, time.RFC3339, Day}

// Parse reads any of those and returns the moment it names, in the
// machine's own zone for a value that carries no offset. An empty or
// unreadable value is an error naming it rather than a zero time passed
// quietly along: these values end up in the write that starts a sprint for a
// whole team.
func Parse(value string) (time.Time, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return time.Time{}, fmt.Errorf("a date is missing")
	}
	for _, layout := range layouts {
		t, err := time.ParseInLocation(layout, v, time.Local)
		if err != nil {
			continue
		}
		if layout == Day {
			return time.Date(t.Year(), t.Month(), t.Day(), dayHour, 0, 0, 0, time.Local), nil
		}
		return t, nil
	}
	return time.Time{}, fmt.Errorf("%q is not a date TAM can read; write it as YYYY-MM-DD", value)
}

// Format renders a moment the way Jira's Agile API wants it written.
func Format(t time.Time) string { return t.Format(Agile) }

// FormatDay renders the bare date a date input shows.
func FormatDay(t time.Time) string { return t.Format(Day) }
