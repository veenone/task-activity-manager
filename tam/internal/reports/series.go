package reports

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
	fieldSprint = "sprint"
	fieldStatus = "status"
	fieldPoints = "storyPoints"
)

// change is one changelog entry with its timestamp already read.
type change struct {
	at    time.Time
	field string
	from  string
	to    string
}

// card is one issue's reconstructed state as the walk moves through the
// sprint: whether it is in the sprint, the name of the status it is in,
// and the estimate it carries.
//
// The two charged flags are what keep a card that left and came back from
// being counted as scope twice. A crossing is charged the first time it
// happens in each direction and never again, so the card that bounces out
// on Tuesday and back on Thursday shows once in Removed and once in Added
// rather than describing a sprint that gained and lost work all week.
type card struct {
	in             bool
	status         string
	points         *float64
	changes        []change
	addedCharged   bool
	removedCharged bool
}

// rewound is the one place the reconstruction runs backwards, and the
// reason the package comment leads with it.
//
// Each card starts from what the issue row says it is today and then has
// every change dated after the sprint's start undone, oldest undone last,
// which leaves it holding the values it had when the sprint began. Undoing
// a change means taking its "from" side, which is why a status or an
// estimate written a fortnight after the sprint closed cannot reach the
// series: the rewind steps back past it before the forward walk starts,
// and the walk never replays a change dated after the sprint ended.
//
// Changes dated exactly at the start are deliberately not undone. Their
// "to" side is the value the sprint opened with.
func rewound(sprint backend.Sprint, issues []backend.IssueHistory, start time.Time) ([]*card, error) {
	id := strconv.Itoa(sprint.ID)
	out := make([]*card, 0, len(issues))
	for _, h := range issues {
		chs, err := parseChanges(h)
		if err != nil {
			return nil, err
		}
		c := &card{in: h.Issue.SprintID == id, status: h.Issue.Status, changes: chs}
		if p := h.Issue.StoryPoints; p != nil {
			v := *p
			c.points = &v
		}
		for i := len(chs) - 1; i >= 0; i-- {
			ch := chs[i]
			if !ch.at.After(start) {
				break
			}
			switch ch.field {
			case fieldSprint:
				c.in = inSprint(ch.from, sprint)
			case fieldStatus:
				c.status = ch.from
			case fieldPoints:
				c.points = parsePoints(ch.from)
			}
		}
		out = append(out, c)
	}
	return out, nil
}

// parseChanges reads every timestamp through sprintdate, which is the only
// parser in this repo that accepts Jira's datetime: the offset carries no
// colon, so time.RFC3339 rejects the real thing outright while a fixture
// written with a Z sails through.
//
// The sort is not distrust of backend.IssueHistory, which documents its
// changes as oldest first. It is that the rewind below reads the slice as
// an ordered timeline and stops at the first entry old enough to keep, so
// one entry out of order would leave a card holding a value from the wrong
// side of the sprint's start with nothing anywhere saying so. The Jira
// backend passes through whatever order the instance sent, which is a
// promise made somewhere this package cannot see, and one comparison pass
// is a cheap price for not resting on it.
func parseChanges(h backend.IssueHistory) ([]change, error) {
	out := make([]change, 0, len(h.Changes))
	for _, ch := range h.Changes {
		at, err := sprintdate.Parse(ch.At)
		if err != nil {
			return nil, fmt.Errorf("%s: a changelog entry is dated %q, which TAM cannot read, and a report missing one change is wrong without saying so: %w", h.Issue.Key, ch.At, err)
		}
		out = append(out, change{at: at, field: ch.Field, from: ch.From, to: ch.To})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].at.Before(out[j].at) })
	return out, nil
}

// inSprint reports whether one side of a Sprint field change names this
// sprint.
//
// It is a set membership test and not a boolean toggle, because a card can
// sit in two sprints at once, which is ordinary during a rollover: a card
// leaving Sprint 12 while it stays in Sprint 13 changes from "12, 13" to
// "13", and reading that as "the card left a sprint" takes it out of both.
//
// Both the id and the name are accepted, because either can arrive here.
// Jira's changelog carries two halves for every field change, a raw pair
// and a readable pair, and the backend's normaliser keeps the readable one
// whenever it is populated; on the Sprint field that half is expected to
// be the sprint's names, which is the shape the demo's curated history
// models and what probe 2 of
// docs/superpowers/plans/assets/2026-09-11-report-wire-probe.md exists to
// confirm against a real Data Center. Matching both is what keeps one rule
// serving both rather than the reconstruction seeing no membership at all
// against whichever shape it did not expect.
func inSprint(value string, sprint backend.Sprint) bool {
	id := strconv.Itoa(sprint.ID)
	name := strings.TrimSpace(sprint.Name)
	for _, token := range strings.Split(value, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if token == id || (name != "" && strings.EqualFold(token, name)) {
			return true
		}
	}
	return false
}

