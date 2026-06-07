package jira

import "context"

// Folder mirrors a node in the Xray Test Repository tree (FR-13.1).
type Folder struct {
	ID       string
	ParentID string
	Name     string
}

// CreateFolder / RenameFolder / DeleteFolder manage the Test Repository tree
// (FR-13.3). Demo URLs short-circuit to no-ops; the real-Jira calls are
// best-effort no-ops pending verification against an actual Xray Server 8.4.0
// instance.
//
// TODO(xtm): wire to the Xray test-repository folder endpoints (create / rename
// / delete under /rest/raven/2.0/api/testrepository/{project}/folders) once the
// request and response shapes can be verified on a live instance.
func (c *Client) CreateFolder(ctx context.Context, projectKey, parentPath, name string) error {
	_ = ctx
	if isDemoURL(c.baseURL) {
		return nil
	}
	return nil
}

func (c *Client) RenameFolder(ctx context.Context, projectKey, path, newName string) error {
	_ = ctx
	if isDemoURL(c.baseURL) {
		return nil
	}
	return nil
}

func (c *Client) DeleteFolder(ctx context.Context, projectKey, path string) error {
	_ = ctx
	if isDemoURL(c.baseURL) {
		return nil
	}
	return nil
}

// MoveTestToFolder relocates a Test within the project's Test Repository tree
// (FR-13.3). folderID is the full folder path ("/Authentication/Login"), or
// empty for the repository root. Demo URLs short-circuit to a no-op; the
// real-Jira call is a best-effort no-op pending verification against an actual
// Xray Server 8.4.0 instance.
//
// TODO(xtm): wire to the Xray test-repository move endpoint
// (POST .../testrepository/{project}/folders/{folderId}/tests with {add:[key]})
// once the response shape can be verified on a live instance.
func (c *Client) MoveTestToFolder(ctx context.Context, projectKey, testKey, folderID string) error {
	_ = ctx
	if isDemoURL(c.baseURL) {
		return nil
	}
	return nil
}

// ListFolders returns the Test Repository folder tree for a project. Demo
// URLs short-circuit to a generated hierarchy; the real-Jira call is a
// best-effort no-op pending verification against an actual Xray Server
// 8.4.0 instance.
//
// TODO(xtm): wire to /rest/raven/2.0/api/testrepository/{project}/folders
// once we can verify the response shape on a live instance.
func (c *Client) ListFolders(ctx context.Context, projectKey string) ([]Folder, error) {
	_ = ctx
	if isDemoURL(c.baseURL) {
		return demoFolders(projectKey), nil
	}
	return nil, nil
}
