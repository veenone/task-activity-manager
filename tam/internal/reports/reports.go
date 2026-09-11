// Package reports turns a sprint's issues and their changelogs into the
// numbers a sprint review starts with: what was committed, what was added
// and removed while it ran, what finished, what carried over, and the
// day-by-day line behind those totals.
//
// The whole package rests on one idea, and getting it backwards is the way
// this code goes quietly wrong. A changelog is today's field values plus a
// list of deltas. It does not say what an issue's status, estimate or
// sprint membership were on the morning the sprint started, so those are
// derived by unwinding the deltas from the present back to the start, and
// only then replayed forward day by day. A forward replay seeded from
// today's values draws a sprint that never happened: an issue closed and
// re-pointed a week after the sprint ended would begin the reconstruction
// already done, carrying the estimate it has now.
//
// Two smaller traps sit beside it. Jira's timestamps are not RFC 3339, so
// every one of them is read through internal/sprintdate, which says so in
// its own comment; and the Sprint field's changelog values are comma
// separated lists rather than single ids, because a card sits in two
// sprints at once during a rollover.
//
// What this cannot see is written down in section 3 of
// docs/superpowers/specs/2026-09-09-tam-reports-design.md and is not a
// detail: the issues come from a "sprint = N" search, which returns
// whoever is in the sprint now, so a card dragged out on day four and left
// out is never fetched and leaves no trace here. Committed is therefore a
// floor, and Removed counts only the cards that left and came back, which
// is the one departure the query can still observe. Every surface that
// prints these numbers has to carry that qualification.
package reports

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/sprintdate"
)

// The units a series can be counted in.
const (
	UnitPoints = "points"
	UnitCards  = "cards"
)

// Why a series counts cards instead of points. The two are a different
// problem and only one of them is the user's to fix, which is the whole
// reason the reason is carried rather than just the unit.
const (
	// ReasonNothingEstimated is a sprint whose cards could carry story
	// points and do not: the field is in evidence somewhere in this
	// sprint's issues or their history, and nothing inside the sprint's
	// own window ever held a value. Estimating the cards would give this
	// board a points burndown.
	ReasonNothingEstimated = "nothingEstimated"
	// ReasonNoPointsFieldSeen is a sprint where nothing at all mentions
	// story points: no card carries a value and no changelog entry names
	// the field. That is what a Jira instance without a Story Points
	// field looks like from in here, which is what this constant is named
	// for, but it is evidence and not proof. Build is handed issues and
	// changelogs, never the instance's field list, so a board that has
	// the field and has simply never once used it is indistinguishable
	// from a board that does not have it, and reads as this.
	ReasonNoPointsFieldSeen = "noPointsFieldSeen"
)

// Day is one local day of a sprint, measured at that day's end.
//
// Date is the local calendar date, bucketed in the location Build was
// given, with day one being the local date of the sprint's start. The zone
// is not a formality: bucketing in UTC gives a team ten hours ahead a day
// one that begins the previous afternoon, so half of their first morning's
// work lands on a day the sprint does not contain.
type Day struct {
	Date string `json:"date"`
	// Scope is the work in the sprint at the end of this day, Completed
	// the part of it the board's last column holds, and Remaining the
	// difference, which is the burndown line.
	Scope     float64 `json:"scope"`
	Completed float64 `json:"completed"`
	Remaining float64 `json:"remaining"`
	// Ideal is the guide line: the committed total run down to zero over
	// the sprint's working days, Monday to Friday. It is drawn against
	// the sprint's full length even when the walk stops at today, so a
	// live sprint's guide does not steepen as the days pass.
	Ideal float64 `json:"ideal"`
}

// Series is one sprint's reconstructed history.
type Series struct {
	SprintID   int    `json:"sprintId"`
	SprintName string `json:"sprintName"`
	// Unit is UnitPoints or UnitCards, and UnitReason says why when it is
	// cards. UnitReason is empty for points, where there is nothing to
	// explain.
	Unit       string `json:"unit"`
	UnitReason string `json:"unitReason"`
	// Committed is the scope at the instant the sprint started. A card
	// already finished before then is in it, with nothing left to burn:
	// it was part of what the team took on, and leaving it out would make
	// the sprint look smaller than it was planned to be.
	Committed float64 `json:"committed"`
	// Added and Removed are scope that crossed the sprint's edge while it
	// ran, each card charged once per direction, at the estimate it held
	// when it crossed. Removed can only ever hold cards that came back,
	// for the reason the package comment gives.
	Added   float64 `json:"added"`
	Removed float64 `json:"removed"`
	// Completed and CarriedOver split the scope at the end of the walk by
	// the board's own rule for finished.
	Completed   float64 `json:"completed"`
	CarriedOver float64 `json:"carriedOver"`
	// Days runs from the sprint's first local day to the last one the
	// walk reached, which is the sprint's end for a closed sprint and
	// today for one still running.
	Days []Day `json:"days"`
	// Truncated names the issues whose changelog came back cut short.
	// Their part of these numbers rests on a partial history, so a report
	// holding any of them is not exact and has to say so.
	Truncated []string `json:"truncated"`
}

