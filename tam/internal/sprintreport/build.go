package sprintreport

import (
	"context"
	"errors"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/donerule"
	"agile-suite/tam/internal/reports"
)

// Build is one sprint's report and its board's velocity table, in one call
// under one lock.
//
// They come back together because the view needs both the moment it opens
// and the per-profile lock refuses rather than waits: two bound calls made
// on mount would have raced, and one of them would have been refused every
// single time the view was opened. Together they also share the work, since
// the sprint being reported on is usually the newest row of the table.
//
// sprintID may be zero, which asks for the board's most recent closed
// sprint. That is the report a scrum master wants the morning after a
// sprint closes, and it is what the view has to ask for before its own
// sprint picker has anything in it.
//
// refresh throws the stored copies away and builds every series in the
// report again from the wire, writing each closed one back over the row it
// replaces. It is the only way a report that is wrong can be made right:
// reports.AlgoVersion invalidates a stored row when this app's own
// reconstruction changes, and nothing invalidates one that was built while
// Jira was handing back cut short changelogs, so without this a table
// carrying a truncation marker would carry it forever however many times
// the view was reopened. It costs the whole table's fetch, six sprints of
// changelog and the most expensive thing TAM does, so it is what a user
// asks for and never what a view does on mount.
//
// The order matters in one place. The board's rule for what counts as
// finished is read first, before a single request goes out, because a
// changelog TAM cannot classify is worth nothing and a sprint's changelog
// takes minutes to fetch.
func (s *Service) Build(ctx context.Context, profileID string, boardID, sprintID int, refresh bool) (Report, error) {
	cols, err := s.store.Columns(ctx, profileID, boardID)
	if err != nil {
		return Report{}, err
	}
	done := donerule.Done(cols)
	if done == nil {
		return unavailable(ReasonBoardNotSynced), nil
	}

	cached, err := s.store.ListSprints(ctx, profileID, boardID)
	if err != nil {
		return Report{}, err
	}
	all := boardSprints(cached)
	sprint, reason := choose(all, sprintID)
	if reason != "" {
		return unavailable(reason), nil
	}

	series, builtAt, err := s.seriesFor(ctx, profileID, boardID, sprint, done, PhaseSprint, refresh)
	if err != nil {
		if errors.Is(err, reports.ErrNoDates) {
			return unavailable(ReasonNoDates), nil
		}
		return Report{}, err
	}

	rows, err := s.velocity(ctx, profileID, boardID, all, done, series, refresh)
	if err != nil {
		return Report{}, err
	}
	return Report{Series: series, Velocity: rows, BuiltAt: builtAt}, nil
}

// choose is the sprint a report is about: the one named, or the board's
// most recent closed sprint when none was.
//
// Zero means the board's most recent closed sprint, deliberately and not
// by falling through. It is the call the view makes before its own sprint
// picker has anything in it, and it is what makes ReasonNoClosedSprint
// reachable at all, since a named sprint is either in the cache or it is
// not. The default reads the last row of reports.VelocitySprints rather
// than sorting the sprints again here. That function is already the one
// place that decides which sprints are reportable and in what order,
// oldest first, so its last row is the newest closed sprint by definition
// and the report a call with no sprint id opens on can never be a sprint
// the velocity table below would leave out.
//
// A negative id is not that, and is answered as a sprint that does not
// exist. Wails marshals a bound method's arguments with JSON.stringify, so
// an undefined or a NaN from the frontend arrives as null and decodes into
// this int as zero; that much is indistinguishable from asking for the
// default and has to be. A number that was actually negative is a caller
// with a real bug, and quietly reporting on a different sprint than the
// one it named is the worst available answer.
func choose(all []backend.Sprint, sprintID int) (backend.Sprint, string) {
	if sprintID < 0 {
		return backend.Sprint{}, ReasonSprintNotFound
	}
	if sprintID > 0 {
		for _, sp := range all {
			if sp.ID == sprintID {
				return sp, ""
			}
		}
		return backend.Sprint{}, ReasonSprintNotFound
	}
	table := reports.VelocitySprints(all)
	if len(table) == 0 {
		return backend.Sprint{}, ReasonNoClosedSprint
	}
	return table[len(table)-1], ""
}

// boardSprints is the cache's sprint rows as the reconstruction takes them.
// The two types carry the same fields; they are separate because one is a
// table's shape and the other is the backend's, and neither package should
// have to import the other to say so.
func boardSprints(rows []boardrepo.Sprint) []backend.Sprint {
	out := make([]backend.Sprint, 0, len(rows))
	for _, r := range rows {
		out = append(out, backend.Sprint{
			ID:           r.ID,
			BoardID:      r.BoardID,
			Name:         r.Name,
			State:        r.State,
			StartDate:    r.StartDate,
			EndDate:      r.EndDate,
			Goal:         r.Goal,
			CompleteDate: r.CompleteDate,
		})
	}
	return out
}
