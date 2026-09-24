package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agile-suite/tam/internal/reportout"
	"agile-suite/tam/internal/ritualrepo"
	"agile-suite/tam/internal/ritualtemplate"
)

func exportDoc() reportout.Document {
	return reportout.Document{
		Title: "Sprint 14 · Report",
		Sections: []reportout.Section{{
			Heading: "Sprint outcome",
			Lines:   []string{"Closed sprint · final results"},
			Table:   reportout.Table{Columns: []string{"Figure", "Amount"}, Rows: [][]string{{"Committed", "34 points"}}},
			Notes:   []string{"Committed is a minimum estimate."},
		}},
	}
}

func appWithDatabaseDir(t *testing.T) *App {
	t.Helper()
	a, _ := newRitualSyncApp(t)
	a.dbPath = filepath.Join(t.TempDir(), "tam.db")
	return a
}

func TestExportSprintReportWritesBesideTheDatabaseAndSaysWhere(t *testing.T) {
	a := appWithDatabaseDir(t)
	for _, tc := range []struct {
		name   string
		export func() (string, error)
		ext    string
	}{
		{"xlsx", func() (string, error) { return a.ExportSprintReportXLSX(exportDoc()) }, ".xlsx"},
		{"pptx", func() (string, error) { return a.ExportSprintReportPPTX(exportDoc()) }, ".pptx"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, err := tc.export()
			if err != nil {
				t.Fatalf("export: %v", err)
			}
			if filepath.Dir(path) != filepath.Dir(a.dbPath) {
				t.Errorf("wrote to %s, want the database's own directory %s", filepath.Dir(path), filepath.Dir(a.dbPath))
			}
			if filepath.Ext(path) != tc.ext {
				t.Errorf("path = %s, want a %s file", path, tc.ext)
			}
			if !strings.Contains(filepath.Base(path), "sprint-14") {
				t.Errorf("the file name %s does not say which report it holds", filepath.Base(path))
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read back: %v", err)
			}
			if _, err := zip.NewReader(strings.NewReader(string(data)), int64(len(data))); err != nil {
				t.Errorf("the file on disk is not a readable package: %v", err)
			}
		})
	}
}

func TestExportSprintReportRefusesAReportThatIsNotThere(t *testing.T) {
	a := appWithDatabaseDir(t)
	dir := filepath.Dir(a.dbPath)
	if _, err := a.ExportSprintReportXLSX(reportout.Document{}); err == nil {
		t.Error("an empty report should be refused")
	}
	if _, err := a.ExportSprintReportPPTX(reportout.Document{}); err == nil {
		t.Error("an empty report should be refused")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("a file was written for a report that does not exist: %v", entries)
	}
}

func TestPublishSprintReportWritesUnderTheSprintsOwnPage(t *testing.T) {
	a, p := newRitualSyncApp(t)
	if _, err := a.EnsureSprintRituals(p.ID, 1, 14); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SyncRituals(p.ID, 1); err != nil {
		t.Fatal(err)
	}
	published, err := a.PublishSprintReport(p.ID, 1, 14, exportDoc())
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if published.Title != "Sprint 14 · Report" {
		t.Errorf("title = %q", published.Title)
	}
	page, ok := a.demoConfluence[p.ID].Page(published.PageID)
	if !ok {
		t.Fatalf("page %s is not in the space", published.PageID)
	}
	if !strings.Contains(page.Body, "34 points") || !strings.Contains(page.Body, "Committed is a minimum estimate.") {
		t.Errorf("the figures or their caveat did not reach the page:\n%s", page.Body)
	}
	overview, _, err := a.rituals.Document(a.ctx, mustRitualKey(t, p.ID, 1, 14, ritualtemplate.Sprint))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.AncestorIDs) == 0 || page.AncestorIDs[len(page.AncestorIDs)-1] != overview.PageID {
		t.Errorf("the report page sits under %v, want the sprint's own page %s", page.AncestorIDs, overview.PageID)
	}
}

func TestPublishSprintReportRefusesAReportThatIsNotThere(t *testing.T) {
	a, p := newRitualSyncApp(t)
	if _, err := a.PublishSprintReport(p.ID, 1, 14, reportout.Document{}); err == nil {
		t.Fatal("an empty report should be refused")
	}
	if len(a.demoConfluence) != 0 {
		t.Error("a Confluence transport was reached for a report that does not exist")
	}
}

func mustRitualKey(t *testing.T, profileID string, boardID, sprintID int, ritualType string) ritualrepo.Key {
	t.Helper()
	k, err := ritualKey(profileID, boardID, sprintID, ritualType)
	if err != nil {
		t.Fatal(err)
	}
	return k
}
