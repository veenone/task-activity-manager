package main

import (
	"agile-suite/tam/internal/backend"
)

// GetVersions is the cached releases (Jira fix versions) of the profile's
// project, drafts included. It reads the store only, so it answers offline.
func (a *App) GetVersions(profileID string) ([]backend.Version, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return nil, err
	}
	return a.repo.Versions(a.ctx, p.ID, p.ProjectKey)
}

// GetComponents is the cached components of the profile's project.
func (a *App) GetComponents(profileID string) ([]backend.Component, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return nil, err
	}
	return a.repo.Components(a.ctx, p.ID, p.ProjectKey)
}
