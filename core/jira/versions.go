package jira

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// Version is one Jira fix version. TAM calls it a release.
type Version struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	StartDate   string `json:"startDate"`
	ReleaseDate string `json:"releaseDate"`
	Released    bool   `json:"released"`
	Archived    bool   `json:"archived"`
}

// VersionInput is what create and update send. Project is used on create
// only; Jira does not move a version between projects.
type VersionInput struct {
	Project     string
	Name        string
	Description string
	StartDate   string
	ReleaseDate string
}

// ProjectVersions lists every fix version of the project, released and
// archived ones included.
func (c *Client) ProjectVersions(ctx context.Context, projectKey string) ([]Version, error) {
	var out []Version
	path := fmt.Sprintf("/rest/api/2/project/%s/versions", url.PathEscape(projectKey))
	if err := c.Get(ctx, path, &out); err != nil {
		return nil, fmt.Errorf("project versions %s: %w", projectKey, err)
	}
	return out, nil
}

// CreateVersion creates a version in in.Project. Empty optional fields are
// left out rather than sent as "", which Jira refuses for a date.
func (c *Client) CreateVersion(ctx context.Context, in VersionInput) (Version, error) {
	body := map[string]string{"project": in.Project, "name": in.Name}
	for key, v := range map[string]string{"description": in.Description, "startDate": in.StartDate, "releaseDate": in.ReleaseDate} {
		if v != "" {
			body[key] = v
		}
	}
	var out Version
	if err := c.writeChecked(ctx, http.MethodPost, "/rest/api/2/version", body, &out); err != nil {
		return Version{}, fmt.Errorf("create version %q: %w", in.Name, err)
	}
	return out, nil
}

// UpdateVersion replaces a version's name, description and dates. An empty
// date goes as null, which is how an edit clears it.
func (c *Client) UpdateVersion(ctx context.Context, id string, in VersionInput) (Version, error) {
	body := map[string]any{"name": in.Name, "description": in.Description,
		"startDate": dateOrNull(in.StartDate), "releaseDate": dateOrNull(in.ReleaseDate)}
	var out Version
	if err := c.writeChecked(ctx, http.MethodPut, versionPath(id), body, &out); err != nil {
		return Version{}, fmt.Errorf("update version %s: %w", id, err)
	}
	return out, nil
}

// ReleaseVersion marks a version released on releaseDate. With moveUnfixedTo
// set, Jira first moves the version's unresolved issues to that version; the
// field takes the version's URI, not its id.
func (c *Client) ReleaseVersion(ctx context.Context, id, releaseDate, moveUnfixedTo string) error {
	body := map[string]any{"released": true, "releaseDate": releaseDate}
	if moveUnfixedTo != "" {
		body["moveUnfixedIssuesTo"] = c.BaseURL() + versionPath(moveUnfixedTo)
	}
	if err := c.writeChecked(ctx, http.MethodPut, versionPath(id), body, nil); err != nil {
		return fmt.Errorf("release version %s: %w", id, err)
	}
	return nil
}

// ArchiveVersion archives a version, or unarchives it with archived false.
func (c *Client) ArchiveVersion(ctx context.Context, id string, archived bool) error {
	if err := c.writeChecked(ctx, http.MethodPut, versionPath(id), map[string]bool{"archived": archived}, nil); err != nil {
		return fmt.Errorf("archive version %s: %w", id, err)
	}
	return nil
}

// DeleteVersion deletes a version. With moveFixIssuesTo set, Jira first moves
// the issues fixed in it to that version.
func (c *Client) DeleteVersion(ctx context.Context, id, moveFixIssuesTo string) error {
	path := versionPath(id)
	if moveFixIssuesTo != "" {
		path += "?moveFixIssuesTo=" + url.QueryEscape(moveFixIssuesTo)
	}
	if err := c.deleteChecked(ctx, path); err != nil {
		return fmt.Errorf("delete version %s: %w", id, err)
	}
	return nil
}

func versionPath(id string) string { return "/rest/api/2/version/" + url.PathEscape(id) }

func dateOrNull(d string) any {
	if d == "" {
		return nil
	}
	return d
}
