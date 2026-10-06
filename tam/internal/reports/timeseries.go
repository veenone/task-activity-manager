package reports

import (
	"sort"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/sprintdate"
)

// The burndown counted in time, beside the one counted in estimates.
//
// It answers a different question from the points line, and from a
// different pair of facts. The points line is reconstructed: every change
// is rewound and replayed, so it says what the sprint looked like on each
// day. This one is measured: the scope is what the cards in the sprint were
// estimated at, and what burns it down is the work somebody logged, dated
// by the day they logged it for.
//
// Two deliberate simplifications, both of them in the scope rather than in
// the burn:
//
// ponytail: an estimate is taken as it stands today, not rewound through
// the changelog the way story points are. Jira's timeoriginalestimate
// changes are not among the fields the normaliser recognises, and a
// re-estimate mid-sprint therefore moves the whole scope line rather than
// stepping it on the day it happened. Teach normalizeChanges the field and
// rewind it here if a team re-estimates in hours often enough to notice.
//
// ponytail: a worklog is counted against the sprint when its own day falls
// inside it, whatever the card was doing at the time. An hour logged
// against a card that was out of the sprint that day still burns, because
// the card is in the sprint now and the hour was spent on the work the
// sprint carries.

// hour is what a second count is divided by to reach the unit these days
// are drawn in. The series carries hours rather than seconds because every
// surface that draws a Day already formats a small number, and a chart axis
// in seconds reads as telephone numbers.
const hour = 3600

// workLog is one entry reduced to what a burndown needs: when the work was
// done and how long it took.
type workLog struct {
	at      time.Time
	seconds int
}

// logsOf reads worklogs into the burn, dropping any entry whose
// timestamp will not parse. That is the one place this file is laxer than
// the changelog walk, which refuses the whole report over an unreadable
// date: a changelog entry nobody can read may have moved a card between
// sprints, where a worklog nobody can read is an hour of work missing from
// a line that is already an approximation of effort.
func logsOf(entries []backend.Worklog) []workLog {
	out := make([]workLog, 0, len(entries))
	for _, w := range entries {
		at, err := sprintdate.Parse(w.Started)
		if err != nil {
			continue
		}
		out = append(out, workLog{at: at, seconds: w.Seconds})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].at.Before(out[j].at) })
	return out
}

// estimated reports whether any card in the sprint carries an estimate in
// time. A sprint where none does has no time burndown at all: a flat line
// at zero would say the team finished everything before it started.
func (w *walker) estimated() bool {
	for _, c := range w.cards {
		if w.scopeOf(c) != nil {
			return true
		}
	}
	return false
}

// scopeOf is the estimate a card stands for.
//
// A card whose sub-tasks are in the sprint beside it stands for itself
// alone: the children are cards in their own right and carry their own
// estimates, so counting the family here would count them twice. A card
// whose sub-tasks are not in the sprint stands for the family, because
// that is the work the sprint actually took on; for a leaf the two are
// the same number, so nothing special happens to one (#142).
func (w *walker) scopeOf(c *card) *int {
	if w.hasChildInSprint[c.key] {
		return c.estimate
	}
	if c.family.EstimateSeconds != nil {
		return c.family.EstimateSeconds
	}
	return c.estimate
}

// burnOf is the work that counts against that estimate: the card's own,
// plus its sub-tasks' when the card is standing for the family.
func (w *walker) burnOf(c *card) []workLog {
	if w.hasChildInSprint[c.key] {
		return c.logs
	}
	return append(append([]workLog{}, c.logs...), c.familyLogs...)
}

// timeTotals is the estimated scope of the cards in the sprint right now,
// and the hours logged against those same cards up to boundary. A card with
// no estimate adds nothing to the scope, the way an unestimated card adds
// no points, and its logged hours still burn: the work happened.
func (w *walker) timeTotals(boundary time.Time) (scope, burned float64) {
	for _, c := range w.cards {
		if !c.in {
			continue
		}
		if e := w.scopeOf(c); e != nil {
			scope += float64(*e) / hour
		}
		for _, l := range w.burnOf(c) {
			if l.at.Before(boundary) && !l.at.Before(w.firstMoment) {
				burned += float64(l.seconds) / hour
			}
		}
	}
	return scope, burned
}

// timeDay is one day of the time burndown, taken at the same instant the
// points day is. Remaining stops at zero: a team that logs more hours than
// it estimated has overrun, which the line says by reaching the floor,
// not by burning through it into negative work.
func (w *walker) timeDay(date string, boundary time.Time, committed float64, elapsed, working int) Day {
	scope, burned := w.timeTotals(boundary)
	remaining := scope - burned
	if remaining < 0 {
		remaining = 0
	}
	return Day{
		Date:      date,
		Scope:     scope,
		Completed: burned,
		Remaining: remaining,
		Ideal:     ideal(committed, elapsed, working),
	}
}
