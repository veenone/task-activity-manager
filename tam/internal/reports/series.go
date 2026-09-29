package reports

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/changelog"
	"agile-suite/tam/internal/sprintdate"
)

// card is one issue's reconstructed state as the walk moves through the
// sprint: whether it is in the sprint, the status it is in (by name and,
// when the changelog carried one, by id), and the estimate it carries.
//
// statusID travels with status rather than being derived from it, because
// the two do not always change together: a change that carried no id
// (an older cached changelog, or a backend that cannot supply one) still
// moves status by name and leaves statusID exactly as informed as it was,
// which is empty the moment such a change has happened, not whatever the
// issue row happened to say. done reads statusID first and falls back to
// the name only when it is empty, which is what keeps a card's finished
// state from being decided on a guess when the id is actually known.
//
// The two charged flags are what keep a card that left and came back from
// being counted as scope twice. A crossing is charged the first time it
// happens in each direction and never again, so the card that bounces out
// on Tuesday and back on Thursday shows once in Removed and once in Added
// rather than describing a sprint that gained and lost work all week.
type card struct {
	in             bool
	status         string
	statusID       string
	points         *float64
	changes        []changelog.Change
	addedCharged   bool
	removedCharged bool
}

// done reports whether c's current tracked state counts as finished. The
// id is the primary answer, since a board's rule is keyed by status id;
// the name is the fallback, for the state left behind by a change whose
// id could not be read.
func (c *card) done(byID func(string) bool, byName map[string]bool) bool {
	if c.statusID != "" {
		return byID(c.statusID)
	}
	return byName[c.status]
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
//
// Membership starts true for every card, not from a comparison against the
// issue row's own Sprint field. Every issue Build ever sees came back from
// a "sprint = N" search, so it is in sprint N right now by construction;
// the row's own field can still name a different sprint; parseSprint keeps
// only the last entry of Jira's array, and a card sits in two at once
// during a rollover. Trusting that field here, the way an earlier version
// of this function did, took the older of two sprints a card was still in
// and erased it from that sprint's report entirely.
func rewound(sprint backend.Sprint, issues []backend.IssueHistory, start time.Time) ([]*card, error) {
	out := make([]*card, 0, len(issues))
	for _, h := range issues {
		chs, err := changelog.Parse(h)
		if err != nil {
			return nil, err
		}
		c := &card{in: true, status: h.Issue.Status, statusID: h.Issue.StatusID, changes: chs}
		if p := h.Issue.StoryPoints; p != nil {
			v := *p
			c.points = &v
		}
		for i := len(chs) - 1; i >= 0; i-- {
			ch := chs[i]
			if !ch.At.After(start) {
				break
			}
			switch ch.Field {
			case changelog.FieldSprint:
				c.in = inSprint(ch.From, sprint)
			case changelog.FieldStatus:
				c.status, c.statusID = ch.From, ch.FromID
			case changelog.FieldPoints:
				c.points = changelog.Points(ch.From)
			}
		}
		out = append(out, c)
	}
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

// pending is one card's change waiting to be replayed.
type pending struct {
	c  *card
	ch changelog.Change
}

// walker replays the rewound cards forward and adds up what it sees.
type walker struct {
	sprint   backend.Sprint
	cards    []*card
	doneID   func(string) bool
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
		if c.done(w.doneID, w.doneName) {
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
func (w *walker) apply(c *card, ch changelog.Change) {
	switch ch.Field {
	case changelog.FieldSprint:
		was := c.in
		c.in = inSprint(ch.To, w.sprint)
		switch {
		case c.in && !was && !c.addedCharged:
			w.added += w.value(c)
			c.addedCharged = true
		case !c.in && was && !c.removedCharged:
			w.removed += w.value(c)
			c.removedCharged = true
		}
	case changelog.FieldStatus:
		c.status, c.statusID = ch.To, ch.ToID
	case changelog.FieldPoints:
		c.points = changelog.Points(ch.To)
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
	first := changelog.Civil(start, w.loc)
	stop := changelog.Civil(last, w.loc)
	working := changelog.WorkingDays(first, changelog.Civil(end, w.loc))
	queue := w.queue(start, last)
	span := changelog.DaysBetween(first, stop)
	days := make([]Day, 0, span+1)
	elapsed := 0
	for i := 0; i <= span; i++ {
		d := first.AddDate(0, 0, i)
		boundary := time.Date(d.Year(), d.Month(), d.Day()+1, 0, 0, 0, 0, w.loc)
		for len(queue) > 0 && queue[0].ch.At.Before(boundary) {
			w.apply(queue[0].c, queue[0].ch)
			queue = queue[1:]
		}
		if changelog.IsWorkingDay(d) {
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
			if !ch.At.After(start) || ch.At.After(last) {
				continue
			}
			out = append(out, pending{c: c, ch: ch})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ch.At.Before(out[j].ch.At) })
	return out
}

// ideal is the guide line's value after elapsed working days of a sprint
// that has working of them, which is the committed total run down to zero
// in equal steps. A sprint with no working day at all, a weekend hackathon
// for instance, keeps its guide flat rather than dividing by zero.
//
// An overdue active sprint continues past its planned end. Its ideal line
// stays at zero rather than turning negative while actual work continues.
func ideal(committed float64, elapsed, working int) float64 {
	if working <= 0 {
		return committed
	}
	if elapsed >= working {
		return 0
	}
	return committed * (1 - float64(elapsed)/float64(working))
}
