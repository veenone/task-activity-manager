package main

import (
	"agile-suite/tam/internal/issueexport"
	"agile-suite/tam/internal/issuerepo"
)

// ExportBacklog writes the rows the grid is showing to a workbook, where
// the user says, and answers with the path. A cancelled dialog answers ""
// with no error: nothing was written, so nothing went wrong.
//
// It exports the filter and not the page, which is why the query goes to
// ListIssuesForExport rather than ListIssues: a planning session wants the
// whole backlog it narrowed to, not the 25 rows on screen.
func (a *App) ExportBacklog(profileID string, q issuerepo.IssueQuery) (string, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return "", err
	}
	rows, err := a.repo.ListIssuesForExport(a.ctx, p.ID, q)
	if err != nil {
		return "", err
	}
	data, err := issueexport.Workbook(rows)
	if err != nil {
		return "", err
	}
	return a.writeExport(backlogExport, p.Name, "xlsx", data)
}
