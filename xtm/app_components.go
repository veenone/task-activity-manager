package main

import (
	"context"
	"fmt"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/componentadmin"
)

// componentAdminBackend joins a backend's component writes with its
// component-name list, which is what componentadmin.Service needs.
type componentAdminBackend struct {
	backend.ComponentManager
	names backend.Backend
}

func (c componentAdminBackend) ProjectComponents(ctx context.Context, key string) ([]string, error) {
	return c.names.ProjectComponents(ctx, key)
}

// componentAdmin returns the component service for profileID, or
// backend.ErrUnsupported when the profile's backend cannot manage components.
func (a *App) componentAdmin(profileID string) (*componentadmin.Service, backend.ComponentManager, error) {
	if err := a.requireStore(); err != nil {
		return nil, nil, err
	}
	b, err := a.backendFor(profileID)
	if err != nil {
		return nil, nil, err
	}
	cm, ok := b.(backend.ComponentManager)
	if !ok || !b.Capabilities().SupportsComponentAdmin {
		return nil, nil, fmt.Errorf("components: %w", backend.ErrUnsupported)
	}
	p, err := a.profiles.Get(profileID)
	if err != nil {
		return nil, nil, err
	}
	return componentadmin.New(a.repo, componentAdminBackend{cm, b}, profileID, p.ProjectKey), cm, nil
}

// ListProjectComponentDetails lists the project's components live from Jira.
func (a *App) ListProjectComponentDetails(profileID string) (out []backend.Component, err error) {
	defer recoverToError("ListProjectComponentDetails", &err)
	svc, _, err := a.componentAdmin(profileID)
	if err != nil {
		return nil, err
	}
	return svc.List(a.ctx)
}

// CreateComponent creates a project component in Jira.
func (a *App) CreateComponent(profileID string, in backend.ComponentInput) (out backend.Component, err error) {
	defer recoverToError("CreateComponent", &err)
	svc, _, err := a.componentAdmin(profileID)
	if err != nil {
		return backend.Component{}, err
	}
	return svc.Create(a.ctx, in)
}

// UpdateComponent edits a project component in Jira.
func (a *App) UpdateComponent(profileID, id string, in backend.ComponentInput) (out backend.Component, err error) {
	defer recoverToError("UpdateComponent", &err)
	svc, _, err := a.componentAdmin(profileID)
	if err != nil {
		return backend.Component{}, err
	}
	return svc.Update(a.ctx, id, in)
}

// DeleteComponent deletes a project component in Jira, optionally moving its
// issues to another component first.
func (a *App) DeleteComponent(profileID, id, moveIssuesTo string) (err error) {
	defer recoverToError("DeleteComponent", &err)
	svc, _, err := a.componentAdmin(profileID)
	if err != nil {
		return err
	}
	return svc.Delete(a.ctx, id, moveIssuesTo)
}

// ComponentIssueCount is how many issues carry a component, for the delete
// dialog.
func (a *App) ComponentIssueCount(profileID, id string) (n int, err error) {
	defer recoverToError("ComponentIssueCount", &err)
	_, cm, err := a.componentAdmin(profileID)
	if err != nil {
		return 0, err
	}
	return cm.ComponentIssueCount(a.ctx, id)
}

// SearchUsers finds Jira users for the component lead picker.
func (a *App) SearchUsers(profileID, query string) (out []backend.User, err error) {
	defer recoverToError("SearchUsers", &err)
	_, cm, err := a.componentAdmin(profileID)
	if err != nil {
		return nil, err
	}
	return cm.SearchUsers(a.ctx, query)
}
