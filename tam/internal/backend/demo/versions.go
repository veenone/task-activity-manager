package demo

import (
	"context"
	"fmt"
	"slices"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/demo"
)

var _ backend.VersionBackend = (*Backend)(nil)

// The demo's fix version writes, held in memory for the run the way a
// created sprint is, so a demo profile can draft, release and delete
// releases and see Commit do it.

// versionList is the dataset's versions with this run's writes, copied
// from the dataset on first use. Callers hold b.mu.
func (b *Backend) versionList() []backend.Version {
	if b.versions == nil {
		b.versions = demo.Versions(b.project)
		b.nextVersionID = 10200
	}
	return b.versions
}

func (b *Backend) versionIndex(id int) (int, error) {
	for i, v := range b.versionList() {
		if v.ID == id {
			return i, nil
		}
	}
	return 0, fmt.Errorf("demo: no version %d", id)
}

// CreateVersion adds a version with the next free id.
func (b *Backend) CreateVersion(_ context.Context, projectKey string, in backend.VersionInput) (backend.Version, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.versionList()
	b.nextVersionID++
	v := backend.Version{ID: b.nextVersionID, Project: projectKey, Name: in.Name, Description: in.Description, StartDate: in.StartDate, ReleaseDate: in.ReleaseDate}
	b.versions = append(b.versions, v)
	return v, nil
}

// UpdateVersion rewrites the four fields, an empty date clearing it as
// Jira's own update does.
func (b *Backend) UpdateVersion(_ context.Context, id int, in backend.VersionInput) (backend.Version, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	i, err := b.versionIndex(id)
	if err != nil {
		return backend.Version{}, err
	}
	v := &b.versions[i]
	v.Name, v.Description, v.StartDate, v.ReleaseDate = in.Name, in.Description, in.StartDate, in.ReleaseDate
	return *v, nil
}

// ReleaseVersion marks the version released, moving its unfinished issues
// to moveUnfixedTo when that is set.
func (b *Backend) ReleaseVersion(_ context.Context, id int, releaseDate string, moveUnfixedTo int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	i, err := b.versionIndex(id)
	if err != nil {
		return err
	}
	b.versions[i].Released = true
	if releaseDate != "" {
		b.versions[i].ReleaseDate = releaseDate
	}
	if moveUnfixedTo > 0 {
		b.moveIssues(id, moveUnfixedTo, func(iss backend.Issue) bool { return !backend.Finished(iss) })
	}
	return nil
}

// ArchiveVersion sets the archived flag.
func (b *Backend) ArchiveVersion(_ context.Context, id int, archived bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	i, err := b.versionIndex(id)
	if err != nil {
		return err
	}
	b.versions[i].Archived = archived
	return nil
}

// DeleteVersion removes the version from the list and from every issue,
// moving those issues to moveFixIssuesTo when that is set.
func (b *Backend) DeleteVersion(_ context.Context, id, moveFixIssuesTo int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	i, err := b.versionIndex(id)
	if err != nil {
		return err
	}
	b.versions = slices.Delete(b.versions, i, i+1)
	b.moveIssues(id, moveFixIssuesTo, func(backend.Issue) bool { return true })
	return nil
}

// UnresolvedInVersion lists the version's unfinished issues.
func (b *Backend) UnresolvedInVersion(_ context.Context, id int) ([]string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	keys := []string{}
	for _, iss := range b.issues() {
		if slices.Contains(iss.FixVersions, id) && !backend.Finished(iss) {
			keys = append(keys, iss.Key)
		}
	}
	return keys, nil
}

// moveIssues takes version from off every issue that matches and puts to
// on it instead, or nothing when to is 0. Callers hold b.mu.
func (b *Backend) moveIssues(from, to int, match func(backend.Issue) bool) {
	for _, iss := range b.issues() {
		if !slices.Contains(iss.FixVersions, from) || !match(iss) {
			continue
		}
		next := []int{}
		for _, v := range iss.FixVersions {
			if v == from {
				v = to
			}
			if v != 0 && !slices.Contains(next, v) {
				next = append(next, v)
			}
		}
		iss.FixVersions = next
		iss.Updated = time.Now().UTC().Format(time.RFC3339)
		b.over[iss.Key] = iss
	}
}
