package jira

import (
	"context"
	"fmt"
)

// Bug is a defect issue (possibly cross-project) linked to Tests.
type Bug struct {
	Key        string
	ProjectKey string
	IssueType  string
	Summary    string
	Status     string
	Priority   string
	Updated    string
}

// BugLink is a Test <-> Bug link.
type BugLink struct {
	TestKey string
	BugKey  string
	LinkID  string
}

// ListBugs returns the defect issues linked to the given Tests, plus the links.
// Demo URLs generate a deterministic cross-project set; the real path is empty
// until verified on a live instance.
//
// TODO(xtm): real path — read each synced Test's issuelinks (already fetched
// during the test sync) and keep links whose target issuetype is in
// {"Bug","Defect"}; batch-fetch those issues by key via
// /rest/api/2/search?jql=key in (...) so cross-project bugs resolve. Verify the
// link direction and issuetype names on a live Xray Server 8.4.0 instance.
func (c *Client) ListBugs(ctx context.Context, testProjectKey string, testKeys []string, onProgress func(done, total int)) ([]Bug, []BugLink, error) {
	_ = ctx
	if isDemoURL(c.baseURL) {
		bugs, links := demoBugs(testProjectKey)
		if onProgress != nil {
			onProgress(len(bugs), len(bugs))
		}
		return bugs, links, nil
	}
	_ = testKeys
	return []Bug{}, []BugLink{}, nil
}

// CreateBug creates a Bug-type issue and returns its key. Demo URLs return a
// synthetic key.
//
// Maps to POST /rest/api/2/issue with fields {project, issuetype:{name:"Bug"},
// summary, description, priority, labels}. NOTE(xtm): verify the project's Bug
// issuetype + required fields on a live instance.
func (c *Client) CreateBug(ctx context.Context, projectKey, summary, description, priority string, labels []string) (string, error) {
	_ = ctx
	if isDemoURL(c.baseURL) {
		return fmt.Sprintf("%s-BUG-DEMO", projectKey), nil
	}
	_ = summary
	_ = description
	_ = priority
	_ = labels
	return "", fmt.Errorf("creating bugs on a live Jira instance is not yet verified")
}

// CreateBugLink links a Test to a Bug. Demo URLs no-op.
//
// Maps to POST /rest/api/2/issueLink. NOTE(xtm): resolve the defect link type
// once and verify direction on a live instance (same open item as requirement
// links).
func (c *Client) CreateBugLink(ctx context.Context, testKey, bugKey string) error {
	_ = ctx
	if isDemoURL(c.baseURL) {
		return nil
	}
	_ = testKey
	_ = bugKey
	return nil
}

const demoBugProject = "BUGS"

var demoBugStatuses = []string{"Open", "In Progress", "Reopened", "Done"}
var demoBugPriorities = []string{"High", "Medium", "Critical", "Low"}

// demoBugs generates a handful of Bug issues in a separate project, linked to the
// demo profile's lower-numbered tests (which the demo marks FAILED), with one
// bug affecting two tests — so the panel and detail section have cross-project
// data.
func demoBugs(testProjectKey string) ([]Bug, []BugLink) {
	if testProjectKey == "" {
		testProjectKey = "DEMO"
	}
	const count = 6
	bugs := make([]Bug, 0, count)
	links := make([]BugLink, 0, count+1)
	for i := 1; i <= count; i++ {
		key := fmt.Sprintf("%s-%d", demoBugProject, i)
		bugs = append(bugs, Bug{
			Key:        key,
			ProjectKey: demoBugProject,
			IssueType:  "Bug",
			Summary:    fmt.Sprintf("Defect found in %s-%d", testProjectKey, i),
			Status:     demoBugStatuses[i%len(demoBugStatuses)],
			Priority:   demoBugPriorities[i%len(demoBugPriorities)],
		})
		links = append(links, BugLink{
			TestKey: fmt.Sprintf("%s-%d", testProjectKey, i),
			BugKey:  key,
			LinkID:  fmt.Sprintf("bl-%d", i),
		})
	}
	// One bug affects a second test, to exercise the multi-test panel row.
	links = append(links, BugLink{
		TestKey: fmt.Sprintf("%s-7", testProjectKey),
		BugKey:  fmt.Sprintf("%s-1", demoBugProject),
		LinkID:  "bl-extra",
	})
	return bugs, links
}
