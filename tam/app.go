package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"agile-suite/core/profile"
	"agile-suite/core/settings"
	"agile-suite/core/shareddb"
	"agile-suite/core/store"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/tamstore"
)

// The issue repository satisfies boardrepo's read seam: this is the one
// place that imports both packages, so it is where the check belongs.
// Drift here would surface as the boards view compiling against a
// different repository than the one app_boards.go actually passes it.
var _ boardrepo.IssueSource = (*issuerepo.Repository)(nil)

// App is the backend bound to the React frontend. Every exported method here
// is callable from JavaScript, so it only validates and delegates; the rules
// live in internal/.
type App struct {
	ctx       context.Context
	local     *store.DB
	shared    *store.DB
	profiles  *profile.Manager
	creds     profile.CredentialStore
	settings  *settings.Manager
	repo      *issuerepo.Repository
	boards    *boardrepo.Repository
	backendMu sync.Mutex
	backends  map[string]backend.IssueBackend
	// busy names the operation running for a profile, one of "sync",
	// "commit", "import", "sprint", "boards refresh" and "report", so none
	// of them overlap; the frontend reducer mirrors this.
	busy map[string]string
	// reportCancels holds the cancel func of the sprint report running for
	// a profile, so the view can stop one it has walked away from. It is
	// guarded by backendMu, the same mutex busy is, and app_reports.go is
	// the only file that touches it.
	reportCancels map[string]context.CancelFunc

	dbPath     string
	sharedPath string
	logPath    string
	startupErr string
}

// HealthInfo tells the frontend whether startup succeeded. The Windows build
// has no console, so a failure has to reach the UI as data.
type HealthInfo struct {
	OK         bool   `json:"ok"`
	Error      string `json:"error"`
	DBPath     string `json:"dbPath"`
	SharedPath string `json:"sharedPath"`
	LogPath    string `json:"logPath"`
}

