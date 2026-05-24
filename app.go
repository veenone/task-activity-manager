package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"xray-test-manager/internal/jira"
	"xray-test-manager/internal/profile"
	"xray-test-manager/internal/store"
	"xray-test-manager/internal/syncer"
	"xray-test-manager/internal/testrepo"
)

// App is the Wails application backend. Exported methods on App are bound and
// callable from the React frontend.
type App struct {
	ctx      context.Context
	store    *store.Store
	profiles *profile.Manager
	creds    profile.CredentialStore
	repo     *testrepo.Repository
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{}
}

// startup opens the local database and wires the service layer.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	dbPath, err := defaultDBPath()
	if err != nil {
		log.Printf("xtm: resolve database path: %v", err)
		return
	}
	st, err := store.Open(dbPath)
	if err != nil {
		log.Printf("xtm: open local store: %v", err)
		return
	}
	a.store = st
	a.profiles = profile.NewManager(st)
	a.creds = profile.NewCredentialStore()
	a.repo = testrepo.NewRepository(st)
	log.Printf("xtm: local store ready at %s", dbPath)
}

// shutdown closes the local database when the window is closed.
func (a *App) shutdown(ctx context.Context) {
	if a.store != nil {
		if err := a.store.Close(); err != nil {
			log.Printf("xtm: close local store: %v", err)
		}
	}
}

// --- Profiles (FR-5) ---

// ListProfiles returns all configured connection profiles.
func (a *App) ListProfiles() ([]profile.Profile, error) {
	return a.profiles.List()
}

// CreateProfile stores a new profile and saves its PAT to the OS credential
// manager. The token is never written to the database.
func (a *App) CreateProfile(name, jiraURL, projectKey, token string) (profile.Profile, error) {
	p, err := a.profiles.Create(name, jiraURL, projectKey)
	if err != nil {
		return profile.Profile{}, err
	}
	if err := a.creds.Save(p.ID, token); err != nil {
		_ = a.profiles.Delete(p.ID) // don't leave a credential-less profile behind
		return profile.Profile{}, fmt.Errorf("save credentials: %w", err)
	}
	return p, nil
}

// DeleteProfile removes a profile and its stored credentials.
func (a *App) DeleteProfile(id string) error {
	if err := a.profiles.Delete(id); err != nil {
		return err
	}
	if err := a.creds.Delete(id); err != nil {
		log.Printf("xtm: delete credentials for %s: %v", id, err)
	}
	return nil
}

// --- Connection & sync (FR-1, FR-8) ---

// TestConnection verifies a Jira URL and PAT, returning the display name of
// the authenticated user (FR-8.4).
func (a *App) TestConnection(jiraURL, token string) (string, error) {
	user, err := jira.NewClient(jiraURL, token).TestConnection(a.ctx)
	if err != nil {
		return "", err
	}
	return user.DisplayName, nil
}

// SyncProfile syncs a profile, emitting "sync:progress" events to the
// frontend as pages complete. The first sync (no watermark) is a full pull;
// subsequent syncs use the previous sync's timestamp as a watermark for an
// incremental fetch (FR-1.1 / FR-1.2).
func (a *App) SyncProfile(profileID string) error {
	p, err := a.profiles.Get(profileID)
	if err != nil {
		return err
	}
	token, err := a.creds.Load(profileID)
	if err != nil {
		return fmt.Errorf("load credentials: %w", err)
	}
	state, err := a.repo.GetSyncState(profileID)
	if err != nil {
		return fmt.Errorf("read sync state: %w", err)
	}
	engine := syncer.New(jira.NewClient(p.JiraURL, token), a.repo)
	return engine.Sync(a.ctx, profileID, p.ProjectKey, state.LastSyncedAt, func(pr syncer.Progress) {
		runtime.EventsEmit(a.ctx, "sync:progress", pr)
	})
}

// GetSyncState reports when a profile last synced and how many Tests it holds.
func (a *App) GetSyncState(profileID string) (testrepo.SyncState, error) {
	return a.repo.GetSyncState(profileID)
}

// --- Test Repository (FR-13) ---

// ListFolders returns the synced Test Repository folder tree for a profile.
func (a *App) ListFolders(profileID string) ([]testrepo.Folder, error) {
	return a.repo.ListFolders(profileID)
}

// GetTestPreconditions returns the Preconditions linked to a Test.
func (a *App) GetTestPreconditions(profileID, testKey string) ([]testrepo.Precondition, error) {
	return a.repo.ListTestPreconditions(profileID, testKey)
}

// --- Local editing & change tracking (FR-2 / FR-1.5 / FR-12.6) ---

// EditTestField applies a local edit to a Test field and queues a pending
// change for commit. Editable fields: summary, description, priority, labels.
// Repeated edits to the same field are coalesced; reverting to the original
// value drops the pending change.
func (a *App) EditTestField(profileID, testKey, field, newValue string) error {
	return a.repo.EditTestField(profileID, testKey, field, newValue)
}

// DiscardPendingChange reverts a queued change and removes it from the
// pending list.
func (a *App) DiscardPendingChange(profileID string, changeID int64) error {
	return a.repo.DiscardPendingChange(profileID, changeID)
}

// ListPendingChanges returns all uncommitted local edits for a profile.
func (a *App) ListPendingChanges(profileID string) ([]testrepo.PendingChange, error) {
	return a.repo.ListPendingChanges(profileID)
}

// ListAuditEntries returns the most recent audit log entries for a profile
// (newest first). Defaults to 200 entries.
func (a *App) ListAuditEntries(profileID string, limit int) ([]testrepo.AuditEntry, error) {
	return a.repo.ListAuditEntries(profileID, limit)
}

// --- Browse (FR-11) ---

// ListTests returns a filtered, sorted, paginated page of Tests for a profile.
func (a *App) ListTests(profileID string, q testrepo.Query) (testrepo.Page, error) {
	return a.repo.ListTests(profileID, q)
}

// GetTest returns one Test by its Jira key.
func (a *App) GetTest(profileID, key string) (testrepo.TestCase, error) {
	return a.repo.GetTest(profileID, key)
}

// defaultDBPath returns <user-config-dir>/xray-test-manager/xtm.db, creating
// the directory if needed.
func defaultDBPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(dir, "xray-test-manager")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(appDir, "xtm.db"), nil
}
