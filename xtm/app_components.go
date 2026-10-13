package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/componentadmin"
	"agile-suite/xtm/internal/testrepo"

	"github.com/wailsapp/wails/v2/pkg/runtime"
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

// CheckComponents says which rows of a batch create cannot be created and
// why, for the preview before CreateComponents.
func (a *App) CheckComponents(profileID string, rows []backend.ComponentInput) (out []componentadmin.BatchRow, err error) {
	defer recoverToError("CheckComponents", &err)
	svc, _, err := a.componentAdmin(profileID)
	if err != nil {
		return nil, err
	}
	return svc.Check(a.ctx, rows)
}

// CreateComponents creates several project components in Jira, carrying on
// past rows that fail.
func (a *App) CreateComponents(profileID string, rows []backend.ComponentInput) (out componentadmin.BatchResult, err error) {
	defer recoverToError("CreateComponents", &err)
	svc, _, err := a.componentAdmin(profileID)
	if err != nil {
		return componentadmin.BatchResult{}, err
	}
	return svc.CreateMany(a.ctx, rows)
}

// ParseComponentWorkbook reads component rows from an uploaded XLSX file.
func (a *App) ParseComponentWorkbook(contentB64 string) (out []backend.ComponentInput, err error) {
	defer recoverToError("ParseComponentWorkbook", &err)
	data, err := base64.StdEncoding.DecodeString(contentB64)
	if err != nil {
		return nil, fmt.Errorf("decode upload: %w", err)
	}
	return componentadmin.ParseWorkbook(data)
}

// DownloadComponentTemplate saves the component import template where the
// user chooses. Returns the path written, or "" when cancelled.
func (a *App) DownloadComponentTemplate() (path string, err error) {
	defer recoverToError("DownloadComponentTemplate", &err)
	data, err := componentadmin.TemplateWorkbook()
	if err != nil {
		return "", err
	}
	dest, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save component import template",
		DefaultFilename: "component-import-template.xlsx",
		Filters:         []runtime.FileFilter{{DisplayName: "Excel Workbook", Pattern: "*.xlsx"}},
	})
	if err != nil {
		return "", fmt.Errorf("save dialog: %w", err)
	}
	if dest == "" {
		return "", nil
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return "", fmt.Errorf("write template: %w", err)
	}
	return dest, nil
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

// SetTestComponents replaces one test's components, queued for the next
// commit.
func (a *App) SetTestComponents(profileID, testKey string, names []string) (err error) {
	defer recoverToError("SetTestComponents", &err)
	if err := a.requireStore(); err != nil {
		return err
	}
	return a.repo.SetTestComponents(profileID, testKey, names)
}

// BulkEditComponents adds, removes or replaces components across tests,
// queued for the next commit.
func (a *App) BulkEditComponents(profileID string, testKeys, add, remove []string, replace bool) (result testrepo.BulkEditResult, err error) {
	defer recoverToError("BulkEditComponents", &err)
	empty := testrepo.BulkEditResult{Succeeded: []string{}, Failed: []testrepo.BulkFailure{}}
	if err := a.requireStore(); err != nil {
		return empty, err
	}
	return a.repo.BulkEditComponents(profileID, testKeys, add, remove, replace)
}

// ListTestComponents returns each given test's components, for the Bulk
// Components preview.
func (a *App) ListTestComponents(profileID string, testKeys []string) (map[string][]string, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.repo.ListTestComponents(profileID, testKeys)
}
