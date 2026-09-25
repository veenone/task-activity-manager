package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"agile-suite/core/profile"
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

// askedFor stubs the save dialog and records the options the export opened
// it with, so a test can read the folder it started in and the name it
// prefilled without a window.
func askedFor(a *App, answer string) *runtime.SaveDialogOptions {
	asked := &runtime.SaveDialogOptions{}
	a.saveDialog = func(o runtime.SaveDialogOptions) (string, error) {
		*asked = o
		return answer, nil
	}
	return asked
}

func TestExportSprintReportWritesWhereTheDialogSaysAndStartsBesideTheDatabase(t *testing.T) {
	for _, tc := range []struct {
		name   string
		export func(*App) (string, error)
		ext    string
	}{
		{"xlsx", func(a *App) (string, error) { return a.ExportSprintReportXLSX(exportDoc()) }, ".xlsx"},
		{"pptx", func(a *App) (string, error) { return a.ExportSprintReportPPTX(exportDoc()) }, ".pptx"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := appWithDatabaseDir(t)
			chosen := filepath.Join(t.TempDir(), "wherever-i-like"+tc.ext)
			asked := askedFor(a, chosen)

			path, err := tc.export(a)
			if err != nil {
				t.Fatalf("export: %v", err)
			}
			if path != chosen {
				t.Errorf("path = %s, want the file the dialog chose %s", path, chosen)
			}
			if asked.DefaultDirectory != filepath.Dir(a.dbPath) {
				t.Errorf("the dialog started in %s, want the app data directory %s", asked.DefaultDirectory, filepath.Dir(a.dbPath))
			}
			if !strings.Contains(asked.DefaultFilename, "sprint-14") || !strings.HasSuffix(asked.DefaultFilename, tc.ext) {
				t.Errorf("the dialog prefilled %q, want a name carrying the report and ending %s", asked.DefaultFilename, tc.ext)
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

func TestExportSprintReportStartsInTheConfiguredFolder(t *testing.T) {
	a := appWithDatabaseDir(t)
	folder := t.TempDir()
	if err := a.SetReportExportDirectory(folder); err != nil {
		t.Fatalf("set the export folder: %v", err)
	}
	asked := askedFor(a, filepath.Join(folder, "report.xlsx"))

	if _, err := a.ExportSprintReportXLSX(exportDoc()); err != nil {
		t.Fatalf("export: %v", err)
	}
	if asked.DefaultDirectory != folder {
		t.Errorf("the dialog started in %s, want the configured folder %s", asked.DefaultDirectory, folder)
	}
}

// A folder that has gone (a drive unplugged, a directory deleted) must not
// stop the export: the dialog opens where it always did.
func TestExportSprintReportFallsBackWhenTheConfiguredFolderIsGone(t *testing.T) {
	a := appWithDatabaseDir(t)
	folder := filepath.Join(t.TempDir(), "gone")
	if err := a.settings.SetReportExportDir(folder); err != nil {
		t.Fatalf("store the export folder: %v", err)
	}
	asked := askedFor(a, filepath.Join(t.TempDir(), "report.xlsx"))

	if _, err := a.ExportSprintReportXLSX(exportDoc()); err != nil {
		t.Fatalf("export: %v", err)
	}
	if asked.DefaultDirectory != filepath.Dir(a.dbPath) {
		t.Errorf("the dialog started in %s, want the app data directory %s", asked.DefaultDirectory, filepath.Dir(a.dbPath))
	}
}

// A cancelled dialog is an answer, not a failure: nothing is written, no
// error is raised, and the empty path is what says so.
func TestExportSprintReportCancelledWritesNothingAndIsNotAnError(t *testing.T) {
	a := appWithDatabaseDir(t)
	dir := filepath.Dir(a.dbPath)
	askedFor(a, "")

	for _, export := range []func() (string, error){
		func() (string, error) { return a.ExportSprintReportXLSX(exportDoc()) },
		func() (string, error) { return a.ExportSprintReportPPTX(exportDoc()) },
	} {
		path, err := export()
		if err != nil {
			t.Errorf("a cancelled dialog reported an error: %v", err)
		}
		if path != "" {
			t.Errorf("path = %q, want empty for a cancelled dialog", path)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("a file was written although the dialog was cancelled: %v", entries)
	}
}

func TestSetReportExportDirectoryTakesAFolderAndRefusesAnythingElse(t *testing.T) {
	a := appWithDatabaseDir(t)
	folder := t.TempDir()
	file := filepath.Join(folder, "not-a-folder.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.SetReportExportDirectory(file); err == nil {
		t.Error("a file is not a folder and should be refused")
	}
	if err := a.SetReportExportDirectory(filepath.Join(folder, "nope")); err == nil {
		t.Error("a folder that is not there should be refused")
	}
	if err := a.SetReportExportDirectory("  " + folder + "  "); err != nil {
		t.Fatalf("a real folder was refused: %v", err)
	}
	s, err := a.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.ReportExportDir != folder {
		t.Errorf("stored %q, want the trimmed folder %q", s.ReportExportDir, folder)
	}
	if err := a.SetReportExportDirectory(""); err != nil {
		t.Fatalf("clearing the folder was refused: %v", err)
	}
	if s, err := a.GetSettings(); err != nil || s.ReportExportDir != "" {
		t.Errorf("after clearing, ReportExportDir = %q, %v", s.ReportExportDir, err)
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

// TestReportDestinationLeavesAnUnsetProfileWhereItWas is the backward
// compatibility this change turns on: with no reports space and no reports
// root, a report goes to the rituals space, under the sprint's own overview
// page, and to the rituals root when that sprint has no page yet.
func TestReportDestinationLeavesAnUnsetProfileWhereItWas(t *testing.T) {
	rituals := profile.ConfluenceConfig{SpaceKey: "TEAM", RootPageID: "10"}
	for _, tc := range []struct {
		name       string
		cfg        profile.ConfluenceConfig
		sprintPage string
		space      string
		parent     string
	}{
		{"nothing set, the sprint has a page", rituals, "555", "TEAM", "555"},
		{"nothing set, the sprint has none", rituals, "10", "TEAM", "10"},
		{
			"only a reports root: the rituals space, under that root",
			profile.ConfluenceConfig{SpaceKey: "TEAM", RootPageID: "10", ReportsRootPageID: "900"},
			"555", "TEAM", "900",
		},
		{
			"a reports space and root",
			profile.ConfluenceConfig{SpaceKey: "TEAM", RootPageID: "10", ReportsSpaceKey: "REPORTS", ReportsRootPageID: "900"},
			"555", "REPORTS", "900",
		},
		{
			// The sprint's ritual page is in the rituals space and cannot
			// parent a page in another one, so the report sits at the top.
			"a reports space with no root",
			profile.ConfluenceConfig{SpaceKey: "TEAM", RootPageID: "10", ReportsSpaceKey: "REPORTS"},
			"555", "REPORTS", "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			space, parent := reportDestination(tc.cfg, tc.sprintPage)
			if space != tc.space || parent != tc.parent {
				t.Errorf("destination = %s / %s, want %s / %s", space, parent, tc.space, tc.parent)
			}
		})
	}
}

func TestPublishSprintReportHangsUnderTheProfilesReportsRoot(t *testing.T) {
	a, p := newRitualSyncApp(t)
	if _, err := a.EnsureSprintRituals(p.ID, 1, 14); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SyncRituals(p.ID, 1); err != nil {
		t.Fatal(err)
	}
	root, err := a.CreateReportRoot(p.ID, "", "PLAT Reports", false)
	if err != nil {
		t.Fatalf("create the reports root: %v", err)
	}
	if root.Outcome != "created" || root.PageID == "" || !root.TopLevel {
		t.Fatalf("root = %+v, want a page created at the top of the space", root)
	}
	// The root is created, not saved: the profile form saves it with the rest.
	stored, err := a.profiles.ConfluenceConfig(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ReportsRootPageID != "" {
		t.Errorf("the reports root was saved behind the form's back: %q", stored.ReportsRootPageID)
	}
	stored.ReportsRootPageID = root.PageID
	if err := a.profiles.SetConfluenceConfig(p.ID, stored); err != nil {
		t.Fatal(err)
	}

	published, err := a.PublishSprintReport(p.ID, 1, 14, exportDoc())
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	page, ok := a.demoConfluence[p.ID].Page(published.PageID)
	if !ok {
		t.Fatalf("page %s is not in the space", published.PageID)
	}
	if len(page.AncestorIDs) == 0 || page.AncestorIDs[len(page.AncestorIDs)-1] != root.PageID {
		t.Errorf("the report page sits under %v, want the reports root %s", page.AncestorIDs, root.PageID)
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
