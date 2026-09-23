package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// logTailBytes bounds what ReadLog answers with. tam.log is appended to for
// the life of the install and nothing rotates it, so reading the file would
// be unbounded in both memory and in what the dialog tries to draw. 64 KiB is
// a few hundred entries, which reaches back past a failed sync and its cause
// on any machine that has not been idle for a week.
const logTailBytes = 64 << 10

// ReadLog returns the tail of the log TAM itself wrote. It takes no path: the
// file is the one setupFileLogging opened at startup, recorded in a.logPath,
// so this binding cannot be asked for any other file (I1). Nothing else in
// the app reads a file on the frontend's say-so, and this is not the place to
// start.
func (a *App) ReadLog() (string, error) {
	if a.logPath == "" {
		return "", errors.New("no log file is configured for this window")
	}
	f, err := os.Open(a.logPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("no log file has been written yet at %s", a.logPath)
		}
		return "", fmt.Errorf("read the log: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("read the log: %w", err)
	}
	size := info.Size()
	start := int64(0)
	if size > logTailBytes {
		start = size - logTailBytes
	}
	buf, err := io.ReadAll(io.NewSectionReader(f, start, size-start))
	if err != nil {
		return "", fmt.Errorf("read the log: %w", err)
	}

	text := string(buf)
	// A byte window lands mid-line, and half an entry reads as corruption.
	if start > 0 {
		if i := strings.IndexByte(text, '\n'); i >= 0 {
			text = text[i+1:]
		}
	}
	return strings.TrimRight(text, "\r\n"), nil
}

// ExportDiagnostics writes the environment summary and the recent log to a
// timestamped file beside the database, and returns its path, so a user can
// attach one file to a ticket instead of copying the dialog out by hand. It
// is XTM's ExportDiagnostics with TAM's two extra paths.
func (a *App) ExportDiagnostics() (string, error) {
	d := a.GetDiagnostics()
	// A log that cannot be read is worth exporting the summary without; the
	// reason takes the log's place in the file.
	logTail, err := a.ReadLog()
	if err != nil {
		logTail = err.Error()
	}

	var b strings.Builder
	fmt.Fprintln(&b, "Task Activity Manager diagnostics")
	fmt.Fprintf(&b, "Version:         %s\n", d.Version)
	fmt.Fprintf(&b, "Generated:       %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "OS / Arch:       %s / %s\n", d.OS, d.Arch)
	fmt.Fprintf(&b, "Go version:      %s\n", d.GoVersion)
	fmt.Fprintf(&b, "Schema version:  %d\n", d.SchemaVersion)
	fmt.Fprintf(&b, "Profiles:        %d\n", d.ProfileCount)
	fmt.Fprintf(&b, "Database:        %s\n", d.DBPath)
	fmt.Fprintf(&b, "Shared profiles: %s\n", d.SharedPath)
	fmt.Fprintf(&b, "Log file:        %s\n", d.LogPath)
	if d.StartupError != "" {
		fmt.Fprintf(&b, "Startup error:   %s\n", d.StartupError)
	}
	fmt.Fprintf(&b, "\n--- recent log ---\n%s\n", logTail)

	dir := filepath.Dir(a.dbPath)
	if dir == "" || dir == "." {
		return "", errors.New("no app data directory to export into")
	}
	path := filepath.Join(dir, fmt.Sprintf("tam-diagnostics-%d.txt", time.Now().Unix()))
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return "", fmt.Errorf("write the diagnostics file: %w", err)
	}
	return path, nil
}
