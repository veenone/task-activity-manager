package jira

import "context"

// Folder mirrors a node in the Xray Test Repository tree (FR-13.1).
type Folder struct {
	ID       string
	ParentID string
	Name     string
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
