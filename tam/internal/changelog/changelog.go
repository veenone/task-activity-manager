// Package changelog is the vocabulary for reading a Jira changelog: one
// entry with its timestamp already parsed, the logical field names TAM
// knows those entries by, and the civil-date arithmetic a day-by-day walk
// through them needs.
//
// It holds what more than one report needs and nothing about any one kind
// of report. internal/reports reconstructs a sprint from these entries and
// internal/flow reconstructs a board's flow from them, and neither model
// belongs here: nothing in this package knows what a sprint is, what a
// column is, or what finished means.
//
// It exists because of the trap internal/reports' own package comment opens
// with. A changelog is today's field values plus a list of deltas. It does
// not say what an issue's status or estimate were on the morning a window
// started, so those are derived by unwinding the deltas from the present
// back to that morning, and only then replayed forward. A forward replay
// seeded from today's values draws a history that never happened. That is
// one trap, and two packages reading changelogs off two copies of this code
// would be two chances to get it backwards; internal/donerule's header is
// here to record what two implementations of one rule already cost this
// codebase.
package changelog

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/sprintdate"
)

// The logical field names backend.Change carries. They are the names the
// rest of TAM already uses for the same three fields, not Jira's
// per-instance customfield ids, which each backend translates before a
// change ever reaches here.
const (
	FieldSprint = "sprint"
	FieldStatus = "status"
	FieldPoints = "storyPoints"
)

// Change is one changelog entry with its timestamp already read.
//
// Jira's changelog carries two halves for every field change, a raw pair
// and a readable pair. FromID and ToID are the raw half and may be empty:
// an older cached changelog, or a backend that cannot supply one, moves a
// field by name alone. A caller that needs to tell a status apart reliably
// reads the id first and falls back to the name only when it is empty,
// rather than deciding on a guess when the id is actually known.
type Change struct {
	At     time.Time
	Field  string
	From   string
	To     string
	FromID string
	ToID   string
}

// Parse reads one issue's changelog into entries sorted oldest first.
//
// The sort is not a formality. The order entries arrive in is a promise made
// somewhere outside TAM: the Jira backend passes through whatever the
// instance sent. A replay that trusted that order could apply a change on
// the wrong side of a window's boundary with nothing anywhere saying so, and
// one comparison pass is a cheap price for not resting on it.
//
// An unreadable timestamp refuses the whole issue rather than skipping the
// entry, because a reconstruction missing one change is wrong and says
// nothing about it. Jira's timestamps are not RFC 3339, which is why every
// one of them goes through internal/sprintdate.
func Parse(h backend.IssueHistory) ([]Change, error) {
	out := make([]Change, 0, len(h.Changes))
	for _, ch := range h.Changes {
		at, err := sprintdate.Parse(ch.At)
		if err != nil {
			return nil, fmt.Errorf("%s: a changelog entry is dated %q, which TAM cannot read, and a report missing one change is wrong without saying so: %w", h.Issue.Key, ch.At, err)
		}
		out = append(out, Change{At: at, Field: ch.Field, From: ch.From, To: ch.To, FromID: ch.FromID, ToID: ch.ToID})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out, nil
}

// Points reads an estimate as the changelog writes it, which is text. Blank
// is no estimate, and so is anything that is not a number: a single odd
// value on one card is not worth refusing a whole report over, and a card
// with no estimate contributes nothing in points mode anyway, which is the
// same thing this produces.
func Points(value string) *float64 {
	v := strings.TrimSpace(value)
	if v == "" {
		return nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil
	}
	return &f
}

// Civil is the date part of a moment in loc, as a UTC midnight.
//
// UTC is deliberate and not a shortcut: it has no daylight saving, so
// adding a day to one of these can never land on the same date twice or
// skip one, which stepping a local midnight through a spring forward can. A
// caller needing a real local moment, such as the midnight a day's changes
// are cut off at, builds it from these fields in loc rather than reusing
// this value.
func Civil(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// DaysBetween is how many whole days separate two civil dates, negative
// when to is the earlier one.
func DaysBetween(from, to time.Time) int {
	return int(to.Sub(from).Hours() / 24)
}

// WorkingDays counts the Mondays to Fridays from one civil date to another
// inclusive.
func WorkingDays(from, to time.Time) int {
	n := 0
	for i := 0; i <= DaysBetween(from, to); i++ {
		if IsWorkingDay(from.AddDate(0, 0, i)) {
			n++
		}
	}
	return n
}

// IsWorkingDay is the guide line's definition of a day the team works,
// which is Monday to Friday. It is not configurable: a per-profile working
// week and a holiday calendar are a feature with their own settings and
// their own tests, and nothing that reads this asks for one.
func IsWorkingDay(d time.Time) bool {
	switch d.Weekday() {
	case time.Saturday, time.Sunday:
		return false
	}
	return true
}
