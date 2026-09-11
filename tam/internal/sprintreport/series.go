package sprintreport

import (
	"context"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/reports"
)

// seriesFor is the one path from a sprint to its series, used by the sprint
// the report is about and by every row of the velocity table, so the two
// halves of one screen cannot be built two different ways.
//
// The stored copy is read only for a closed sprint, and only a closed
// sprint's is written. A live sprint's series changes every time somebody
// drags a card, so serving the stored one would answer this morning's
// question with yesterday's numbers and look exactly like a correct answer
// while doing it.
//
// A closed sprint's series is not fixed, and the reason for keeping it is
// narrower than that. Reopening a card or re-estimating it afterwards does
// not move a walk that ended when the sprint did, so the numbers survive
// those; what they do not survive is a change of membership, because the
// series is built from a "sprint = N" search and a card moved into or out
// of a closed sprint, or deleted outright, is a different set of issues to
// reconstruct from. So a stored series is what this sprint looked like from
// the last fetch and not what it will look like from the next one, which is
// exactly why refresh exists.
//
// refresh skips the read, so the series is fetched and rebuilt, and the
// write that follows it replaces the stored row. That is the only path that
// overwrites a row reports.AlgoVersion still considers current, and the
// only one that can clear a truncation marker a bad afternoon at Jira's end
// left behind.
//
// The second return is when the series was built: the stamp the store kept
// for a copy read back, and now for one built here.
func (s *Service) seriesFor(ctx context.Context, profileID string, boardID int, sprint backend.Sprint, done func(string) bool, phase string, refresh bool) (reports.Series, string, error) {
	closed := reports.Closed(sprint)
	if closed && !refresh {
		saved, ok, err := s.store.Report(ctx, profileID, boardID, sprint.ID)
		if err != nil {
			return reports.Series{}, "", err
		}
		// ok is false both for a sprint nothing was stored for and for one
		// whose stored row an older reports.AlgoVersion wrote, which
		// boardrepo.Report deliberately does not tell apart: either way the
		// answer is to build it again and store the result over the top.
		// Its third answer is not one of those. A stored row whose JSON
		// will not decode comes back as an error and fails the call, and
		// the write below is never reached to replace it; a refresh, which
		// does not read the row at all, is what gets past that one.
		if ok {
			return saved.Series, saved.BuiltAt, nil
		}
	}

	issues, err := s.fetch(ctx, sprint, phase)
	if err != nil {
		return reports.Series{}, "", err
	}
	series, err := reports.Build(sprint, done, issues, s.Now(), s.Loc)
	if err != nil {
		return reports.Series{}, "", err
	}
	if closed {
		if err := s.store.SaveReport(ctx, profileID, boardID, series); err != nil {
			return reports.Series{}, "", err
		}
	}
	return series, s.Now().UTC().Format(time.RFC3339), nil
}
