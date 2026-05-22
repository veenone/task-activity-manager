package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"xray-test-manager/internal/profile"
	"xray-test-manager/internal/store"
)

// App is the Wails application backend. Exported methods on App are bound and
// callable from the React frontend.
type App struct {
	ctx      context.Context
	store    *store.Store
	profiles *profile.Manager
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{}
}

// startup opens the local database and wires the service layer (Phase 0).
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

// Greet returns a greeting for the given name.
//
// TODO(xtm): placeholder from the Wails template — replaced by real bound
// methods (profiles, sync, browse) as the frontend is built in Phase 1.
func (a *App) Greet(name string) string {
	return "Hello " + name + ", It's show time!"
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
