package testrepo

import "fmt"

// SyncLogEntry records the outcome of one sync run (FR-1.7).
type SyncLogEntry struct {
	ID         int64  `json:"id"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt"`
	Outcome    string `json:"outcome"` // "success" | "error"
	Fetched    int    `json:"fetched"`
	Error      string `json:"error"`
}

// RecordSyncLog appends a sync run's outcome to the history (FR-1.7).
func (r *Repository) RecordSyncLog(profileID, startedAt, finishedAt, outcome string, fetched int, errMsg string) error {
	if _, err := r.db.Exec(
		`INSERT INTO sync_log (profile_id, started_at, finished_at, outcome, fetched, error)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		profileID, startedAt, finishedAt, outcome, fetched, errMsg,
	); err != nil {
		return fmt.Errorf("record sync log: %w", err)
	}
	return nil
}

// ListSyncLog returns a profile's most recent sync runs, newest first. A limit
// ≤ 0 or > 200 defaults to 50.
func (r *Repository) ListSyncLog(profileID string, limit int) ([]SyncLogEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.Query(
		`SELECT id, started_at, finished_at, outcome, fetched, error
		 FROM sync_log WHERE profile_id = ?
		 ORDER BY started_at DESC, id DESC LIMIT ?`,
		profileID, limit)
	if err != nil {
		return nil, fmt.Errorf("list sync log: %w", err)
	}
	defer rows.Close()

	out := []SyncLogEntry{}
	for rows.Next() {
		var e SyncLogEntry
		if err := rows.Scan(&e.ID, &e.StartedAt, &e.FinishedAt, &e.Outcome, &e.Fetched, &e.Error); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
