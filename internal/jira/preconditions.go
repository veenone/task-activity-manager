package jira

import "context"

// Precondition mirrors a Xray Precondition issue (FR-13.4).
type Precondition struct {
	Key         string
	Summary     string
	Type        string
	Description string
}

// ListPreconditions returns all Preconditions for a project plus a mapping
// from Test key to the keys of the Preconditions linked to it. Demo URLs
// short-circuit to generated data; the real-Jira call is a best-effort no-op
// pending verification against an actual Xray Server 8.4.0 instance.
//
// TODO(xtm): wire to /rest/raven/2.0/api/test/{key}/preconditions (or the
// project-wide equivalent) once we can verify the response shape on a live
// instance.
func (c *Client) ListPreconditions(ctx context.Context, projectKey string) ([]Precondition, map[string][]string, error) {
	_ = ctx
	if isDemoURL(c.baseURL) {
		return demoPreconditionsAndLinks(projectKey)
	}
	return nil, nil, nil
}
