package jira

import (
	"context"
	"net/url"
)

// MyPermissions reads what the token may do in one project: every permission
// Jira knows, by key, with whether this account holds it there.
//
// The endpoint takes a `permissions` filter, which the instance this was
// written against ignored, answering the full list. So this asks for
// everything and leaves the caller to read the keys it needs.
func (c *Client) MyPermissions(ctx context.Context, projectKey string) (map[string]bool, error) {
	var resp struct {
		Permissions map[string]struct {
			HavePermission bool `json:"havePermission"`
		} `json:"permissions"`
	}
	if err := c.Get(ctx, "/rest/api/2/mypermissions?projectKey="+url.QueryEscape(projectKey), &resp); err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(resp.Permissions))
	for key, p := range resp.Permissions {
		out[key] = p.HavePermission
	}
	return out, nil
}
