package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// What a create made, as FindCreatedIssues takes it; the container kinds
// (KindTestSet, KindTestPlan, KindTestExec) are used as they are. The values
// match backend.CreateKind*, which the adapter passes through.
const (
	CreatedKindTest         = "test"
	CreatedKindPrecondition = "precondition"
	CreatedKindIssue        = "issue"
)

// CreatedIssue is one issue FindCreatedIssues found.
type CreatedIssue struct {
	Key     string
	Summary string
}

// createdIssuesPage is how many issues one page of the duplicate check asks
// for.
const createdIssuesPage = 100

// FindCreatedIssues lists the issues of one kind the connected user created
// in projectKey since the given time, with their summaries (#607). kind is a
// CreatedKind* or container kind; issueType names the type for
// CreatedKindIssue and may be empty to match any type.
//
// The window is relative ("created >= -Nm") rather than a date, because JQL
// reads a date in the user's Jira time zone, which this machine does not know.
// Demo URLs have nothing to find.
func (c *Client) FindCreatedIssues(ctx context.Context, projectKey, kind, issueType string, since time.Time) ([]CreatedIssue, error) {
	if isDemoURL(c.baseURL) {
		return nil, nil
	}
	typeClause, err := c.createdTypeClause(ctx, kind, issueType)
	if err != nil {
		return nil, err
	}
	minutes := int(math.Ceil(time.Since(since).Minutes()))
	if minutes < 1 {
		minutes = 1
	}
	jql := fmt.Sprintf(`project = "%s"%s AND reporter = currentUser() AND created >= "-%dm" ORDER BY created ASC`,
		jqlEscape(projectKey), typeClause, minutes)
	var out []CreatedIssue
	for startAt := 0; ; {
		page, err := c.SearchIssues(ctx, jql, []string{"summary"}, nil, startAt, createdIssuesPage)
		if err != nil {
			return nil, err
		}
		for _, iss := range page.Issues {
			var summary string
			_ = json.Unmarshal(iss.Fields["summary"], &summary)
			out = append(out, CreatedIssue{Key: iss.Key, Summary: summary})
		}
		startAt += len(page.Issues)
		if len(page.Issues) == 0 || startAt >= page.Total {
			return out, nil
		}
	}
}

// createdTypeClause is the " AND issuetype = ..." part of the duplicate
// check, resolved the same way the matching create resolves its type.
func (c *Client) createdTypeClause(ctx context.Context, kind, issueType string) (string, error) {
	switch kind {
	case CreatedKindTest, CreatedKindPrecondition:
		resolve := c.resolveTestType
		if kind == CreatedKindPrecondition {
			resolve = c.resolvePreconditionType
		}
		id, _, err := resolve(ctx)
		if err != nil {
			return "", fmt.Errorf("resolve %s issue type: %w", kind, err)
		}
		if id == "" {
			return "", fmt.Errorf("this Jira instance has no %s issue type", kind)
		}
		return " AND issuetype = " + id, nil
	case CreatedKindIssue:
		if issueType == "" {
			return "", nil
		}
		return fmt.Sprintf(` AND issuetype = "%s"`, jqlEscape(issueType)), nil
	}
	name, err := containerIssueType(kind)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(` AND issuetype = "%s"`, name), nil
}
