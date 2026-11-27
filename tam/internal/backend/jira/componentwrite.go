package jira

import (
	"context"
	"fmt"
	"strconv"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
)

var _ backend.ComponentBackend = (*Backend)(nil)

// The component writes Commit pushes (#605), over core/jira's client, which
// XTM uses for the same calls. The id conversion is versionwrite.go's.

func componentInput(projectKey string, in backend.ComponentInput) corejira.ComponentInput {
	return corejira.ComponentInput{Project: projectKey, Name: in.Name, Description: in.Description, LeadUserName: in.LeadName, AssigneeType: in.AssigneeType}
}

func fromJiraComponent(projectKey string, c corejira.Component) (backend.Component, error) {
	id, err := strconv.Atoi(c.ID)
	if err != nil {
		return backend.Component{}, fmt.Errorf("jira answered component %q with id %q, not a number", c.Name, c.ID)
	}
	return backend.Component{ID: id, Project: projectKey, Name: c.Name, Description: c.Description,
		LeadName: c.LeadName, LeadDisplayName: c.LeadDisplayName, AssigneeType: c.AssigneeType}, nil
}

// CreateComponent creates a component of the project.
func (b *Backend) CreateComponent(ctx context.Context, projectKey string, in backend.ComponentInput) (backend.Component, error) {
	made, err := b.c.CreateComponent(ctx, componentInput(projectKey, in))
	if err != nil {
		return backend.Component{}, err
	}
	return fromJiraComponent(projectKey, made)
}

// UpdateComponent rewrites a component's name, description, lead and
// default assignee; an empty lead clears it.
func (b *Backend) UpdateComponent(ctx context.Context, id int, in backend.ComponentInput) (backend.Component, error) {
	made, err := b.c.UpdateComponent(ctx, strconv.Itoa(id), componentInput("", in))
	if err != nil {
		return backend.Component{}, err
	}
	return fromJiraComponent("", made)
}

// DeleteComponent deletes the component, moving its issues when a target is
// given.
func (b *Backend) DeleteComponent(ctx context.Context, id, moveIssuesTo int) error {
	return b.c.DeleteComponent(ctx, strconv.Itoa(id), idOrEmpty(moveIssuesTo))
}
