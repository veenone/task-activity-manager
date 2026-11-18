package issuerepo

import (
	"context"
	"encoding/json"
	"fmt"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
)

// AllPendingWorklogs returns every entry the journal holds for the profile,
// oldest first, each with the issue it was logged on. It is PendingWorklogs
// across issues, for the Worklog view, which draws a week of one person's
// time and so cannot ask issue by issue.
func (r *Repository) AllPendingWorklogs(ctx context.Context, profileID string) ([]backend.LoggedWork, error) {
	rows, err := r.ListPendingChanges(ctx, profileID)
	if err != nil {
		return nil, err
	}
	out := []backend.LoggedWork{}
	for _, p := range rows {
		if p.EntityType != EntityWorklog {
			continue
		}
		log, err := pendingWorklog(p)
		if err != nil {
			return nil, err
		}
		out = append(out, backend.LoggedWork{Key: p.EntityKey, Log: log})
	}
	return out, nil
}

// pendingWorklog reads a journal row back into the entry it holds, flagged
// pending with the row's id so it can be discarded.
func pendingWorklog(p journal.PendingChange) (backend.Worklog, error) {
	var d backend.WorklogDraft
	if err := json.Unmarshal([]byte(p.AfterVal), &d); err != nil {
		return backend.Worklog{}, fmt.Errorf("decode pending worklog %d: %w", p.ID, err)
	}
	return backend.Worklog{
		Started: d.Started, TimeSpent: d.TimeSpent, Seconds: d.Seconds, Comment: d.Comment,
		Pending: true, PendingID: p.ID,
	}, nil
}
