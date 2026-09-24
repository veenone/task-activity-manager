package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeLog fills a log file with numbered lines and returns its path.
func writeLog(t *testing.T, dir string, lines int) string {
	t.Helper()
	path := filepath.Join(dir, "tam.log")
	var b strings.Builder
	for i := 0; i < lines; i++ {
		fmt.Fprintf(&b, "2026/09/23 10:00:00.000000 tam: line %s %d\n", strings.Repeat("x", 80), i)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	return path
}

// TestReadLogReturnsTheEndOfTheLogNotTheWholeFile pins the bound: tam.log
// grows for the life of the install, so the reader answers with its tail and
// never the file. The first line it returns is a whole line, not the half of
// one the byte window cut.
func TestReadLogReturnsTheEndOfTheLogNotTheWholeFile(t *testing.T) {
	dir := t.TempDir()
	a := &App{logPath: writeLog(t, dir, 2000)}

	out, err := a.ReadLog()
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if len(out) > logTailBytes {
		t.Errorf("returned %d bytes, want at most %d", len(out), logTailBytes)
	}
	if !strings.HasSuffix(out, "1999") {
		t.Errorf("tail does not end at the last line: %q", out[max(0, len(out)-40):])
	}
	if strings.Contains(out, " 0\n") {
		t.Error("the head of the log came back too, so this is not a tail")
	}
	first, _, _ := strings.Cut(out, "\n")
	if !strings.HasPrefix(first, "2026/09/23") {
		t.Errorf("first line is a fragment of one: %q", first)
	}
}

// TestReadLogSaysPlainlyWhenThereIsNoLogToRead covers the two states the
// dialog has to explain rather than render as an empty pane.
func TestReadLogSaysPlainlyWhenThereIsNoLogToRead(t *testing.T) {
	missing := &App{logPath: filepath.Join(t.TempDir(), "tam.log")}
	if _, err := missing.ReadLog(); err == nil || !strings.Contains(err.Error(), "no log file") {
		t.Errorf("missing log error = %v, want one saying there is no log file yet", err)
	}

	none := &App{}
	if _, err := none.ReadLog(); err == nil || !strings.Contains(err.Error(), "no log file") {
		t.Errorf("unconfigured log error = %v, want one saying there is no log file", err)
	}
}

// TestReadLogTakesNoArgument is the trust boundary (I1). The reader answers
// with the log TAM itself opened at startup; a path parameter would make the
// binding a general file reader for anything the frontend asked for.
func TestReadLogTakesNoArgument(t *testing.T) {
	m, ok := reflect.TypeOf(&App{}).MethodByName("ReadLog")
	if !ok {
		t.Fatal("ReadLog is not a bound method")
	}
	if n := m.Type.NumIn(); n != 1 {
		t.Errorf("ReadLog takes %d arguments beside the receiver, want none", n-1)
	}
}

// TestExportDiagnosticsWritesTheSummaryAndTheLogBesideTheDatabase pins what
// a user attaches to a ticket: the paths, the build, and the recent log, in
// the app data directory the database already lives in.
func TestExportDiagnosticsWritesTheSummaryAndTheLogBesideTheDatabase(t *testing.T) {
	dir := t.TempDir()
	a := &App{
		dbPath:     filepath.Join(dir, "tam.db"),
		sharedPath: filepath.Join(dir, "profiles.db"),
		logPath:    writeLog(t, dir, 3),
	}

	path, err := a.ExportDiagnostics()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if filepath.Dir(path) != dir {
		t.Errorf("exported to %s, want a file in %s", path, dir)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	body := string(data)
	for _, want := range []string{a.dbPath, a.sharedPath, a.logPath, "tam: line", "Task Activity Manager"} {
		if !strings.Contains(body, want) {
			t.Errorf("export does not carry %q:\n%s", want, body)
		}
	}
}
