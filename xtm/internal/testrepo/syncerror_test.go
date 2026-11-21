package testrepo_test

import "testing"

// The last sync's failure is kept with the sync state, so the status bar can
// still say it after a restart, the way TAM's does (#542). A later successful
// sync clears it.
func TestSyncErrorLastsUntilTheNextSuccessfulSync(t *testing.T) {
	repo := newRepo(t)

	if err := repo.SetSyncError("p1", "Jira refused the token"); err != nil {
		t.Fatalf("set error: %v", err)
	}
	state, err := repo.GetSyncState("p1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if state.LastError != "Jira refused the token" {
		t.Errorf("LastError = %q, want the failure", state.LastError)
	}
	if state.LastSyncedAt != "" {
		t.Errorf("LastSyncedAt = %q, want empty: a failed first sync is not a sync", state.LastSyncedAt)
	}

	if err := repo.SetSyncState("p1"); err != nil {
		t.Fatalf("set state: %v", err)
	}
	state, err = repo.GetSyncState("p1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if state.LastError != "" {
		t.Errorf("LastError = %q after a successful sync, want empty", state.LastError)
	}

	if err := repo.SetSyncError("p1", "Jira timed out"); err != nil {
		t.Fatalf("set error: %v", err)
	}
	state, err = repo.GetSyncState("p1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if state.LastError != "Jira timed out" || state.LastSyncedAt == "" {
		t.Errorf("state = %+v, want the new failure and the earlier sync time kept", state)
	}
}
