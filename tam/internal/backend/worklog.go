package backend

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Worklog is one entry against an issue, as the panel reads it. Started
// carries its own UTC offset, because that offset is what Jira dates the
// entry by. TimeSpent is the phrase somebody typed and Seconds is what Jira
// made of it, which is the only one a total can be added up from.
//
// Pending marks an entry the journal holds and Commit has not pushed, and
// PendingID is its journal row, for Discard. They mirror Link's pair for the
// same reason: the panel draws Jira's entries and TAM's own in one list, and
// only the second kind can be taken back.
type Worklog struct {
	ID         string `json:"id"`
	Author     string `json:"author"`
	AuthorName string `json:"authorName"`
	Started    string `json:"started"`
	TimeSpent  string `json:"timeSpent"`
	Seconds    int    `json:"seconds"`
	Comment    string `json:"comment"`
	Pending    bool   `json:"pending"`
	PendingID  int64  `json:"pendingId"`
}

// WorklogDraft is an entry TAM holds in the journal until Commit sends it.
// Seconds is kept beside the typed phrase so a pending entry can be added
// into the section's total without parsing the phrase again at every render.
type WorklogDraft struct {
	Started   string `json:"started"`
	TimeSpent string `json:"timeSpent"`
	Comment   string `json:"comment"`
	Seconds   int    `json:"seconds"`
}

// WorklogBackend is the pair of worklog calls. It is an optional interface
// rather than part of IssueBackend, the way BoardCreator is: a backend that
// cannot answer it says so once, and the section and the Commit phase report
// that instead of pretending the entry went somewhere.
type WorklogBackend interface {
	// Worklogs is every entry Jira holds for the issue, in the order Jira
	// returns them.
	Worklogs(ctx context.Context, key string) ([]Worklog, error)
	// AddWorklog logs the draft against the issue.
	AddWorklog(ctx context.Context, key string, d WorklogDraft) error
}

// startedLayout is the shape Jira takes a worklog's started in: a local
// clock reading with the offset that dates it. Sending the same instant as
// UTC moves the work to another day for anyone away from Greenwich, which is
// the one thing a worklog must not do.
const startedLayout = "2006-01-02T15:04:05.000-0700"

// A day and a week in Jira's durations are the instance's own settings, and
// these are its defaults. They are only used for the total TAM shows beside
// a pending entry; Jira's own seconds replace it once Commit has pushed, so
// an instance with a six-hour day reads a pending "1d" a little high until
// then rather than reporting a wrong number for ever.
const (
	workHoursPerDay = 8
	workDaysPerWeek = 5
)

// durationPart is one "2h", "30m", "1.5h" or bare-number piece of a Jira
// duration. A bare number is minutes, which is what Jira's own field does
// with one.
var durationPart = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)([wdhm]?)$`)

// ParseWorkSeconds reads a Jira duration ("2h 30m", "90m", "1d") and answers
// the seconds in it. It is the one place the rule lives: the entry form shows
// this refusal under the field it was typed in, and the journal write calls
// it again before a row is written, so neither can accept what the other
// would not. Jira itself refuses an unparseable or zero value with a 400,
// which without this would surface at Commit rather than at the keystroke.
func ParseWorkSeconds(s string) (int, error) {
	fields := strings.Fields(strings.ToLower(s))
	if len(fields) == 0 {
		return 0, fmt.Errorf("enter how long you worked, for example 2h 30m")
	}
	total := 0.0
	for _, f := range fields {
		m := durationPart.FindStringSubmatch(f)
		if m == nil {
			return 0, fmt.Errorf("%q is not a duration Jira understands. Use w, d, h or m, for example 2h 30m, 90m or 1d", strings.TrimSpace(s))
		}
		n, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return 0, fmt.Errorf("%q is not a duration Jira understands. Use w, d, h or m, for example 2h 30m, 90m or 1d", strings.TrimSpace(s))
		}
		switch m[2] {
		case "w":
			total += n * workDaysPerWeek * workHoursPerDay * 3600
		case "d":
			total += n * workHoursPerDay * 3600
		case "h":
			total += n * 3600
		default:
			total += n * 60
		}
	}
	seconds := int(total)
	if seconds <= 0 {
		return 0, fmt.Errorf("a worklog of no time is not a worklog. Enter how long you worked, for example 2h 30m")
	}
	return seconds, nil
}

// NewWorklogDraft builds the journal payload from what the user typed and the
// moment they logged it. at is the logger's own clock, zone and all: its
// offset is what tells Jira which day the work belongs to.
func NewWorklogDraft(timeSpent, comment string, at time.Time) (WorklogDraft, error) {
	seconds, err := ParseWorkSeconds(timeSpent)
	if err != nil {
		return WorklogDraft{}, err
	}
	return WorklogDraft{
		Started:   at.Format(startedLayout),
		TimeSpent: strings.TrimSpace(timeSpent),
		Comment:   strings.TrimSpace(comment),
		Seconds:   seconds,
	}, nil
}
