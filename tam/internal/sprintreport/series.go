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
// sprint's is written. A closed sprint's series cannot change: its cards can
// still be reopened and re-estimated afterwards, and none of that moves a
// walk that ended when the sprint did, which is why storing it is safe and
// why reading it back is the whole reason the report is openable offline. A
// live sprint's series changes every time somebody drags a card, so serving
// the stored one would answer this morning's question with yesterday's
// numbers and look exactly like a correct answer while doing it.
//
// The second return is when the series was built: the stamp the store kept
// for a copy read back, and now for one built here.
func (s *Service) seriesFor(ctx context.Context, profileID string, boardID int, sprint backend.Sprint, done func(string) bool, phase string) (reports.Series, string, error) {
	closed := reports.Closed(sprint)
	if closed {
		saved, ok, err := s.store.Report(ctx, profileID, boardID, sprint.ID)
		if err != nil {
			return reports.Series{}, "", err
		}
		// ok is false both for a sprint nothing was stored for and for one
		// whose stored row an older reports.AlgoVersion wrote, which
		// boardrepo.Report deliberately does not tell apart: either way the
		// answer is to build it again and store the result over the top.
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
