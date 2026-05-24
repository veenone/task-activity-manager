package jira

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// Test is a Jira issue of type Test, flattened to the fields the app caches.
type Test struct {
	Key         string
	ID          string
	Summary     string
	Description string
	Status      string
	Priority    string
	Labels      []string
	Updated     string
	FolderID    string
}

// testFields are the issue fields requested from Jira's search API.
const testFields = "summary,description,status,priority,labels,updated"

// searchResponse is the /rest/api/2/search payload.
type searchResponse struct {
	Total  int `json:"total"`
	Issues []struct {
		ID     string `json:"id"`
		Key    string `json:"key"`
		Fields struct {
			Summary     string   `json:"summary"`
			Description string   `json:"description"`
			Updated     string   `json:"updated"`
			Labels      []string `json:"labels"`
			Status      *struct {
				Name string `json:"name"`
			} `json:"status"`
			Priority *struct {
				Name string `json:"name"`
			} `json:"priority"`
		} `json:"fields"`
	} `json:"issues"`
}

// SearchTestsPage fetches one page of Test issues for a project, beginning
// at startAt. If `since` is non-empty (RFC3339), only Tests updated at or
// after that time are returned — the incremental-sync path (FR-1.2). The
// caller pages until the total reported by Jira is reached.
func (c *Client) SearchTestsPage(ctx context.Context, projectKey, since string, startAt, maxResults int) ([]Test, int, error) {
	if isDemoURL(c.baseURL) {
		// Demo mode ignores `since` so an incremental sync still fills the
		// progress bar against the regenerated dataset.
		tests, total := demoTestsPage(projectKey, startAt, maxResults)
		return tests, total, nil
	}
	jql := fmt.Sprintf("project = %s AND issuetype = Test", projectKey)
	if extra := incrementalSinceClause(since); extra != "" {
		jql += " AND " + extra
	}
	jql += " ORDER BY updated ASC"

	q := url.Values{}
	q.Set("jql", jql)
	q.Set("startAt", strconv.Itoa(startAt))
	q.Set("maxResults", strconv.Itoa(maxResults))
	q.Set("fields", testFields)

	var resp searchResponse
	if err := c.get(ctx, "/rest/api/2/search?"+q.Encode(), &resp); err != nil {
		return nil, 0, err
	}

	tests := make([]Test, 0, len(resp.Issues))
	for _, iss := range resp.Issues {
		t := Test{
			Key:         iss.Key,
			ID:          iss.ID,
			Summary:     iss.Fields.Summary,
			Description: iss.Fields.Description,
			Updated:     iss.Fields.Updated,
			Labels:      iss.Fields.Labels,
		}
		if iss.Fields.Status != nil {
			t.Status = iss.Fields.Status.Name
		}
		if iss.Fields.Priority != nil {
			t.Priority = iss.Fields.Priority.Name
		}
		tests = append(tests, t)
	}
	return tests, resp.Total, nil
}

// incrementalSinceClause builds the JQL fragment for an updated-since filter.
// A 1-hour safety buffer is subtracted from the watermark to absorb clock
// skew and Jira's server-timezone interpretation of bare date literals — a
// small overlap is harmless because UpsertTests is idempotent.
func incrementalSinceClause(rfc3339 string) string {
	if rfc3339 == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return ""
	}
	return fmt.Sprintf(`updated >= "%s"`,
		t.Add(-time.Hour).Format("2006-01-02 15:04"))
}

// TODO(xtm): Test Steps (Xray /rest/raven/2.0/api/test/{key}/step) are fetched
// lazily on the detail view, not during sync — one call per Test would be
// 10k+ requests. Test Repository folders and Preconditions (FR-13) follow.
