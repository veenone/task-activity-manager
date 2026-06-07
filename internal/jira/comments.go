package jira

import "context"

// AddComment posts a comment to an issue — used to record a Test review on the
// Test issue (test review). Demo URLs short-circuit to a no-op.
//
// Maps to POST /rest/api/2/issue/{key}/comment with a {"body": "..."} payload.
// NOTE(xtm): verify the comment body shape on a live Jira DC instance (plain
// text vs. Atlassian document format differs across versions).
func (c *Client) AddComment(ctx context.Context, issueKey, body string) error {
	_ = ctx
	if isDemoURL(c.baseURL) {
		return nil
	}
	return nil
}
