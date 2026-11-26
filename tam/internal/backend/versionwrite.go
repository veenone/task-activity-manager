package backend

import "context"

// VersionInput is what a release's create and update send: the name, the
// description and the two dates, each date YYYY-MM-DD or empty for none.
type VersionInput struct {
	Name        string
	Description string
	StartDate   string
	ReleaseDate string
}

// VersionBackend writes the project's fix versions, which Commit's releases
// and closing phases push (#605). Optional, like CatalogBackend: a backend
// without it fails each release change with a reason.
type VersionBackend interface {
	CreateVersion(ctx context.Context, projectKey string, in VersionInput) (Version, error)
	UpdateVersion(ctx context.Context, id int, in VersionInput) (Version, error)
	// ReleaseVersion marks the version released on releaseDate. A
	// moveUnfixedTo above 0 has Jira move the version's unresolved issues
	// to that version in the same call.
	ReleaseVersion(ctx context.Context, id int, releaseDate string, moveUnfixedTo int) error
	ArchiveVersion(ctx context.Context, id int, archived bool) error
	// DeleteVersion deletes the version, moving its issues to
	// moveFixIssuesTo when that is above 0.
	DeleteVersion(ctx context.Context, id, moveFixIssuesTo int) error
	// UnresolvedInVersion lists the keys of the version's issues whose
	// status category is not Done, whatever the profile's scope.
	UnresolvedInVersion(ctx context.Context, id int) ([]string, error)
}

// Finished says whether an issue counts as done for a release: Jira's
// status category, or the status name when the row has no category. The
// Releases view's progress and the Release dialog's unfinished list both
// read it, so the two always add up to the release's issue count.
func Finished(iss Issue) bool {
	if iss.StatusCategory != "" {
		return iss.StatusCategory == "done"
	}
	return IsDone(iss.Status)
}
