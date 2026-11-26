package main

import (
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
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

// SetIssuesCatalog sets components or fix versions on every key, adding to
// what each issue has or replacing it, as pending edits. Issues from other
// projects are skipped and named in the result.
func (a *App) SetIssuesCatalog(profileID string, keys []string, field string, ids []int, add bool) (issuerepo.CatalogSetResult, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return issuerepo.CatalogSetResult{}, err
	}
	if err := a.allowEdit(p.ID, keys, field); err != nil {
		return issuerepo.CatalogSetResult{}, err
	}
	return a.repo.SetCatalog(a.ctx, p.ID, p.ProjectKey, keys, field, ids, add)
}

// GetComponents is the cached components of the profile's project.
func (a *App) GetComponents(profileID string) ([]backend.Component, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return nil, err
	}
	return a.repo.Components(a.ctx, p.ID, p.ProjectKey)
}