// parsePoints reads an estimate as the changelog writes it, which is text.
// Blank is no estimate, and so is anything that is not a number: a single
// odd value on one card is not worth refusing a whole sprint's report
// over, and a card with no estimate contributes nothing in points mode
// anyway, which is the same thing this produces.
func parsePoints(value string) *float64 {
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

// pending is one card's change waiting to be replayed.
type pending struct {
	c  *card
	ch change
}

// walker replays the rewound cards forward and adds up what it sees.
type walker struct {
	sprint   backend.Sprint
	cards    []*card
	doneName map[string]bool
	points   bool
	loc      *time.Location
	added    float64
	removed  float64
}

// value is what one card contributes, which is its estimate in points mode
// and one in cards mode. A card with no estimate contributes nothing in
// points mode: counting it as zero is what a board that estimates most of
// its work actually means, and counting it as one would mix the two units
// in a single total.
func (w *walker) value(c *card) float64 {
	if !w.points {
		return 1
	}
	if c.points == nil {
		return 0
	}
	return *c.points
}

// totals is the scope in the sprint right now and the finished part of it.
func (w *walker) totals() (scope, completed float64) {
	for _, c := range w.cards {
		if !c.in {
			continue
		}
		v := w.value(c)
		scope += v
		if w.doneName[c.status] {
			completed += v
		}
	}
	return scope, completed
}

// apply moves one card on by one change.
//
// An estimate takes effect the moment it changes, which for a card outside
// the sprint means it changes nothing anyone can see until the card comes
// back, and then comes back with it. That rule needs no code of its own:
// the estimate is tracked whether the card is in the sprint or not, and
// only the cards that are in it are ever added up.
func (w *walker) apply(c *card, ch change) {
	switch ch.field {
	case fieldSprint:
		was := c.in
		c.in = inSprint(ch.to, w.sprint)
		switch {
		case c.in && !was && !c.addedCharged:
			w.added += w.value(c)
			c.addedCharged = true
		case !c.in && was && !c.removedCharged:
			w.removed += w.value(c)
			c.removedCharged = true
		}
	case fieldStatus:
		c.status = ch.to
	case fieldPoints:
		c.points = parsePoints(ch.to)
	}
}

// run walks the sprint one local day at a time from its start to last,
// taking each day's figures at the day's end, and returns them.
//
// end is the sprint's own end rather than where the walk stops, and it is
// only used for the guide line, so a sprint still running keeps a guide
// aimed at the day it is due to finish.
//
// A day with no changes in it repeats the day before, which is what makes
// a sprint whose start was edited backwards after it began readable: the
// days before the earliest thing the changelog knows about all carry day
// one's figures rather than nothing at all.
func (w *walker) run(start, last, end time.Time, committed float64) []Day {
	first := civil(start, w.loc)
	stop := civil(last, w.loc)
	working := workingDays(first, civil(end, w.loc))
	queue := w.queue(start, last)
	span := daysBetween(first, stop)
	days := make([]Day, 0, span+1)
	elapsed := 0
	for i := 0; i <= span; i++ {
		d := first.AddDate(0, 0, i)
		boundary := time.Date(d.Year(), d.Month(), d.Day()+1, 0, 0, 0, 0, w.loc)
		for len(queue) > 0 && queue[0].ch.at.Before(boundary) {
			w.apply(queue[0].c, queue[0].ch)
			queue = queue[1:]
		}
		if isWorkingDay(d) {
			elapsed++
		}
		scope, completed := w.totals()
		days = append(days, Day{
			Date:      d.Format(sprintdate.Day),
			Scope:     scope,
			Completed: completed,
			Remaining: scope - completed,
			Ideal:     ideal(committed, elapsed, working),
		})
	}
	return days
}

// queue is every change the walk replays, in time order: the ones dated
// after the sprint's start and no later than where the walk stops.
//
// The two bounds are what the rewind and this walk share. Anything at or
// before the start is already in the state the cards were rewound to, and
// anything after the last day was undone by the rewind and is deliberately
// never put back, which is why an issue reopened and re-estimated a week
// after the sprint closed leaves the series exactly as it was.
func (w *walker) queue(start, last time.Time) []pending {
	var out []pending
	for _, c := range w.cards {
		for _, ch := range c.changes {
			if !ch.at.After(start) || ch.at.After(last) {
				continue
			}
			out = append(out, pending{c: c, ch: ch})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ch.at.Before(out[j].ch.at) })
	return out
}

// ideal is the guide line's value after elapsed working days of a sprint
// that has working of them, which is the committed total run down to zero
// in equal steps. A sprint with no working day at all, a weekend hackathon
// for instance, keeps its guide flat rather than dividing by zero.
func ideal(committed float64, elapsed, working int) float64 {
	if working <= 0 {
		return committed
	}
	v := committed * (1 - float64(elapsed)/float64(working))
	if v < 0 {
		return 0
	}
	return v
}

// civil is the calendar date t falls on in loc, carried as a UTC midnight.
//
// The arithmetic below counts and steps days, and it does that in UTC on
// purpose: UTC has no daylight saving, so adding a day can never land on
// the same date twice or skip one, which stepping a local midnight through
// a spring forward can. The one place a real local moment is needed, the
// midnight a day's changes are cut off at, builds it from these fields in
// loc rather than reusing this value.
func civil(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// daysBetween is how many whole days separate two civil dates, negative
// when to is the earlier one.
func daysBetween(from, to time.Time) int {
	return int(to.Sub(from).Hours() / 24)
}

// workingDays counts the Mondays to Fridays from one civil date to another
// inclusive.
func workingDays(from, to time.Time) int {
	n := 0
	for i := 0; i <= daysBetween(from, to); i++ {
		if isWorkingDay(from.AddDate(0, 0, i)) {
			n++
		}
	}
	return n
}

// isWorkingDay is the guide line's definition of a day the team works,
// which is Monday to Friday. It is not configurable: a per-profile working
// week and a holiday calendar are a feature with their own settings and
// their own tests, and nothing in this phase's design asks for one.
func isWorkingDay(d time.Time) bool {
	switch d.Weekday() {
	case time.Saturday, time.Sunday:
		return false
	}
	return true
}
