// Identity on a Zephyr instance: the Jira issue id behind an issue key, and
// the name of the issue type Zephyr treats as a test.
//
// Both exist because Zephyr and Jira disagree about how to name the same
// thing. Zephyr's step endpoints take the numeric issue id while every caller
// in XTM holds an issue key, and Zephyr reports the id of its test issue type
// while Jira's searches look for one by name.
//
// Measured facts behind this file are in agents/project/domain-context.md.

package zephyr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
)

// JiraIssueID is the numeric Jira issue id for an issue key, which is what
// Zephyr's step endpoints take. GET /teststep/PROJ-101 answers 400; the same
// call with 10501 answers the steps.
//
// The answer is cached per key for the life of the client, because a test's
// steps are read again every time the user selects it and the id of an issue
// never changes.
func (c *Client) JiraIssueID(ctx context.Context, key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("zephyr: no issue key to resolve")
	}
	return c.issueIDs.get(key, func() (string, error) {
		var issue struct {
			ID string `json:"id"`
		}
		// fields=id keeps the response small. Jira returns the issue id at the
		// top level whatever is asked for, so this never asks for content.
		path := "/rest/api/2/issue/" + url.PathEscape(key) + "?fields=id"
		if err := c.jira.Get(ctx, path, &issue); err != nil {
			return "", fmt.Errorf("zephyr: resolve the Jira issue id of %s: %w", key, err)
		}
		if strings.TrimSpace(issue.ID) == "" {
			return "", fmt.Errorf("zephyr: Jira returned no issue id for %s", key)
		}
		return issue.ID, nil
	})
}

// TestIssueTypeName is the display name of the issue type Zephyr uses for a
// test here, resolved from the id Zephyr reports. An empty name with no error
// means the instance did not say: reading the issue type list needs permission
// a plain user may not have, and that is not evidence of a problem.
func (c *Client) TestIssueTypeName(ctx context.Context) (string, error) {
	id, err := c.TestIssueTypeID(ctx)
	if err != nil {
		return "", err
	}
	var types []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := c.jira.Get(ctx, "/rest/api/2/issuetype", &types); err != nil {
		return "", nil
	}
	for _, t := range types {
		if t.ID == id {
			return strings.TrimSpace(t.Name), nil
		}
	}
	return "", nil
}

// verifyTestIssueTypeName refuses the one instance that would otherwise
// connect cleanly and then show an empty project.
//
// A Zephyr test is an ordinary Jira issue, so the test reads are Jira searches
// for an issue type named Test. Zephyr reports an id instead, and the two only
// agree while the type carries its default name. When they disagree the
// searches return nothing at all, which reads as a sync that did not run
// rather than as a configuration XTM cannot follow.
//
// TODO(xtm) #371: the searches could take the id rather than the name, which
// would remove the restriction instead of reporting it. That means changing
// the JQL the Jira client builds for every backend, so it belongs to no phase
// of this plan and needs an issue of its own.
func (a *Adapter) verifyTestIssueTypeName(ctx context.Context) error {
	name, err := a.z.TestIssueTypeName(ctx)
	if err != nil {
		return err
	}
	if name == "" || strings.EqualFold(name, "Test") {
		return nil
	}
	return fmt.Errorf("zephyr uses the issue type %q for tests on this instance, and XTM's searches look for one named Test, "+
		"so this profile would list no tests", name)
}

// keyedCache holds one answer per key for the life of the client. Failures are
// not cached, for the reason recorded on cache in client.go.
type keyedCache struct {
	mu sync.Mutex
	m  map[string]string
}

func (k *keyedCache) get(key string, load func() (string, error)) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if v, ok := k.m[key]; ok {
		return v, nil
	}
	v, err := load()
	if err != nil {
		return "", err
	}
	if k.m == nil {
		k.m = make(map[string]string)
	}
	k.m[key] = v
	return v, nil
}

// jsonString reads a JSON value that may arrive quoted or bare as a string.
// Zephyr is inconsistent about it within one response: a traceability
// execution carries its id as "2001" where the execution read carries 2001.
func jsonString(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return ""
	}
	return strings.Trim(s, `"`)
}
