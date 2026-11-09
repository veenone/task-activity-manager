package main

import (
	goruntime "runtime"

	"agile-suite/core/store"
)

// Diagnostics is the readout Help > About shows, in the shape core's
// AboutDialog reads for TAM and XTM too (#450). PIE keeps no log file of its
// own, so LogPath is empty.
type Diagnostics struct {
	Version       string `json:"version"`
	DBPath        string `json:"dbPath"`
	LogPath       string `json:"logPath"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	GoVersion     string `json:"goVersion"`
	SchemaVersion int    `json:"schemaVersion"`
	ProfileCount  int    `json:"profileCount"`
	StartupError  string `json:"startupError"`
}

// GetDiagnostics answers even when the database failed to open, so About can
// say what broke.
func (a *App) GetDiagnostics() Diagnostics {
	d := Diagnostics{
		Version:      a.Version(),
		DBPath:       a.dbPath,
		OS:           goruntime.GOOS,
		Arch:         goruntime.GOARCH,
		GoVersion:    goruntime.Version(),
		StartupError: a.startupErr,
	}
	if a.db != nil {
		if v, err := store.ReadSchemaVersion(a.db.DB()); err == nil {
			d.SchemaVersion = v
		}
	}
	if a.profiles != nil {
		if ps, err := a.profiles.List(); err == nil {
			d.ProfileCount = len(ps)
		}
	}
	return d
}
