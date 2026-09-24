package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agile-suite/tam/internal/reportout"
	"agile-suite/tam/internal/ritualrepo"
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
// under the sprint's overview page when the rituals sync has made one, and
// at the rituals root otherwise.
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

	published, err := reportout.Publish(a.ctx, pages, cfg.SpaceKey, a.sprintPageID(p.ID, boardID, sprintID, cfg.RootPageID), doc)
	if err != nil {
		log.Printf("tam: publishing the report for sprint %d on board %d for %s failed: %v", sprintID, boardID, p.Name, err)
		return reportout.Published{}, err
	}
	log.Printf("tam: report for sprint %d on board %d for %s published to page %s (%s)", sprintID, boardID, p.Name, published.PageID, published.Title)
	return published, nil
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

// ExportSprintReportXLSX writes the report as a spreadsheet beside tam.db
// and answers with the path, the convention ExportDiagnostics set.
func (a *App) ExportSprintReportXLSX(doc reportout.Document) (string, error) {
	data, err := reportout.XLSX(doc)
	if err != nil {
		return "", err
	}
	return a.writeExport(doc.Title, "xlsx", data)
}

// ExportSprintReportPPTX writes the report as a deck beside tam.db and
// answers with the path.
func (a *App) ExportSprintReportPPTX(doc reportout.Document) (string, error) {
	data, err := reportout.PPTX(doc)
	if err != nil {
		return "", err
	}
	return a.writeExport(doc.Title, "pptx", data)
}

// writeExport saves an export beside the database and answers with where it
// went, so the user is told a path rather than left to find the file.
//
// The name carries the report's own title so three sprints exported in one
// sitting can be told apart, and a timestamp so a second export of the same
// sprint does not silently replace a file somebody has already opened.
func (a *App) writeExport(title, extension string, data []byte) (string, error) {
	dir := filepath.Dir(a.dbPath)
	if a.dbPath == "" || dir == "" || dir == "." {
		return "", errors.New("no app data directory to export into")
	}
	path := filepath.Join(dir, fmt.Sprintf("tam-report-%s-%d.%s", slug(title), time.Now().Unix(), extension))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write the report to %s: %w", path, err)
	}
	log.Printf("tam: sprint report exported to %s", path)
	return path, nil
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
