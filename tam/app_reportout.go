package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"agile-suite/core/profile"
	"agile-suite/tam/internal/errtext"
	"agile-suite/tam/internal/reportout"
	"agile-suite/tam/internal/ritualrepo"
	"agile-suite/tam/internal/ritualsync"
	"agile-suite/tam/internal/ritualtemplate"
)

// The sprint report's three outputs: a Confluence page, a spreadsheet and a
// deck. All three take the document the Reports view built, because the
// report's wording lives in the frontend and nothing here words any of it;
// lib/reportDocument.ts says why. None of them reaches Jira, and there is
// no code path here that could.
//
// Each is the thin adapter app*.go files are meant to be: resolve the
// profile, check what arrived, hand internal/reportout the work, and say
// what happened in one line.

// PublishSprintReport writes a sprint's report to its own Confluence page,
// where the profile's reports configuration says, and, when it says nothing,
// under the sprint's overview page or at the rituals root as it always did.
//
// It is a write, so it happens when the user asks and never on a view's
// mount, and it reports the page it wrote. It takes the profile's lock
// under "report" because it is a report operation; a sync or a commit
// refused while it runs says so.
func (a *App) PublishSprintReport(profileID string, boardID, sprintID int, doc reportout.Document) (reportout.Published, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return reportout.Published{}, err
	}
	// Checked before the transport is built, so a report that does not exist
	// costs no credential read and touches no Confluence at all.
	if err := doc.Check(); err != nil {
		return reportout.Published{}, err
	}
	if err := a.requireRituals(); err != nil {
		return reportout.Published{}, err
	}
	cfg, pages, err := a.confluencePages(p)
	if err != nil {
		return reportout.Published{}, err
	}
	if err := a.acquire(p.ID, "report"); err != nil {
		return reportout.Published{}, err
	}
	defer a.release(p.ID)

	space, parent := reportDestination(cfg, a.sprintPageID(p.ID, boardID, sprintID, cfg.RootPageID))
	published, err := reportout.Publish(a.ctx, pages, space, parent, doc)
	if err != nil {
		log.Printf("tam: publishing the report for sprint %d on board %d for %s failed: %v", sprintID, boardID, p.Name, err)
		return reportout.Published{}, err
	}
	log.Printf("tam: report for sprint %d on board %d for %s published to page %s (%s)", sprintID, boardID, p.Name, published.PageID, published.Title)
	return published, nil
}

// reportDestination is the space a report is published to and the page it
// hangs under. A profile that set neither a reports space nor a reports root
// lands exactly where it did before they existed: the rituals space, under
// the sprint's own overview page or the rituals root, whichever sprintPageID
// found.
//
// A reports space of its own with no root chosen puts the report at the top of
// that space: the sprint's ritual page is in another space and cannot be its
// parent.
func reportDestination(cfg profile.ConfluenceConfig, sprintPage string) (space, parent string) {
	rituals := strings.TrimSpace(cfg.SpaceKey)
	space = strings.TrimSpace(cfg.ReportsSpaceKey)
	if space == "" {
		space = rituals
	}
	switch root := strings.TrimSpace(cfg.ReportsRootPageID); {
	case root != "":
		return space, root
	case space == rituals:
		return space, sprintPage
	default:
		return space, ""
	}
}

// CreateReportRoot creates, or adopts, a top-level page for a profile's sprint
// reports to hang under, the way the missing-rituals-root dialog creates the
// rituals one. An empty space key means the rituals space, which is where an
// unconfigured profile publishes.
//
// It writes nothing locally: the page id goes back to the profile form, and
// the form saves it with the rest of the profile. Forbidden and a taken title
// come back as the outcome, with no page made.
func (a *App) CreateReportRoot(profileID, spaceKey, title string, adopt bool) (ritualsync.Root, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return ritualsync.Root{}, err
	}
	cfg, pages, err := a.confluencePages(p)
	if err != nil {
		return ritualsync.Root{}, err
	}
	// The space key is typed into a form, so it is trimmed here rather than
	// sent to Confluence with whatever whitespace came with it.
	space := strings.TrimSpace(spaceKey)
	if space == "" {
		space = cfg.SpaceKey
	}
	root, err := ritualsync.CreateRoot(a.ctx, pages, space, ritualtemplate.ReportRootBody(p.ProjectKey), title, adopt)
	if err != nil {
		log.Printf("tam: reports root for %s in %s refused: %v", p.ID, space, err)
		return ritualsync.Root{}, errors.New(errtext.Line(err))
	}
	log.Printf("tam: reports root for %s in %s: %s (page %s)", p.ID, space, root.Outcome, root.PageID)
	return root, nil
}

// sprintPageID is the page a report hangs under: the sprint's own overview
// page when the rituals sync has placed one, and the rituals root when it
// has not. A report is about one sprint, so it belongs beneath that sprint's
// page rather than loose in the space.
func (a *App) sprintPageID(profileID string, boardID, sprintID int, rootID string) string {
	k, err := ritualKey(profileID, boardID, sprintID, ritualtemplate.Sprint)
	if err != nil {
		return rootID
	}
	overview, ok, err := a.rituals.Document(a.ctx, k)
	if err != nil || !ok || overview.PageID == "" || overview.Status == ritualrepo.StatusGone {
		return rootID
	}
	return overview.PageID
}

