package issuerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
)

// LogWork journals an entry against an issue the cache holds, a draft
// included: nothing reaches Jira until Commit pushes it. The duration is put
// through backend.ParseWorkSeconds again here rather than trusted from the
// caller, because the value started as text somebody typed and this is the
// last boundary before it is stored (I1).
//
// The row's field is the draft's started stamp. The journal is unique on
// entity type, key and field, so a fixed field would make a second entry on
// the same issue overwrite the first, and a worklog is a record of somebody's
// day rather than a value with one current setting.
func (r *Repository) LogWork(ctx context.Context, profileID, key string, d backend.WorklogDraft) error {
	if _, err := backend.ParseWorkSeconds(d.TimeSpent); err != nil {
		return err
	}
	if d.Started == "" {
		return errors.New("the worklog has no start time")
	}
	encoded, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("encode worklog: %w", err)
	}
	return r.inTx(ctx, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM issue WHERE profile_id = ? AND key = ?`, profileID, key).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return fmt.Errorf("check %s exists: %w", key, err)
		}
		if err := journal.Put(tx, profileID, EntityWorklog, key, d.Started, "", string(encoded), ""); err != nil {
			return err
		}
		return journal.Audit(tx, profileID, EntityWorklog, key, "worklog", d.Started, "", d.TimeSpent, d.Comment)
	})
}

// PendingWorklogs returns the entries the journal holds for key, oldest
// first, as Worklog rows flagged pending with their journal ids. The panel
// draws them beside the ones Jira answered with, which is the only place the
// two kinds meet.
func (r *Repository) PendingWorklogs(ctx context.Context, profileID, key string) ([]backend.Worklog, error) {
	rows, err := journal.ListForKey(r.db, profileID, key)
	if err != nil {
		return nil, err
	}
	logs := []backend.Worklog{}
	for _, p := range rows {
		if p.EntityType != EntityWorklog {
			continue
		}
		var d backend.WorklogDraft
		if err := json.Unmarshal([]byte(p.AfterVal), &d); err != nil {
			return nil, fmt.Errorf("decode pending worklog %d: %w", p.ID, err)
		}
		logs = append(logs, backend.Worklog{
			Started: d.Started, TimeSpent: d.TimeSpent, Seconds: d.Seconds, Comment: d.Comment,
			Pending: true, PendingID: p.ID,
		})
	}
	return logs, nil
}
