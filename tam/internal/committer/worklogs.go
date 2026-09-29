package committer

import (
	"context"
	"encoding/json"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// Logged is a worklog entry Commit pushed.
type Logged struct {
	Key       string `json:"key"`
	TimeSpent string `json:"timeSpent"`
}

// commitWorklogs pushes every journalled entry, one row at a time with its
// own journal delete. One row at a time is the point rather than a
// simplification: an entry is somebody's record of an hour of their day, so a
// refusal on one takes that row alone and leaves the rest of the morning
// pending for the next Commit.
//
// An entry under a key this Commit could not create is held, and one still
// under a draft key is left for next time, which is exactly what the links
// pass does. Nothing needs rewriting for a draft that was created: the rekey
// renames every journal row of the old key, worklogs among them, so by the
// time this phase runs the row names the key Jira gave.
func (r *commitRun) pushWorklogs(ctx context.Context) {
	w, canLog := r.e.b.(backend.WorklogBackend)
	for _, p := range r.rows {
		if p.EntityType != issuerepo.EntityWorklog {
			continue
		}
		if waits, held := r.deps.blockedBy(p.EntityKey); held {
			r.deps.hold(r.res, p.EntityKey, issuerepo.EntityWorklog, p.ID, waits)
			continue
		}
		if isDraftKey(p.EntityKey) {
			continue
		}
		var d backend.WorklogDraft
		if err := json.Unmarshal([]byte(p.AfterVal), &d); err != nil {
			// A row nobody can decode will not decode next time either.
			r.res.Failures = append(r.res.Failures, worklogFailure(p, "the worklog could not be decoded: "+err.Error(), false))
			continue
		}
		if !canLog {
			r.res.Failures = append(r.res.Failures, worklogFailure(p, "this connection cannot log work", false))
			continue
		}
		if err := w.AddWorklog(ctx, p.EntityKey, d); err != nil {
			r.res.Failures = append(r.res.Failures, worklogFailure(p, err.Error(), true))
			continue
		}
		if err := r.e.repo.MarkCommitted(ctx, r.profileID, []journal.PendingChange{p}); err != nil {
			r.res.Failures = append(r.res.Failures, worklogFailure(p, "logged in Jira but the journal could not be cleared: "+err.Error(), true))
			continue
		}
		r.res.Logged = append(r.res.Logged, Logged{Key: p.EntityKey, TimeSpent: d.TimeSpent})
	}
}

// worklogFailure is one entry that did not land, carrying its row id so the
// dialog can offer to discard exactly that entry and no other.
func worklogFailure(p journal.PendingChange, message string, retryable bool) Failure {
	return Failure{Key: p.EntityKey, EntityType: issuerepo.EntityWorklog, RowID: p.ID, Error: message, Retryable: retryable, Reachable: []string{}}
}
