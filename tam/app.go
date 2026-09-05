package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"

	"agile-suite/core/profile"
	"agile-suite/core/settings"
	"agile-suite/core/shareddb"
	"agile-suite/core/store"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/suiteprofiles"
	"agile-suite/tam/internal/tamstore"
)

// App is the backend bound to the React frontend. Every exported method here
// is callable from JavaScript, so it only validates and delegates; the rules
// live in internal/.
type App struct {
	ctx        context.Context
	local      *store.DB
	shared     *store.DB
	profiles   *profile.Manager
	creds      profile.CredentialStore
	settings   *settings.Manager
	repo       *issuerepo.Repository
	backendMu  sync.Mutex
	backends   map[string]backend.IssueBackend
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
	}
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
	a.backends = map[string]backend.IssueBackend{}

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

// ListProfiles returns the shared profiles TAM can use.
func (a *App) ListProfiles() ([]profile.Profile, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	ps, err := a.profiles.List()
	if err != nil {
		return nil, err
	}
	return suiteprofiles.Visible(ps), nil
}

// CreateProfile adds a profile to the shared store and saves its token in the
// OS credential manager. A demo profile ("demo" as the URL) needs no token.
func (a *App) CreateProfile(name, jiraURL, projectKey, token string, makeDefault bool) (profile.Profile, error) {
	if err := a.requireStore(); err != nil {
		return profile.Profile{}, err
	}
	if err := suiteprofiles.ValidateNew(name, jiraURL, projectKey, token); err != nil {
		return profile.Profile{}, err
	}
	p, err := a.profiles.Create(
		strings.TrimSpace(name), strings.TrimSpace(jiraURL), strings.TrimSpace(projectKey),
		"", "", "", "", "", false, suiteprofiles.Backend,
	)
	if err != nil {
		return profile.Profile{}, err
	}
	if strings.TrimSpace(token) != "" {
		if err := a.creds.Save(p.ID, strings.TrimSpace(token)); err != nil {
			if delErr := a.profiles.Delete(p.ID); delErr != nil {
				log.Printf("tam: rollback profile %s after credential save failure: %v", p.ID, delErr)
			}
			return profile.Profile{}, fmt.Errorf("save credentials: %w", err)
		}
	}
	if makeDefault {
		if err := a.settings.SetDefaultProfileID(p.ID); err != nil {
			log.Printf("tam: set default profile after create: %v", err)
		}
	}
	return p, nil
}

// DeleteProfile removes a profile from the shared store, so it disappears
// from XTM as well, and drops its credential.
func (a *App) DeleteProfile(id string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	if err := a.profiles.Delete(id); err != nil {
		return err
	}
	if err := a.creds.Delete(id); err != nil {
		log.Printf("tam: delete credentials for %s: %v", id, err)
	}
	if s, err := a.settings.Get(); err == nil && s.DefaultProfileID == id {
		if err := a.settings.SetDefaultProfileID(""); err != nil {
			log.Printf("tam: clear default profile after delete: %v", err)
		}
	}
	return nil
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
