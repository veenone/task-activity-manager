package jira

import (
	"context"
	"fmt"
)

// Container kinds — the three Xray issue types that group Tests. The values
// match the Xray REST path segments (testset / testplan / testexec).
const (
	KindTestSet  = "testset"
	KindTestPlan = "testplan"
	KindTestExec = "testexec"
)

// Container mirrors a Xray Test Set, Test Plan or Test Execution issue
// (FR-1.3). The three share a shape — a key, a summary and a workflow status —
// and differ only in how they relate to Tests, so one type with a Kind
// discriminator avoids three near-identical structs.
type Container struct {
	Key     string
	Kind    string
	Summary string
	Status  string
}

// ContainerLink is one Test's membership in a Container. RunStatus carries the
// Test Run result (PASS / FAIL / TODO / …) when the Container is a Test
// Execution; it is empty for Test Sets and Test Plans, which are plain
// memberships.
type ContainerLink struct {
	ContainerKey string
	TestKey      string
	RunStatus    string
}

// ListContainers returns every Test Set, Test Plan and Test Execution for a
// project plus the Test memberships that link them to Tests. Demo URLs
// short-circuit to generated data; the real-Jira call is a best-effort no-op
// pending verification against an actual Xray Server 8.4.0 instance.
//
// TODO(xtm): wire to the Xray Server REST endpoints — JQL search by issuetype
// for the containers, then /rest/raven/2.0/api/{testset,testplan,testexec}/
// {key}/test for memberships (the testexec variant returns run status) — once
// the response shapes can be verified on a live instance.
func (c *Client) ListContainers(ctx context.Context, projectKey string) ([]Container, []ContainerLink, error) {
	_ = ctx
	if isDemoURL(c.baseURL) {
		return demoContainersAndLinks(projectKey)
	}
	return nil, nil, nil
}

// containerPathSegment maps a Container kind to its Xray REST path segment.
func containerPathSegment(kind string) (string, error) {
	switch kind {
	case KindTestSet:
		return "testset", nil
	case KindTestPlan:
		return "testplan", nil
	case KindTestExec:
		return "testexec", nil
	}
	return "", fmt.Errorf("unknown container kind %q", kind)
}

// AddTestsToContainer adds Tests to a Test Set, Test Plan or Test Execution
// (FR-3.4–3.6, add-only). Demo URLs short-circuit to a no-op.
//
// Maps to POST /rest/raven/2.0/api/{testset|testplan|testexec}/{key}/test with
// an {"add": [keys]} body. NOTE(xtm): the endpoint + body shape are assumed
// from the Xray Server/DC API and need verification on a live instance.
func (c *Client) AddTestsToContainer(ctx context.Context, kind, containerKey string, testKeys []string) error {
	if len(testKeys) == 0 {
		return nil
	}
	if isDemoURL(c.baseURL) {
		return nil
	}
	segment, err := containerPathSegment(kind)
	if err != nil {
		return err
	}
	body := map[string]any{"add": testKeys}
	return c.post(ctx, fmt.Sprintf("/rest/raven/2.0/api/%s/%s/test", segment, containerKey), body)
}