// Build reconstructs one sprint from its issues' changelogs.
//
// done is the board's rule for finished, which internal/donerule builds
// from the board's last column. now and loc are taken rather than reached
// for, the way syncer.Engine takes its clock: a live sprint's walk stops
// at now, and days bucket at midnight in loc, so a test pins both. Reading
// the machine's clock and zone instead would leave a live sprint's series
// either non-deterministic or untestable.
//
// It refuses rather than half-answers. A sprint with no readable dates
// cannot be reported on at all, and a changelog entry whose timestamp will
// not parse fails the whole build, because a report quietly missing one
// change is wrong without saying so.
func Build(sprint backend.Sprint, done func(string) bool, issues []backend.IssueHistory, now time.Time, loc *time.Location) (Series, error) {
	if loc == nil {
		return Series{}, errors.New("a sprint report needs the zone its days are bucketed in")
	}
	if done == nil {
		return Series{}, fmt.Errorf("sprint %d has no rule for what counts as finished, so nothing can be burned down", sprint.ID)
	}
	start, err := sprintdate.Parse(sprint.StartDate)
	if err != nil {
		return Series{}, fmt.Errorf("sprint %d has no start date TAM can read, so there is nothing to reconstruct: %w", sprint.ID, err)
	}
	end, err := sprintdate.Parse(sprint.EndDate)
	if err != nil {
		return Series{}, fmt.Errorf("sprint %d has no end date TAM can read, so there is nothing to reconstruct: %w", sprint.ID, err)
	}
	if end.Before(start) {
		return Series{}, fmt.Errorf("sprint %d ends before it starts, so it has no days to walk", sprint.ID)
	}

	cards, err := rewound(sprint, issues, start)
	if err != nil {
		return Series{}, err
	}

	// The walk stops at the sprint's end or at now, whichever comes
	// first, so a sprint still running is not drawn with days that have
	// not happened yet. A sprint whose start is still in the future gets
	// the single day it starts on rather than an empty series.
	last := end
	if now.Before(last) {
		last = now
	}
	if last.Before(start) {
		last = start
	}

	w := &walker{
		sprint:   sprint,
		cards:    cards,
		doneName: doneNames(issues, done),
		loc:      loc,
	}
	s := Series{SprintID: sprint.ID, SprintName: sprint.Name, Truncated: truncatedKeys(issues)}
	s.Unit, s.UnitReason = unitOf(cards, issues, start, last)
	w.points = s.Unit == UnitPoints
	// Committed is read off the rewound state before a single change is
	// replayed, which is the one moment the cards hold the values they
	// had when the sprint started.
	s.Committed, _ = w.totals()
	s.Days = w.run(start, last, end, s.Committed)
	scope, completed := w.totals()
	s.Added, s.Removed = w.added, w.removed
	s.Completed, s.CarriedOver = completed, scope-completed
	return s, nil
}

// unitOf decides what this sprint is counted in and why. Points win as
// soon as one card in the sprint carries an estimate, either at the start
// or after a re-estimate inside the sprint, since a board that estimates
// some of its work is still a board that estimates.
func unitOf(cards []*card, issues []backend.IssueHistory, start, last time.Time) (string, string) {
	for _, c := range cards {
		if c.points != nil {
			return UnitPoints, ""
		}
		for _, ch := range c.changes {
			if ch.field != fieldPoints || !ch.at.After(start) || ch.at.After(last) {
				continue
			}
			if parsePoints(ch.to) != nil {
				return UnitPoints, ""
			}
		}
	}
	return UnitCards, cardsReason(issues)
}

// cardsReason tells the two ways a sprint ends up counting cards apart, as
// far as anything in these inputs can. Any sign of the field anywhere,
// including on a card estimated long after this sprint closed, is taken as
// the field existing on this instance.
func cardsReason(issues []backend.IssueHistory) string {
	for _, h := range issues {
		if h.Issue.StoryPoints != nil {
			return ReasonNothingEstimated
		}
		for _, ch := range h.Changes {
			if ch.Field == fieldPoints {
				return ReasonNothingEstimated
			}
		}
	}
	return ReasonNoPointsFieldSeen
}

// doneNames is the board's rule translated from status ids to status
// names, because a changelog says an issue moved to "In Review" and never
// which id that was: the backend's normaliser keeps the readable half of
// each change, and the ids live only on the issue rows.
//
// The pairs come from the sprint's own issues, each of which carries its
// current status beside the id that goes with it. A historical status no
// card in this sprint currently sits in cannot be translated and reads as
// not done, which keeps that card's work on the burndown for the days it
// was in that status rather than retiring it on a guess. It is a real
// limit, and it bites hardest on a small sprint whose cards have all since
// moved on; the alternative, carrying status ids through the changelog, is
// a change to backend.Change and to both backends that implement it.
func doneNames(issues []backend.IssueHistory, done func(string) bool) map[string]bool {
	out := map[string]bool{}
	for _, h := range issues {
		if h.Issue.Status == "" {
			continue
		}
		out[h.Issue.Status] = done(h.Issue.StatusID)
	}
	return out
}

// truncatedKeys is the issues whose changelog came back cut short, sorted
// so two builds of the same sprint do not differ only in this list's
// order once one of them has been stored.
func truncatedKeys(issues []backend.IssueHistory) []string {
	out := []string{}
	for _, h := range issues {
		if h.Truncated {
			out = append(out, h.Issue.Key)
		}
	}
	sort.Strings(out)
	return out
}
