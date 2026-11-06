package main

import (
	"time"

	corejira "agile-suite/core/jira"
	"agile-suite/pie/internal/cases"
	"agile-suite/pie/internal/query"
)

// The bindings for a case's page: its notes, the projects to assess with a
// verdict for each, and the named project groups. Each validates through
// internal/cases; nothing here reaches Jira except the project list.

// GetCase returns one case with its projects.
func (a *App) GetCase(profileID string, id int64) (cases.Detail, error) {
	if err := a.requireStore(); err != nil {
		return cases.Detail{}, err
	}
	return a.cases.Get(profileID, id)
}

// SetCaseNotes replaces a case's notes.
func (a *App) SetCaseNotes(profileID string, id int64, notes string) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	return a.cases.SetNotes(profileID, id, notes)
}

// AddCaseProjects puts projects on a case at not assessed.
func (a *App) AddCaseProjects(profileID string, id int64, keys []string) (cases.Detail, error) {
	if err := a.requireStore(); err != nil {
		return cases.Detail{}, err
	}
	if err := a.cases.AddProjects(profileID, id, keys); err != nil {
		return cases.Detail{}, err
	}
	return a.cases.Get(profileID, id)
}

// RemoveCaseProject takes a project and its verdict off a case.
func (a *App) RemoveCaseProject(profileID string, id int64, key string) (cases.Detail, error) {
	if err := a.requireStore(); err != nil {
		return cases.Detail{}, err
	}
	if err := a.cases.RemoveProject(profileID, id, key); err != nil {
		return cases.Detail{}, err
	}
	return a.cases.Get(profileID, id)
}

// SetVerdict records the analyst's verdict for one project on a case.
func (a *App) SetVerdict(profileID string, id int64, key, verdict, reason string) (cases.Detail, error) {
	if err := a.requireStore(); err != nil {
		return cases.Detail{}, err
	}
	if err := a.cases.SetVerdict(profileID, id, key, verdict, reason, time.Now()); err != nil {
		return cases.Detail{}, err
	}
	return a.cases.Get(profileID, id)
}

// JiraProjects lists every project the profile's account can see.
func (a *App) JiraProjects(profileID string) ([]corejira.Project, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	client, err := a.client(profileID)
	if err != nil {
		return nil, err
	}
	return query.Projects(a.ctx, client)
}

// ProjectGroups returns the profile's named project groups.
func (a *App) ProjectGroups(profileID string) ([]cases.Group, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.cases.Groups(profileID)
}

// SaveProjectGroup stores a new named group of projects.
func (a *App) SaveProjectGroup(profileID, name string, keys []string) (cases.Group, error) {
	if err := a.requireStore(); err != nil {
		return cases.Group{}, err
	}
	return a.cases.SaveGroup(profileID, name, keys)
}

// DeleteProjectGroup removes a named group. Cases keep their projects.
func (a *App) DeleteProjectGroup(profileID string, id int64) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	return a.cases.DeleteGroup(profileID, id)
}

// CaseGrid returns every case with its projects and verdicts, for the grid.
func (a *App) CaseGrid(profileID string) ([]cases.Detail, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.cases.Grid(profileID)
}
