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
// issueType is the profile's configured defect issuetype (default "Bug") used to
// recognize which linked issues are defects. Demo URLs generate a deterministic
// cross-project set; the real path is empty until verified on a live instance.
//
// TODO(xtm): real path — read each synced Test's issuelinks (already fetched
// during the test sync) and keep links whose target issuetype matches the
// configured issueType; batch-fetch those issues by key via
// /rest/api/2/search?jql=key in (...) so cross-project bugs resolve. Verify the
// link direction and issuetype names on a live Xray Server 8.4.0 instance.
func (c *Client) ListBugs(ctx context.Context, testProjectKey string, testKeys []string, issueType string, onProgress func(done, total int)) ([]Bug, []BugLink, error) {
	_ = ctx
	if isDemoURL(c.baseURL) {
		bugs, links := demoBugs(testProjectKey)
		if onProgress != nil {
			onProgress(len(bugs), len(bugs))
		}
		return bugs, links, nil
	}
	_ = testKeys
	_ = issueType
	return []Bug{}, []BugLink{}, nil
}

// CreateBug creates a defect issue of the given issuetype (the profile's
// configured bug issue type, default "Bug") and returns its key. Demo URLs
// return a synthetic key.
//
// Maps to POST /rest/api/2/issue with fields {project, issuetype:{name:issueType},
// summary, description, priority, labels}. NOTE(xtm): verify the project's
// issuetype + required fields on a live instance.
func (c *Client) CreateBug(ctx context.Context, projectKey, issueType, summary, description, priority string, labels []string) (string, error) {
	_ = ctx
	if isDemoURL(c.baseURL) {
		return fmt.Sprintf("%s-BUG-DEMO", projectKey), nil
	}
	_ = issueType
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

// Two separate defect-tracking projects (both distinct from the test project) so
// the demo shows the feature's cross-project capability — defects rarely live in
// the same project as the tests.
const (
	demoBugProject  = "BUGS"
	demoBugProject2 = "SUP"
)

var demoBugStatuses = []string{"Open", "In Progress", "Reopened", "Done"}
var demoBugPriorities = []string{"Critical", "High", "Medium", "Low"}
var demoBugSummaries = []string{
	"crashes on submit", "returns HTTP 500", "wrong total displayed",
	"times out under load", "validation is bypassed", "data is not persisted",
	"UI freezes intermittently", "incorrect permission check",
	"race condition on save", "leaks memory over time", "off-by-one in pagination",
	"stale cache after edit",
}

// demoFailedTestNums returns the 1-based numbers of demo Tests that carry a FAIL
// run status in their primary execution. Derived from the same demoRunStatuses
// mapping demoContainersAndLinks uses, so the bug seed stays in sync with which
// tests the demo actually fails — keeping the story coherent (a failed test has
// a filed defect).
func demoFailedTestNums(limit int) []int {
	out := make([]int, 0, limit)
	for i := 0; i < demoLinkedTests && i < demoTestCount; i++ {
		if demoRunStatuses[i%len(demoRunStatuses)] == "FAIL" {
			out = append(out, i+1)
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

// demoBugs seeds defect issues across two non-test projects, each linked to a
// demo Test that is actually marked FAILED, plus a test with two defects and a
// defect spanning two tests — so the Bugs panel and the test-detail section show
// realistic, cross-project data.
func demoBugs(testProjectKey string) ([]Bug, []BugLink) {
	if testProjectKey == "" {
		testProjectKey = "DEMO"
	}
	failed := demoFailedTestNums(10)
	if len(failed) < 3 {
		return []Bug{}, []BugLink{}
	}

	projects := []string{demoBugProject, demoBugProject2}
	bugs := []Bug{}
	links := []BugLink{}

	addBug := func(testNum int) string {
		n := len(bugs)
		project := projects[n%len(projects)]
		key := fmt.Sprintf("%s-%d", project, 100+n)
		bugs = append(bugs, Bug{
			Key:        key,
			ProjectKey: project,
			IssueType:  "Bug",
			Summary:    fmt.Sprintf("%s-%d %s", testProjectKey, testNum, demoBugSummaries[n%len(demoBugSummaries)]),
			Status:     demoBugStatuses[n%len(demoBugStatuses)],
			Priority:   demoBugPriorities[n%len(demoBugPriorities)],
		})
		return key
	}
	link := func(testNum int, bugKey string) {
		links = append(links, BugLink{
			TestKey: fmt.Sprintf("%s-%d", testProjectKey, testNum),
			BugKey:  bugKey,
			LinkID:  fmt.Sprintf("bl-%d", len(links)+1),
		})
	}

	// One defect per failed test.
	for _, n := range failed {
		link(n, addBug(n))
	}
	// The first failed test carries a second defect (a test with multiple bugs).
	link(failed[0], addBug(failed[0]))
	// One defect spans two failed tests (a bug affecting more than one test).
	spanKey := addBug(failed[1])
	link(failed[1], spanKey)
	link(failed[2], spanKey)

	return bugs, links
}
