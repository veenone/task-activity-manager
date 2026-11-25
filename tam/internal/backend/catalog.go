package backend

import "context"

// Version is a Jira fix version of the profile's project, which TAM's UI
// calls a release. ID is Jira's numeric id, or negative for a draft made in
// TAM and not yet committed, the scheme sprints use.
type Version struct {
	ID          int    `json:"id"`
	Project     string `json:"project"`
	Name        string `json:"name"`
	Description string `json:"description"`
	StartDate   string `json:"startDate"`
	ReleaseDate string `json:"releaseDate"`
	Released    bool   `json:"released"`
	Archived    bool   `json:"archived"`
	Draft       bool   `json:"draft"`
}

// Component is one of the project's components, with the same id scheme as
// Version.
type Component struct {
	ID              int    `json:"id"`
	Project         string `json:"project"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	LeadName        string `json:"leadName"`
	LeadDisplayName string `json:"leadDisplayName"`
	AssigneeType    string `json:"assigneeType"`
	Draft           bool   `json:"draft"`
}

// CatalogBackend reads a project's fix versions and components. Optional,
// like PermissionBackend: the syncer skips it when a backend lacks it.
type CatalogBackend interface {
	ProjectVersions(ctx context.Context, projectKey string) ([]Version, error)
	ProjectComponents(ctx context.Context, projectKey string) ([]Component, error)
}
