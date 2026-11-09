package main

import (
	"path/filepath"
	"testing"

	"agile-suite/core/settings"
	"agile-suite/core/store"
	"agile-suite/pie/internal/saved"
)

func themeApp(t *testing.T) *App {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "pie.db"), saved.Schema())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &App{settings: settings.NewManager(db.DB())}
}

func TestThemeIsSystemUntilSetAndRefusesAnUnknownOne(t *testing.T) {
	a := themeApp(t)
	if got := a.Theme(); got != "system" {
		t.Fatalf("theme before any choice = %q", got)
	}
	if err := a.SetTheme("blue"); err == nil {
		t.Fatal("an unknown theme was stored")
	}
	if err := a.SetTheme("dark"); err != nil {
		t.Fatal(err)
	}
	if got := a.Theme(); got != "dark" {
		t.Fatalf("theme = %q", got)
	}
}

func TestVersionIsWailsJSONsProductVersion(t *testing.T) {
	if got := productVersion([]byte(`{"info":{"productVersion":"0.1.0"}}`)); got != "0.1.0" {
		t.Fatalf("version = %q", got)
	}
	if got := (&App{}).Version(); got == "" {
		t.Fatal("the embedded wails.json gave no version")
	}
}
