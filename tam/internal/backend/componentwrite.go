package backend

import "context"

// ComponentInput is what a component's create and update send. LeadName is
// a username, empty for no lead; AssigneeType is one of Jira's four values.
type ComponentInput struct {
	Name         string
	Description  string
	LeadName     string
	AssigneeType string
}

// ComponentBackend writes the project's components, which Commit pushes
// (#605). Optional, like VersionBackend: a backend without it fails each
// component change with a reason.
type ComponentBackend interface {
	CreateComponent(ctx context.Context, projectKey string, in ComponentInput) (Component, error)
	UpdateComponent(ctx context.Context, id int, in ComponentInput) (Component, error)
	// DeleteComponent deletes the component, moving its issues to
	// moveIssuesTo when that is above 0.
	DeleteComponent(ctx context.Context, id, moveIssuesTo int) error
}
