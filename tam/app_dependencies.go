package main

import (
	"strings"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issueexport"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/permission"
)

// RemoveLink journals the removal of key's cached link l, pushed to Jira on
// Commit (#612).
func (a *App) RemoveLink(profileID, key string, l backend.Link) error {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return err
	}
	if err := a.allowOnIssue(p.ID, key, permission.LinkIssues); err != nil {
		return err
	}
	return a.repo.RemoveLink(a.ctx, p.ID, strings.TrimSpace(key), l)
}

// RetypeLink journals a change of l's type as a removal plus an add (#612).
func (a *App) RetypeLink(profileID, key string, l backend.Link, newType string) error {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return err
	}
	if err := a.allowOnIssue(p.ID, key, permission.LinkIssues); err != nil {
		return err
	}
	return a.repo.RetypeLink(a.ctx, p.ID, strings.TrimSpace(key), l, newType)
}

// GetDependencyGraph returns the Dependencies view's data from the cache and
// the journal: every link once, outward issue to inward, and the issues it
// joins (#612).
func (a *App) GetDependencyGraph(profileID string) (issuerepo.DependencyGraph, error) {
	if err := a.requireStore(); err != nil {
		return issuerepo.DependencyGraph{}, err
	}
	return a.repo.DependencyGraph(a.ctx, profileID)
}

// ExportDependencies writes the Dependencies view as a workbook where the
// user says (#612): one issue's chain, or with no focus every issue of the
// profile, linked or not. image is the graph the view drew, as a PNG data
// URL, or "" for none. A cancelled dialog answers "" with no error.
func (a *App) ExportDependencies(profileID string, scope issueexport.DependencyScope, image string) (string, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return "", err
	}
	pic, err := issueexport.DecodePNG(image)
	if err != nil {
		return "", err
	}
	all, err := a.repo.ListIssuesForExport(a.ctx, p.ID, issuerepo.IssueQuery{})
	if err != nil {
		return "", err
	}
	g, err := a.repo.DependencyGraph(a.ctx, p.ID)
	if err != nil {
		return "", err
	}
	d := issueexport.SelectDependencies(all, g, scope)
	d.Image = pic
	data, err := issueexport.DependencyWorkbook(d)
	if err != nil {
		return "", err
	}
	title := p.Name
	if scope.Focus != "" {
		title = scope.Focus
	}
	return a.writeExport(dependencyExport, title, "xlsx", data)
}
