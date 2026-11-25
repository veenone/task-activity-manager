package jira

import (
	"context"

	corejira "agile-suite/core/jira"
)

// Component and ComponentInput are core's; the HTTP calls live in core/jira
// so TAM can share them. This file only answers demo profiles locally.
type (
	Component      = corejira.Component
	ComponentInput = corejira.ComponentInput
)

// ProjectComponentDetails lists a project's components with their details.
func (c *Client) ProjectComponentDetails(ctx context.Context, projectKey string) ([]Component, error) {
	if isDemoURL(c.baseURL) {
		return demoComponentDetails(), nil
	}
	return c.Client.ProjectComponentDetails(ctx, projectKey)
}

// CreateComponent creates a component in in.Project.
func (c *Client) CreateComponent(ctx context.Context, in ComponentInput) (Component, error) {
	if isDemoURL(c.baseURL) {
		return demoCreateComponent(in)
	}
	return c.Client.CreateComponent(ctx, in)
}

// UpdateComponent replaces a component's name, description, lead and assignee
// type. An empty LeadUserName clears the lead.
func (c *Client) UpdateComponent(ctx context.Context, id string, in ComponentInput) (Component, error) {
	if isDemoURL(c.baseURL) {
		return demoUpdateComponent(id, in)
	}
	return c.Client.UpdateComponent(ctx, id, in)
}

// DeleteComponent deletes a component, moving its issues to moveIssuesTo
// when set.
func (c *Client) DeleteComponent(ctx context.Context, id, moveIssuesTo string) error {
	if isDemoURL(c.baseURL) {
		return demoDeleteComponent(id)
	}
	return c.Client.DeleteComponent(ctx, id, moveIssuesTo)
}

// ComponentIssueCount is how many issues carry the component.
func (c *Client) ComponentIssueCount(ctx context.Context, id string) (int, error) {
	if isDemoURL(c.baseURL) {
		return 3, nil
	}
	return c.Client.ComponentIssueCount(ctx, id)
}

// SearchUsers finds users for the lead picker.
func (c *Client) SearchUsers(ctx context.Context, query string) ([]User, error) {
	if isDemoURL(c.baseURL) {
		return demoSearchUsers(query), nil
	}
	return c.Client.SearchUsers(ctx, query)
}
