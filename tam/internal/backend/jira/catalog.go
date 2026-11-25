package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"agile-suite/tam/internal/backend"
)

var _ backend.CatalogBackend = (*Backend)(nil)

// ProjectVersions reads the project's fix versions. An id that is not a
// number fails the read: stored as 0 it would collide with every other.
func (b *Backend) ProjectVersions(ctx context.Context, projectKey string) ([]backend.Version, error) {
	raw, err := b.c.ProjectVersions(ctx, projectKey)
	if err != nil {
		return nil, err
	}
	out := make([]backend.Version, 0, len(raw))
	for _, v := range raw {
		id, err := strconv.Atoi(v.ID)
		if err != nil {
			return nil, fmt.Errorf("version %q of %s has id %q, not a number", v.Name, projectKey, v.ID)
		}
		out = append(out, backend.Version{ID: id, Project: projectKey, Name: v.Name, Description: v.Description,
			StartDate: v.StartDate, ReleaseDate: v.ReleaseDate, Released: v.Released, Archived: v.Archived})
	}
	return out, nil
}

// ProjectComponents reads the project's components, with ProjectVersions'
// rule for an id that is not a number.
func (b *Backend) ProjectComponents(ctx context.Context, projectKey string) ([]backend.Component, error) {
	raw, err := b.c.ProjectComponentDetails(ctx, projectKey)
	if err != nil {
		return nil, err
	}
	out := make([]backend.Component, 0, len(raw))
	for _, c := range raw {
		id, err := strconv.Atoi(c.ID)
		if err != nil {
			return nil, fmt.Errorf("component %q of %s has id %q, not a number", c.Name, projectKey, c.ID)
		}
		out = append(out, backend.Component{ID: id, Project: projectKey, Name: c.Name, Description: c.Description,
			LeadName: c.LeadName, LeadDisplayName: c.LeadDisplayName, AssigneeType: c.AssigneeType})
	}
	return out, nil
}

// catalogIDs reads an issue's components or fixVersions field, the objects
// Jira sends, as ids. An id that is not a number is skipped: the issue row
// is worth more than one bad reference on it.
func catalogIDs(raw json.RawMessage) []int {
	var refs []struct {
		ID string `json:"id"`
	}
	out := []int{}
	if err := json.Unmarshal(raw, &refs); err != nil {
		return out
	}
	for _, r := range refs {
		if id, err := strconv.Atoi(r.ID); err == nil {
			out = append(out, id)
		}
	}
	return out
}
