package jira

import (
	"context"
	"errors"
	"net/http"

	corejira "agile-suite/core/jira"
)

// getCreatemeta reads the classic create-metadata endpoint into out and reports
// whether the instance still has that endpoint.
//
// Jira 9 removed GET /rest/api/2/issue/createmeta. Where it is gone the path
// matches /rest/api/2/issue/{issueIdOrKey} instead, with "createmeta" read as
// the key, so the answer is a 404 carrying Jira's "Issue Does Not Exist" for a
// call that never mentioned an issue (#313).
//
// Every caller already knows what to do when createmeta tells it nothing: list
// the instance's custom fields instead, or offer no extra fields. Each of them
// only did that for an instance that answered, though, so a removed endpoint
// failed the whole operation instead, and for the sync that meant no sync at
// all. A false return means "this instance cannot answer, ask something else".
// Any other failure is still returned: a 401 will not read better for having
// been turned into an empty answer.
//
// core/jira solves the same problem from the other side: CreateMeta asks the
// per-type endpoint first and falls back to the classic one when that 404s.
// Reusing it here would mean resolving an issue type id per call, which these
// callers do not have, so they share this instead.
func (c *Client) getCreatemeta(ctx context.Context, path string, out any) (bool, error) {
	err := c.get(ctx, path, out)
	if err == nil {
		return true, nil
	}
	var he *corejira.HTTPError
	if errors.As(err, &he) && he.Code == http.StatusNotFound {
		return false, nil
	}
	return false, err
}
