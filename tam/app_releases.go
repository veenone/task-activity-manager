package main

import (
	"log"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/permission"
)

// The Releases view's bindings (#605). A release is a Jira fix version;
// the code says version and the UI says release. Every write is journaled
// and Commit pushes it, so none needs a backend and all work offline. Each
// write of a release Jira holds needs Administer Projects; a draft's edit
// and delete are local and not gated, as a draft sprint's are. They take
// the "release" lock, which serialises them against a Commit rewriting the
// same rows.

// VersionSummary is one release with how many of its cached issues there
// are and how many are finished.
type VersionSummary struct {
	backend.Version
	Done  int `json:"done"`
	Total int `json:"total"`
}

// ListVersionSummaries is every cached release of the profile's project,
// drafts included, each with its progress, in the catalog's order.
func (a *App) ListVersionSummaries(profileID string) ([]VersionSummary, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return nil, err
	}
	vs, err := a.repo.Versions(a.ctx, p.ID, p.ProjectKey)
	if err != nil {
		return nil, err
	}
	counts, err := a.repo.VersionProgress(a.ctx, p.ID)
	if err != nil {
		return nil, err
	}
	out := make([]VersionSummary, 0, len(vs))
	for _, v := range vs {
		c := counts[v.ID]
		out = append(out, VersionSummary{Version: v, Done: c.Done, Total: c.Total})
	}
	return out, nil
}

// VersionIssues is the cached issues in one release.
func (a *App) VersionIssues(profileID string, id int) ([]backend.Issue, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return nil, err
	}
	return a.repo.VersionIssues(a.ctx, p.ID, id)
}

// lockedRelease is the common prelude of the writes: the profile, the
// "release" lock, and Administer Projects unless the write is to a draft.
// The caller defers a.release(id) when it answers no error.
func (a *App) lockedRelease(profileID string, gated bool) (string, string, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return "", "", err
	}
	if err := a.acquire(p.ID, "release"); err != nil {
		return "", "", err
	}
	if gated {
		if err := a.allow(p.ID, p.ProjectKey, permission.AdministerProjects); err != nil {
			a.release(p.ID)
			return "", "", err
		}
	}
	return p.ID, p.ProjectKey, nil
}

// CreateVersion drafts a new release of the profile's project. Dates are
// YYYY-MM-DD or empty.
func (a *App) CreateVersion(profileID, name, description, start, release string) (backend.Version, error) {
	id, project, err := a.lockedRelease(profileID, true)
	if err != nil {
		return backend.Version{}, err
	}
	defer a.release(id)
	made, err := a.repo.CreateDraftVersion(a.ctx, id, project, issuerepo.VersionFields{Name: name, Description: description, StartDate: start, ReleaseDate: release})
	if err == nil {
		log.Printf("tam: drafted release %d %q in %s", made.ID, made.Name, project)
	}
	return made, err
}

// EditVersion changes a release's name, description and dates: a draft in
// place, a release Jira holds as a pending edit.
func (a *App) EditVersion(profileID string, versionID int, name, description, start, release string) error {
	id, _, err := a.lockedRelease(profileID, versionID > 0)
	if err != nil {
		return err
	}
	defer a.release(id)
	return a.repo.EditVersion(a.ctx, id, versionID, issuerepo.VersionFields{Name: name, Description: description, StartDate: start, ReleaseDate: release})
}

// ReleaseVersion queues a release with the Release dialog's choice for each
// unfinished issue. The choices are fix version edits, so each issue also
// needs what a fix version edit needs.
func (a *App) ReleaseVersion(profileID string, versionID int, date string, moveTo int, moves []issuerepo.ReleaseMove) error {
	id, project, err := a.lockedRelease(profileID, true)
	if err != nil {
		return err
	}
	defer a.release(id)
	keys := make([]string, 0, len(moves))
	for _, m := range moves {
		keys = append(keys, m.Key)
	}
	if err := a.allowEdit(id, keys, "fixVersions"); err != nil {
		return err
	}
	log.Printf("tam: queueing the release of %d in %s with %d issue choices", versionID, project, len(moves))
	return a.repo.QueueVersionRelease(a.ctx, id, versionID, date, moveTo, moves)
}

// ArchiveVersion queues archiving a release, or unarchiving it.
func (a *App) ArchiveVersion(profileID string, versionID int, archived bool) error {
	id, _, err := a.lockedRelease(profileID, true)
	if err != nil {
		return err
	}
	defer a.release(id)
	return a.repo.QueueVersionArchive(a.ctx, id, versionID, archived)
}

// DeleteVersion discards a draft release, or queues deleting one Jira
// holds, moving its issues to moveTo when that is set.
func (a *App) DeleteVersion(profileID string, versionID, moveTo int) error {
	id, _, err := a.lockedRelease(profileID, versionID > 0)
	if err != nil {
		return err
	}
	defer a.release(id)
	return a.repo.QueueVersionDelete(a.ctx, id, versionID, moveTo)
}
