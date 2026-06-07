package jira

import "context"

// CreatePrecondition creates a new Precondition issue and returns its key
// (FR-13.5). Demo URLs short-circuit to a no-op, returning an empty key (the
// placeholder is reconciled on the next sync). The real-Jira call is a
// best-effort no-op pending verification against an actual Xray Server 8.4.0
// instance.
//
// TODO(xtm): POST /rest/api/2/issue with issuetype "Precondition" and the Xray
// precondition-type custom field, then return the new key — verify the field
// id and response on a live instance.
func (c *Client) CreatePrecondition(ctx context.Context, projectKey, summary, ptype, description string) (string, error) {
	_ = ctx
	if isDemoURL(c.baseURL) {
		return "", nil
	}
	return "", nil
}

// UpdateTestPreconditions associates / disassociates Preconditions with a Test
// (FR-13.5 / 13.6). add and remove are Precondition keys. Demo URLs
// short-circuit to a no-op; the real-Jira call is a best-effort no-op pending
// verification against an actual Xray Server 8.4.0 instance.
//
// TODO(xtm): wire to the Xray precondition-association endpoint
// (POST /rest/raven/2.0/api/test/{key}/preconditions with {add:[], remove:[]})
// once the response shape can be verified on a live instance.
func (c *Client) UpdateTestPreconditions(ctx context.Context, testKey string, add, remove []string) error {
	_ = ctx
	if len(add) == 0 && len(remove) == 0 {
		return nil
	}
	if isDemoURL(c.baseURL) {
		return nil
	}
	return nil
}

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
