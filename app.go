package main

import (
	"context"
	"errors"
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
	ctx        context.Context
	store      *store.Store
	profiles   *profile.Manager
	creds      profile.CredentialStore
	repo       *testrepo.Repository
	dbPath     string
	logPath    string
	startupErr string
}

// HealthInfo reports whether the backend initialised successfully. The
// frontend calls Health() first and surfaces any error so users see what
// actually failed instead of a blank screen or a cryptic nil-pointer panic.
type HealthInfo struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error"`
	DBPath  string `json:"dbPath"`
	LogPath string `json:"logPath"`
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{}
}

// startup wires the service layer. Failures are captured into startupErr
// instead of being swallowed — the GUI has no console on Windows, so
// without this they would be invisible.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// File logging in the app data dir so startup output is visible even
	// when launched without a console.
	if path, err := setupFileLogging(); err == nil {
		a.logPath = path
		log.Printf("xtm: starting up — log at %s", path)
	}

	if err := a.initStore(); err != nil {
		a.startupErr = err.Error()
		log.Printf("xtm: startup failed: %v", err)
	}
}

// initStore opens the local database and constructs the service objects.
func (a *App) initStore() error {
	dbPath, err := defaultDBPath()
	if err != nil {
		return fmt.Errorf("resolve database path: %w", err)
	}
	a.dbPath = dbPath

	st, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open local store at %s: %w", dbPath, err)
	}
	a.store = st
	a.profiles = profile.NewManager(st)
	a.creds = profile.NewCredentialStore()
	a.repo = testrepo.NewRepository(st)
	log.Printf("xtm: local store ready at %s", dbPath)
	return nil
}

// shutdown closes the local database when the window is closed.
func (a *App) shutdown(ctx context.Context) {
	if a.store != nil {
		if err := a.store.Close(); err != nil {
			log.Printf("xtm: close local store: %v", err)
		}
	}
}

// Health reports backend startup status. Always safe to call.
func (a *App) Health() HealthInfo {
	return HealthInfo{
		OK:      a.startupErr == "" && a.profiles != nil,
		Error:   a.startupErr,
		DBPath:  a.dbPath,
		LogPath: a.logPath,
	}
}

// requireStore is the guard every store-dependent bound method calls. It
// turns a backend init failure into a useful frontend error instead of a
// nil-pointer panic deep in the call chain.
func (a *App) requireStore() error {
	if a.startupErr != "" {
		return fmt.Errorf("local store unavailable: %s", a.startupErr)
	}
	if a.profiles == nil || a.repo == nil {
		return errors.New("local store not initialised")
	}
	return nil
}

// --- Profiles (FR-5) ---

// ListProfiles returns all configured connection profiles.
func (a *App) ListProfiles() ([]profile.Profile, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.profiles.List()
}

// CreateProfile stores a new profile and saves its PAT to the OS credential
// manager. The token is never written to the database.
func (a *App) CreateProfile(name, jiraURL, projectKey, token string) (profile.Profile, error) {
	if err := a.requireStore(); err != nil {
		return profile.Profile{}, err
	}
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
	if err := a.requireStore(); err != nil {
		return err
	}
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
// the authenticated user (FR-8.4). It does not depend on the local store —
// useful for diagnosing PAT issues even if the store failed to initialise.
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
	if err := a.requireStore(); err != nil {
		return err
	}
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
	if err := a.requireStore(); err != nil {
		return testrepo.SyncState{}, err
	}
	return a.repo.GetSyncState(profileID)
}

// --- Test Repository (FR-13) ---

// ListFolders returns the synced Test Repository folder tree for a profile.
func (a *App) ListFolders(profileID string) ([]testrepo.Folder, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.repo.ListFolders(profileID)
}

// GetTestPreconditions returns the Preconditions linked to a Test.
func (a *App) GetTestPreconditions(profileID, testKey string) ([]testrepo.Precondition, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.repo.ListTestPreconditions(profileID, testKey)
}

// --- Local editing & change tracking (FR-2 / FR-1.5 / FR-12.6) ---

// EditTestField applies a local edit to a Test field and queues a pending
// change for commit. Editable fields: summary, description, priority, labels.
// Repeated edits to the same field are coalesced; reverting to the original
// value drops the pending change.
func (a *App) EditTestField(profileID, testKey, field, newValue string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	return a.repo.EditTestField(profileID, testKey, field, newValue)
}

