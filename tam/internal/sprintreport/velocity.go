package sprintreport

import (
	"context"
	"errors"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/reports"
)

// velocity is the board's velocity table, built out of whatever is already
// stored plus whatever has to be fetched.
//
// Reusing the stored series is the point: six sprints of changelog is the
// most expensive thing this app does, and five of those six were already
// reconstructed the last time somebody opened a report on this board. What
// makes that safe rather than clever is that a row is a row however its
// series arrived. reports.VelocitySprints picks the sprints and their order,
// reports.Row turns a series into a row, and seriesFor decides between the
// store and the wire, so a sprint built just now and the same sprint read
// back out of the store give the identical row.
//
// have is the series Build has already reconstructed for the sprint the
// report is about. It is almost always the newest row of this table, and
// passing it in is what keeps the report from fetching or reading that one
// sprint twice in a single call.
//
// A sprint whose dates cannot be read is dropped from the table rather than
// shown as a zero, for the reason reports.VelocitySprints gives about the
// sprints it drops for the same fault: a zero row reads as a sprint that
// delivered nothing. VelocitySprints has already dropped the ones whose
// start or end will not parse; what reaches this drop is the one it cannot
// see, a sprint whose dates both parse and run backwards. Anything else
// that goes wrong, a failed fetch or a database that will not answer, fails
// the whole call instead of quietly returning a shorter table that looks
// complete.
func (s *Service) velocity(ctx context.Context, profileID string, boardID int, all []backend.Sprint, done func(string) bool, have reports.Series, refresh bool) ([]reports.VelocityRow, error) {
	picked := reports.VelocitySprints(all)
	rows := make([]reports.VelocityRow, 0, len(picked))
	for _, sprint := range picked {
		if sprint.ID == have.SprintID {
			rows = append(rows, reports.Row(have))
			continue
		}
		series, _, err := s.seriesFor(ctx, profileID, boardID, sprint, done, PhaseVelocity, refresh)
		if err != nil {
			if errors.Is(err, reports.ErrNoDates) {
				continue
			}
			return nil, err
		}
		rows = append(rows, reports.Row(series))
	}
	return rows, nil
}
