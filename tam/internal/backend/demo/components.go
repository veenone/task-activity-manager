package demo

import (
	"context"
	"fmt"
	"slices"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/demo"
)

var _ backend.ComponentBackend = (*Backend)(nil)

// The demo's component writes, held in memory for the run the way its
// version writes are (versions.go), so a demo profile can draft, edit and
// delete components and see Commit do it.

// componentList is the dataset's components with this run's writes,
// copied from the dataset on first use. Callers hold b.mu.
func (b *Backend) componentList() []backend.Component {
	if b.components == nil {
		b.components = demo.Components(b.project)
		b.nextComponentID = 10300
	}
	return b.components
}

func (b *Backend) componentIndex(id int) (int, error) {
	for i, c := range b.componentList() {
		if c.ID == id {
			return i, nil
		}
	}
	return 0, fmt.Errorf("demo: no component %d", id)
}

// leadDisplay is the display name the demo's directory gives a username,
// so a lead set on a demo profile reads the way Jira would answer it.
func (b *Backend) leadDisplay(name string) string {
	if name == "" {
		return ""
	}
	users, _ := b.SearchUsers(context.Background(), b.project, name)
	for _, u := range users {
		if u.Name == name {
			return u.DisplayName
		}
	}
	return name
}

// CreateComponent adds a component with the next free id.
func (b *Backend) CreateComponent(_ context.Context, projectKey string, in backend.ComponentInput) (backend.Component, error) {
	display := b.leadDisplay(in.LeadName)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.componentList()
	b.nextComponentID++
	c := backend.Component{ID: b.nextComponentID, Project: projectKey, Name: in.Name, Description: in.Description,
		LeadName: in.LeadName, LeadDisplayName: display, AssigneeType: in.AssigneeType}
	b.components = append(b.components, c)
	return c, nil
}

// UpdateComponent rewrites the fields, an empty lead clearing it as Jira's
// own update does.
func (b *Backend) UpdateComponent(_ context.Context, id int, in backend.ComponentInput) (backend.Component, error) {
	display := b.leadDisplay(in.LeadName)
	b.mu.Lock()
	defer b.mu.Unlock()
	i, err := b.componentIndex(id)
	if err != nil {
		return backend.Component{}, err
	}
	c := &b.components[i]
	c.Name, c.Description, c.LeadName, c.LeadDisplayName, c.AssigneeType = in.Name, in.Description, in.LeadName, display, in.AssigneeType
	return *c, nil
}

// DeleteComponent removes the component from the list and from every
// issue, filing those issues under moveIssuesTo when that is set.
func (b *Backend) DeleteComponent(_ context.Context, id, moveIssuesTo int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	i, err := b.componentIndex(id)
	if err != nil {
		return err
	}
	b.components = slices.Delete(b.components, i, i+1)
	for _, iss := range b.issues() {
		if !slices.Contains(iss.Components, id) {
			continue
		}
		next := []int{}
		for _, c := range iss.Components {
			if c == id {
				c = moveIssuesTo
			}
			if c != 0 && !slices.Contains(next, c) {
				next = append(next, c)
			}
		}
		iss.Components = next
		iss.Updated = time.Now().UTC().Format(time.RFC3339)
		b.over[iss.Key] = iss
	}
	return nil
}
