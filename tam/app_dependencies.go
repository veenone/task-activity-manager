package main

import (
	"strings"

	"agile-suite/tam/internal/backend"
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