// ExportSprintReportXLSX writes the report as a spreadsheet where the user
// says and answers with the path, the convention ExportDiagnostics set. A
// cancelled dialog answers with an empty path and no error.
func (a *App) ExportSprintReportXLSX(doc reportout.Document) (string, error) {
	data, err := reportout.XLSX(doc)
	if err != nil {
		return "", err
	}
	return a.writeExport(doc.Title, "xlsx", data)
}

// ExportSprintReportPPTX writes the report as a deck where the user says and
// answers with the path, empty when the dialog was cancelled.
func (a *App) ExportSprintReportPPTX(doc reportout.Document) (string, error) {
	data, err := reportout.PPTX(doc)
	if err != nil {
		return "", err
	}
	return a.writeExport(doc.Title, "pptx", data)
}

// writeExport asks the user where the export goes and writes it there,
// answering with the path so they are told rather than left to find the file.
// A cancelled dialog answers "" with no error: nothing was written, so there
// is nothing to report and nothing went wrong.
//
// The prefilled name carries the report's own title so three sprints exported
// in one sitting can be told apart, and a timestamp so a second export of the
// same sprint does not land on a file somebody has already opened. Whether to
// overwrite is the dialog's own question to ask.
func (a *App) writeExport(title, extension string, data []byte) (string, error) {
	dir, err := a.exportDirectory()
	if err != nil {
		return "", err
	}
	path, err := a.saveTo(runtime.SaveDialogOptions{
		Title:            "Save the sprint report",
		DefaultDirectory: dir,
		DefaultFilename:  fmt.Sprintf("tam-report-%s-%d.%s", slug(title), time.Now().Unix(), extension),
		Filters:          []runtime.FileFilter{{DisplayName: exportKinds[extension], Pattern: "*." + extension}},
	})
	if err != nil {
		return "", fmt.Errorf("save dialog: %w", err)
	}
	if path == "" {
		return "", nil // cancelled
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write the report to %s: %w", path, err)
	}
	log.Printf("tam: sprint report exported to %s", path)
	return path, nil
}

// exportKinds names each export in the save dialog's file type list.
var exportKinds = map[string]string{"xlsx": "Excel workbook", "pptx": "PowerPoint deck"}

// saveTo opens the save dialog. The field is the seam a test answers through;
// the running app has none and reaches Wails.
func (a *App) saveTo(opts runtime.SaveDialogOptions) (string, error) {
	if a.saveDialog != nil {
		return a.saveDialog(opts)
	}
	return runtime.SaveFileDialog(a.ctx, opts)
}

// exportDirectory is where the save dialog starts: the configured folder while
// it is still a folder, and the app data directory otherwise, which is where
// exports landed before there was a setting at all. A folder that has gone, on
// an unplugged drive or deleted since it was set, must not stop an export.
func (a *App) exportDirectory() (string, error) {
	if a.settings != nil {
		if s, err := a.settings.Get(); err == nil && s.ReportExportDir != "" {
			if info, err := os.Stat(s.ReportExportDir); err == nil && info.IsDir() {
				return s.ReportExportDir, nil
			}
			log.Printf("tam: the report export folder %s is not there; starting in the app data directory", s.ReportExportDir)
		}
	}
	dir := filepath.Dir(a.dbPath)
	if a.dbPath == "" || dir == "" || dir == "." {
		return "", errors.New("no app data directory to export into")
	}
	return dir, nil
}

// SetReportExportDirectory records where the export save dialog starts. The
// path arrives from a text box, so it is checked rather than trusted: an empty
// one clears the setting, and anything else has to be a folder that is there,
// or the dialog would open somewhere the user never meant.
func (a *App) SetReportExportDirectory(dir string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	dir = strings.TrimSpace(dir)
	if dir != "" {
		info, err := os.Stat(dir)
		if err != nil {
			return fmt.Errorf("%s cannot be opened as a folder: %w", dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("%s is a file, not a folder", dir)
		}
	}
	return a.settings.SetReportExportDir(dir)
}

// ChooseReportExportDirectory is the Browse button beside that setting. It
// answers "" when the user closes the picker, which leaves the field alone.
func (a *App) ChooseReportExportDirectory() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose the report export folder"})
	if err != nil {
		return "", fmt.Errorf("folder dialog: %w", err)
	}
	return dir, nil
}

// slug is a title reduced to what every filesystem accepts: lower case
// letters and digits, with everything else becoming a single hyphen. A
// sprint called "Q3 / hardening" would otherwise name a file that cannot be
// written on Windows at all.
func slug(title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case !strings.HasSuffix(b.String(), "-"):
			b.WriteByte('-')
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" {
		return "sprint"
	}
	return name
}
