package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/xuri/excelize/v2"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// seedBacklog puts forty rows in the store, more than a page holds, so a
// test can tell the filter's rows from the page's.
func seedBacklog(t *testing.T, a *App, profileID string) {
	t.Helper()
	rows := make([]backend.Issue, 0, 40)
	for i := 1; i <= 40; i++ {
		kind := backend.TypeTask
		if i%2 == 0 {
			kind = backend.TypeBug
		}
		rows = append(rows, backend.Issue{
			Key: "PLAT-" + itoa(i), ID: itoa(i), Project: "PLAT",
			Type: kind, Summary: "Row " + itoa(i), Status: "To Do", Updated: "2026-09-01T00:00:00Z",
		})
	}
	if err := a.repo.UpsertPage(context.Background(), profileID, rows, time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}

// The export is of the filter, not of the page the grid is showing, and
// it lands where the dialog says.
func TestExportBacklogWritesEveryFilteredRowWhereTheDialogSays(t *testing.T) {
	a := appWithDatabaseDir(t)
	p := newTestProfile(t, a)
	seedBacklog(t, a, p.ID)
	chosen := filepath.Join(t.TempDir(), "backlog.xlsx")
	asked := askedFor(a, chosen)

	path, err := a.ExportBacklog(p.ID, issuerepo.IssueQuery{Types: []string{backend.TypeBug}, Limit: 25})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if path != chosen {
		t.Fatalf("path = %s, want %s", path, chosen)
	}
	if !strings.Contains(asked.Title, "backlog") {
		t.Errorf("dialog title = %q, want it to name what is being saved", asked.Title)
	}
	if !strings.HasPrefix(asked.DefaultFilename, "tam-backlog-") {
		t.Errorf("prefilled name = %q, want a backlog's own name rather than a report's", asked.DefaultFilename)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the file: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open the workbook: %v", err)
	}
	defer f.Close()
	rows, err := f.GetRows("Backlog")
	if err != nil {
		t.Fatalf("read rows: %v", err)
	}
	// Twenty bugs and a header: the page size of 25 narrows nothing, and
	// the twenty tasks the filter excluded are not in the file.
	if len(rows) != 21 {
		t.Errorf("rows = %d, want the header and the twenty bugs the filter matched", len(rows))
	}
}

// Cancelling writes nothing and reports nothing: the user said no.
func TestCancellingTheBacklogExportWritesNothing(t *testing.T) {
	a := appWithDatabaseDir(t)
	p := newTestProfile(t, a)
	seedBacklog(t, a, p.ID)
	dir := t.TempDir()
	a.saveDialog = func(runtime.SaveDialogOptions) (string, error) { return "", nil }

	path, err := a.ExportBacklog(p.ID, issuerepo.IssueQuery{})
	if err != nil {
		t.Fatalf("a cancelled export must not be an error: %v", err)
	}
	if path != "" {
		t.Errorf("path = %q, want nothing", path)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("the cancelled export left %d files behind", len(entries))
	}
}
