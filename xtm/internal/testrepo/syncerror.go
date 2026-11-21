package testrepo

import "fmt"

// SetSyncError records the failure of a profile's last sync, worded as the
// status bar shows it, without touching when it last synced or what it holds.
// The next SetSyncState clears it.
func (r *Repository) SetSyncError(profileID, message string) error {
	_, err := r.db.Exec(
		`INSERT INTO sync_state (profile_id, last_error) VALUES (?, ?)
		 ON CONFLICT(profile_id) DO UPDATE SET last_error = excluded.last_error`,
		profileID, message)
	if err != nil {
		return fmt.Errorf("set sync error: %w", err)
	}
	return nil
}
