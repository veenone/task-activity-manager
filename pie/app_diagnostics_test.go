package main

import (
	"path/filepath"
	"runtime"
	"testing"

	"agile-suite/core/profile"
	"agile-suite/core/store"
	"agile-suite/pie/internal/saved"
)

// About reads PIE's runtime from GetDiagnostics, the readout core's
// AboutDialog shows for TAM and XTM too (#450).
func TestDiagnosticsNameTheVersionRuntimeDatabaseAndProfiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pie.db")
	db, err := store.Open(path, saved.Schema())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	a := &App{db: db, dbPath: path, profiles: profile.NewManager(db.DB())}
	if _, err := a.profiles.Create("Q", "demo", "", "", "", "", "", "", false, ""); err != nil {
		t.Fatal(err)
	}
	d := a.GetDiagnostics()
	if d.Version == "" || d.DBPath != path || d.OS != runtime.GOOS || d.GoVersion != runtime.Version() || d.ProfileCount != 1 || d.SchemaVersion == 0 {
		t.Fatalf("diagnostics = %+v", d)
	}
	if got := (&App{startupErr: "no disk"}).GetDiagnostics(); got.StartupError != "no disk" {
		t.Fatalf("a failed start reads %+v", got)
	}
}
