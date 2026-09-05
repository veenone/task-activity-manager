package issuerepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// SyncState is what the status bar and the sync engine need to know about a
// profile: when it last synced (RFC3339, empty when never), when it last did
// a full sync, the last error, and how many issues are cached.
type SyncState struct {
	LastSynced string `json:"lastSynced"`
	LastFull   string `json:"lastFull"`
	LastError  string `json:"lastError"`
	IssueCount int    `json:"issueCount"`
}

// SyncState reads the profile's state; a profile that never synced returns
// the zero value with the issue count filled in.
func (r *Repository) SyncState(ctx context.Context, profileID string) (SyncState, error) {
	var s SyncState
	err := r.db.QueryRowContext(ctx,
		`SELECT last_synced, last_full, last_error FROM sync_state WHERE profile_id = ?`, profileID,
	).Scan(&s.LastSynced, &s.LastFull, &s.LastError)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return SyncState{}, fmt.Errorf("sync state: %w", err)
	}
	n, err := r.CountIssues(ctx, profileID)
	if err != nil {
		return SyncState{}, err
	}
	s.IssueCount = n
	return s, nil
}

// SetSyncState writes the three timestamps and the error; IssueCount is
// derived and ignored on write.
func (r *Repository) SetSyncState(ctx context.Context, profileID string, s SyncState) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO sync_state (profile_id, last_synced, last_full, last_error) VALUES (?, ?, ?, ?)
		 ON CONFLICT(profile_id) DO UPDATE SET last_synced = excluded.last_synced, last_full = excluded.last_full, last_error = excluded.last_error`,
		profileID, s.LastSynced, s.LastFull, s.LastError)
	if err != nil {
		return fmt.Errorf("set sync state: %w", err)
	}
	return nil
}

// ProfileSetting returns the value stored for key under the profile, or ""
// when unset.
func (r *Repository) ProfileSetting(ctx context.Context, profileID, key string) (string, error) {
	var v string
	err := r.db.QueryRowContext(ctx,
		`SELECT value FROM profile_setting WHERE profile_id = ? AND key = ?`, profileID, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("profile setting %s: %w", key, err)
	}
	return v, nil
}

// SetProfileSetting stores value for key under the profile.
func (r *Repository) SetProfileSetting(ctx context.Context, profileID, key, value string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO profile_setting (profile_id, key, value) VALUES (?, ?, ?)
		 ON CONFLICT(profile_id, key) DO UPDATE SET value = excluded.value`, profileID, key, value)
	if err != nil {
		return fmt.Errorf("set profile setting %s: %w", key, err)
	}
	return nil
}

// ResetSyncCursor clears last_synced so the next sync pulls everything
// again. last_full and last_error are left alone.
func (r *Repository) ResetSyncCursor(ctx context.Context, profileID string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO sync_state (profile_id, last_synced, last_full, last_error) VALUES (?, '', '', '')
		 ON CONFLICT(profile_id) DO UPDATE SET last_synced = ''`, profileID)
	if err != nil {
		return fmt.Errorf("reset sync cursor: %w", err)
	}
	return nil
}

// PurgeProfile drops everything the local store holds for a profile: its
// links, issues, sync state, and settings. The profile row itself lives in
// the shared database, so deleting it there leaves these rows orphaned
// until this runs.
func (r *Repository) PurgeProfile(ctx context.Context, profileID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, table := range []string{"issue_link", "issue", "sync_state", "profile_setting"} {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE profile_id = ?`, profileID); err != nil {
			return fmt.Errorf("purge %s for %s: %w", table, profileID, err)
		}
	}
	return tx.Commit()
}