// Diagnostics is the environment summary shown in the About dialog.
type Diagnostics struct {
	Version       string `json:"version"`
	DBPath        string `json:"dbPath"`
	SharedPath    string `json:"sharedPath"`
	LogPath       string `json:"logPath"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	GoVersion     string `json:"goVersion"`
	SchemaVersion int    `json:"schemaVersion"`
	ProfileCount  int    `json:"profileCount"`
	StartupError  string `json:"startupError"`
}

// NewApp creates the application struct.
func NewApp() *App { return &App{} }

// startup wires the stores. Failures are recorded in startupErr rather than
// returned, so Health can report them.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if path, err := setupFileLogging(); err == nil {
		a.logPath = path
		log.Printf("tam: starting up, log at %s", path)
	}
	if err := a.initStore(); err != nil {
		a.startupErr = err.Error()
		log.Printf("tam: startup failed: %v", err)
		return
	}
	a.refreshMenu()
}

func (a *App) initStore() error {
	dbPath, err := tamstore.DefaultPath()
	if err != nil {
		return fmt.Errorf("resolve database path: %w", err)
	}
	local, err := tamstore.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open local store at %s: %w", dbPath, err)
	}
	a.local = local
	a.dbPath = dbPath
	a.repo = issuerepo.New(local.DB())
	a.boards = boardrepo.New(local.DB())
	a.backends = map[string]backend.IssueBackend{}
	a.busy = map[string]string{}

	sharedPath, err := shareddb.DefaultPath()
	if err != nil {
		return fmt.Errorf("resolve shared profile database path: %w", err)
	}
	shared, err := shareddb.Open(sharedPath)
	if err != nil {
		return fmt.Errorf("open shared profile database at %s: %w", sharedPath, err)
	}
	a.shared = shared
	a.sharedPath = sharedPath

	a.profiles = profile.NewManager(shared.DB())
	a.creds = profile.NewCredentialStore()
	a.settings = settings.NewManager(shared.DB())
	log.Printf("tam: local store ready at %s; shared profiles at %s", dbPath, sharedPath)
	return nil
}

func (a *App) shutdown(ctx context.Context) {
	if a.shared != nil {
		if err := a.shared.Close(); err != nil {
			log.Printf("tam: close shared profile database: %v", err)
		}
	}
	if a.local != nil {
		if err := a.local.Close(); err != nil {
			log.Printf("tam: close local store: %v", err)
		}
	}
}

func (a *App) requireStore() error {
	if a.startupErr != "" {
		return fmt.Errorf("shared profile store unavailable: %s", a.startupErr)
	}
	if a.profiles == nil {
		return errors.New("shared profile store not initialised")
	}
	return nil
}

// Health reports startup status. Always safe to call.
func (a *App) Health() HealthInfo {
	return HealthInfo{
		OK:         a.startupErr == "" && a.profiles != nil,
		Error:      a.startupErr,
		DBPath:     a.dbPath,
		SharedPath: a.sharedPath,
		LogPath:    a.logPath,
	}
}

// GetDiagnostics returns the environment summary. Safe to call even when the
// store failed to open.
func (a *App) GetDiagnostics() Diagnostics {
	d := Diagnostics{
		Version:      productVersion(),
		DBPath:       a.dbPath,
		SharedPath:   a.sharedPath,
		LogPath:      a.logPath,
		OS:           goruntime.GOOS,
		Arch:         goruntime.GOARCH,
		GoVersion:    goruntime.Version(),
		StartupError: a.startupErr,
	}
	if a.local != nil {
		if v, err := store.ReadSchemaVersion(a.local.DB()); err == nil {
			d.SchemaVersion = v
		}
	}
	if a.profiles != nil {
		if ps, err := a.ListProfiles(); err == nil {
			d.ProfileCount = len(ps)
		}
	}
	return d
}

// GetSettings returns the shared preferences.
func (a *App) GetSettings() (settings.Settings, error) {
	if err := a.requireStore(); err != nil {
		return settings.Settings{}, err
	}
	return a.settings.Get()
}

// SetTheme stores the colour theme: "light", "dark", or "system". Both apps
// read it.
func (a *App) SetTheme(theme string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	switch theme {
	case "light", "dark", "system":
		return a.settings.SetTheme(theme)
	}
	return fmt.Errorf("unknown theme %q", theme)
}

// showNavRail reads the stored nav-rail preference, defaulting to hidden when
// the store is not up yet. main() builds the menu before startup runs, so the
// first menu is built on that default and startup rebuilds it once the real
// value is readable.
func (a *App) showNavRail() bool {
	if a.settings == nil {
		return false
	}
	s, err := a.settings.Get()
	if err != nil {
		return false
	}
	return s.ShowNavRail
}

// setShowNavRail persists the nav-rail preference and tells the frontend, so
// the View menu's checkbox and the rail itself never disagree.
func (a *App) setShowNavRail(v bool) {
	if a.settings != nil {
		if err := a.settings.SetShowNavRail(v); err != nil {
			log.Printf("tam: save nav rail preference: %v", err)
		}
	}
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "menu:nav-rail", v)
	}
}

// SetNavRailVisible is the bound counterpart of the menu's checkbox, for the
// in-app control. It persists the preference and rebuilds the menu so the tick
// follows a toggle made from either side.
func (a *App) SetNavRailVisible(v bool) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	if err := a.settings.SetShowNavRail(v); err != nil {
		return err
	}
	a.refreshMenu()
	return nil
}

// refreshMenu rebuilds the native menu from the current settings. Wails renders
// a checkbox's tick from the value the item was built with, so a preference
// change has to rebuild rather than mutate.
func (a *App) refreshMenu() {
	if a.ctx == nil {
		return
	}
	runtime.MenuSetApplicationMenu(a.ctx, appMenu(a))
	runtime.MenuUpdateApplicationMenu(a.ctx)
}

// SetDefaultProfile records which profile opens on launch. An empty id clears
// it.
func (a *App) SetDefaultProfile(id string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	return a.settings.SetDefaultProfileID(id)
}

// setupFileLogging sends the standard logger to tam.log in the app data
// directory so startup output survives without a console.
func setupFileLogging() (string, error) {
	dir, err := tamstore.DefaultDir()
	if err != nil {
		return "", err
	}
	logPath := filepath.Join(dir, "tam.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	return logPath, nil
}
