package main

import (
	"log"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// The Components view's bindings (#605). Every write is journaled and
// Commit pushes it, so none needs a backend and all work offline. They
// share the releases' prelude (lockedRelease): the "release" lock, and
// Administer Projects for any write to a component Jira holds. A draft's
// edit and delete are local and not gated.

// ComponentSummary is one component with how many cached issues carry it.
type ComponentSummary struct {
	backend.Component
	Issues int `json:"issues"`
}

// ComponentForm is what the component form sends: a lead username with
// the display name the picker showed, and Jira's default assignee value.
type ComponentForm struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	LeadName        string `json:"leadName"`
	LeadDisplayName string `json:"leadDisplayName"`
	AssigneeType    string `json:"assigneeType"`
}

func (f ComponentForm) fields() issuerepo.ComponentFields {
	return issuerepo.ComponentFields(f)
}

// ListComponentSummaries is every cached component of the profile's
// project, drafts included, each with its cached issue count, by name.
func (a *App) ListComponentSummaries(profileID string) ([]ComponentSummary, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return nil, err
	}
	cs, err := a.repo.Components(a.ctx, p.ID, p.ProjectKey)
	if err != nil {
		return nil, err
	}
	counts, err := a.repo.ComponentCounts(a.ctx, p.ID)
	if err != nil {
		return nil, err
	}
	out := make([]ComponentSummary, 0, len(cs))
	for _, c := range cs {
		out = append(out, ComponentSummary{Component: c, Issues: counts[c.ID]})
	}
	return out, nil
}

// ComponentIssues is the cached issues filed under one component.
func (a *App) ComponentIssues(profileID string, id int) ([]backend.Issue, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return nil, err
	}
	return a.repo.ComponentIssues(a.ctx, p.ID, id)
}

// CreateComponent drafts a new component of the profile's project.
func (a *App) CreateComponent(profileID string, in ComponentForm) (backend.Component, error) {
	id, project, err := a.lockedRelease(profileID, true)
	if err != nil {
		return backend.Component{}, err
	}
	defer a.release(id)
	made, err := a.repo.CreateDraftComponent(a.ctx, id, project, in.fields())
	if err == nil {
		log.Printf("tam: drafted component %d %q in %s", made.ID, made.Name, project)
	}
	return made, err
}

// EditComponent changes a component: a draft in place, a component Jira
// holds as a pending edit.
func (a *App) EditComponent(profileID string, componentID int, in ComponentForm) error {
	id, _, err := a.lockedRelease(profileID, componentID > 0)
	if err != nil {
		return err
	}
	defer a.release(id)
	return a.repo.EditComponent(a.ctx, id, componentID, in.fields())
}

// DeleteComponent discards a draft component, or queues deleting one Jira
// holds, moving its issues to moveTo when that is set.
func (a *App) DeleteComponent(profileID string, componentID, moveTo int) error {
	id, _, err := a.lockedRelease(profileID, componentID > 0)
	if err != nil {
		return err
	}
	defer a.release(id)
	return a.repo.QueueComponentDelete(a.ctx, id, componentID, moveTo)
}