// DiscardPendingChange reverts a queued change and removes it from the
// pending list.
func (a *App) DiscardPendingChange(profileID string, changeID int64) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	return a.repo.DiscardPendingChange(profileID, changeID)
}

// ListPendingChanges returns all uncommitted local edits for a profile.
func (a *App) ListPendingChanges(profileID string) ([]testrepo.PendingChange, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.repo.ListPendingChanges(profileID)
}

// ListAuditEntries returns the most recent audit log entries for a profile
// (newest first). Defaults to 200 entries.
func (a *App) ListAuditEntries(profileID string, limit int) ([]testrepo.AuditEntry, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.repo.ListAuditEntries(profileID, limit)
}

// CommitPendingChanges pushes a profile's local edits to Jira (FR-1.5).
// Returns a per-Test result describing what succeeded and what failed —
// failed entries stay in the local pending list so the user can retry or
// discard them.
func (a *App) CommitPendingChanges(profileID string) (syncer.CommitResult, error) {
	empty := syncer.CommitResult{
		Succeeded:  []string{},
		Conflicted: []syncer.Conflict{},
		Failed:     []syncer.FailedCommit{},
	}
	if err := a.requireStore(); err != nil {
		return empty, err
	}
	p, err := a.profiles.Get(profileID)
	if err != nil {
		return empty, err
	}
	token, err := a.creds.Load(profileID)
	if err != nil {
		return empty, fmt.Errorf("load credentials: %w", err)
	}
	engine := syncer.New(jira.NewClient(p.JiraURL, token), a.repo)
	return engine.CommitChanges(a.ctx, profileID)
}

// --- Workflow transitions (FR-4.2) ---

// GetTestTransitions returns the workflow transitions available from a
// Test's current local status — used by the detail UI to populate the
// "Move to…" picker. Behind the scenes this reads the Test's current
// status from the local store and asks Jira (or the demo generator) what
// is reachable from there.
func (a *App) GetTestTransitions(profileID, testKey string) ([]jira.Transition, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	test, err := a.repo.GetTest(profileID, testKey)
	if err != nil {
		return nil, err
	}
	p, err := a.profiles.Get(profileID)
	if err != nil {
		return nil, err
	}
	token, err := a.creds.Load(profileID)
	if err != nil {
		return nil, fmt.Errorf("load credentials: %w", err)
	}
	return jira.NewClient(p.JiraURL, token).GetTransitions(a.ctx, testKey, test.Status)
}

// TransitionTest queues a workflow transition locally (FR-4.2). The change
// is pushed to Jira on commit via POST /rest/api/2/issue/{key}/transitions.
func (a *App) TransitionTest(profileID, testKey, targetStatus string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	return a.repo.TransitionTest(profileID, testKey, targetStatus)
}

// --- Bulk operations (FR-3) ---

// BulkEditTests applies a single field-level operation to a batch of Tests,
// queuing a pending change for each modified Test. The changes are then
// pushed to Jira through the existing commit flow.
func (a *App) BulkEditTests(profileID string, testKeys []string, op testrepo.BulkEdit) (testrepo.BulkEditResult, error) {
	empty := testrepo.BulkEditResult{
		Succeeded: []string{},
		Failed:    []testrepo.BulkFailure{},
	}
	if err := a.requireStore(); err != nil {
		return empty, err
	}
	return a.repo.BulkEditTests(profileID, testKeys, op)
}

// --- Browse (FR-11) ---

// ListTests returns a filtered, sorted, paginated page of Tests for a profile.
func (a *App) ListTests(profileID string, q testrepo.Query) (testrepo.Page, error) {
	if err := a.requireStore(); err != nil {
		return testrepo.Page{}, err
	}
	return a.repo.ListTests(profileID, q)
}

// GetTest returns one Test by its Jira key.
func (a *App) GetTest(profileID, key string) (testrepo.TestCase, error) {
	if err := a.requireStore(); err != nil {
		return testrepo.TestCase{}, err
	}
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

// setupFileLogging redirects the standard log output to a file in the app
// data dir so startup output is visible without a console. Failures here
// are non-fatal — file logging is purely for visibility.
func setupFileLogging() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(dir, "xray-test-manager")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return "", err
	}
	logPath := filepath.Join(appDir, "xtm.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	return logPath, nil
}
