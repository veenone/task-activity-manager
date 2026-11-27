package jira

import (
	"context"
	"fmt"
	"strconv"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
)

var _ backend.VersionBackend = (*Backend)(nil)

// The version writes Commit pushes (#605), over core/jira's client. TAM
// holds ids as ints, Jira as numeric strings; the conversion is here.

func versionInput(projectKey string, in backend.VersionInput) corejira.VersionInput {
	return corejira.VersionInput{Project: projectKey, Name: in.Name, Description: in.Description, StartDate: in.StartDate, ReleaseDate: in.ReleaseDate}
}

func fromJiraVersion(projectKey string, v corejira.Version) (backend.Version, error) {
	id, err := strconv.Atoi(v.ID)
	if err != nil {
		return backend.Version{}, fmt.Errorf("jira answered version %q with id %q, not a number", v.Name, v.ID)
	}
	return backend.Version{ID: id, Project: projectKey, Name: v.Name, Description: v.Description,
		StartDate: v.StartDate, ReleaseDate: v.ReleaseDate, Released: v.Released, Archived: v.Archived}, nil
}

// CreateVersion creates a fix version of the project.
func (b *Backend) CreateVersion(ctx context.Context, projectKey string, in backend.VersionInput) (backend.Version, error) {
	made, err := b.c.CreateVersion(ctx, versionInput(projectKey, in))
	if err != nil {
		return backend.Version{}, err
	}
	return fromJiraVersion(projectKey, made)
}

// UpdateVersion rewrites a version's name, description and dates; an empty
// date clears it.
func (b *Backend) UpdateVersion(ctx context.Context, id int, in backend.VersionInput) (backend.Version, error) {
	made, err := b.c.UpdateVersion(ctx, strconv.Itoa(id), versionInput("", in))
	if err != nil {
		return backend.Version{}, err
	}
	return fromJiraVersion("", made)
}

// ReleaseVersion releases the version, moving its unresolved issues when a
// target is given.
func (b *Backend) ReleaseVersion(ctx context.Context, id int, releaseDate string, moveUnfixedTo int) error {
	return b.c.ReleaseVersion(ctx, strconv.Itoa(id), releaseDate, idOrEmpty(moveUnfixedTo))
}

// ArchiveVersion archives or unarchives the version.
func (b *Backend) ArchiveVersion(ctx context.Context, id int, archived bool) error {
	return b.c.ArchiveVersion(ctx, strconv.Itoa(id), archived)
}

// DeleteVersion deletes the version, moving its issues when a target is
// given.
func (b *Backend) DeleteVersion(ctx context.Context, id, moveFixIssuesTo int) error {
	return b.c.DeleteVersion(ctx, strconv.Itoa(id), idOrEmpty(moveFixIssuesTo))
}

// unresolvedPage is how many keys one search page asks for.
const unresolvedPage = 100

// UnresolvedInVersion searches the version's issues whose status category
// is not Done, page by page, with no scope beyond the version: Commit asks
// this just before a release so issues the profile's scope JQL hides are
// not left unfinished in a released version.
func (b *Backend) UnresolvedInVersion(ctx context.Context, id int) ([]string, error) {
	jql := fmt.Sprintf("fixVersion = %d AND statusCategory != Done ORDER BY key", id)
	keys := []string{}
	for start := 0; ; {
		page, err := b.c.SearchIssues(ctx, jql, []string{"summary"}, nil, start, unresolvedPage)
		if err != nil {
			return nil, err
		}
		for _, raw := range page.Issues {
			keys = append(keys, raw.Key)
		}
		start += len(page.Issues)
		if len(page.Issues) == 0 || start >= page.Total {
			return keys, nil
		}
	}
}

func idOrEmpty(id int) string {
	if id <= 0 {
		return ""
	}
	return strconv.Itoa(id)
}
