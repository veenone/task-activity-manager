// The writes a Zephyr profile can make today: a run result, a cycle, and the
// tests in one.
//
// These three are the part of #371 Phase 5 that does not wait on Phase 4. The
// endpoints SmartBear documents without a measured request or response, the
// step writes and every delete among them, stay in unsupported.go until they
// have been run against a scratch project: a wrong guess at a delete loses
// someone's records, while a wrong guess at these three is refused by the
// instance and reported.
//
// Nothing here is reached outside Commit. XTM journals the change locally and
// the commit engine is the only caller, which is the local-first contract
// every other backend follows.

package zephyr

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"agile-suite/xtm/internal/backend"
)

// jobPollInterval and jobPollAttempts bound the wait for an asynchronous add.
// They are variables so a test can shorten the wait; nothing else sets them.
var (
	jobPollInterval = 300 * time.Millisecond
	jobPollAttempts = 60
)

// SetTestRunStatus records a result against one test's execution in a cycle.
//
// It is a two-step write. XTM hands over a cycle key, a test key and a status
// name; Zephyr needs a numeric execution id and this instance's numeric status
// id, and neither is in the call. The execution comes from traceability, which
// was measured, and the status from the vocabulary read at connect.
//
// Both lookups can fail in ways worth saying out loud, so they do: a test that
// is not in that cycle and a status the instance does not define are different
// problems and neither is a reason to write something approximate.
func (a *Adapter) SetTestRunStatus(ctx context.Context, execKey, testKey, status string) error {
	vocabulary, err := a.z.ExecutionStatuses(ctx)
	if err != nil {
		return fmt.Errorf("read this instance's result vocabulary: %w", err)
	}
	statusID, err := vocabulary.IDFor(status)
	if err != nil {
		return err
	}
	executionID, err := a.executionID(ctx, execKey, testKey)
	if err != nil {
		return err
	}
	// The body is the one part of this write the probe could not confirm: the
	// path is documented, the field is not. A wrong field name is refused by
	// the instance rather than silently writing the wrong result, and
	// bodyError catches the 200-with-errorId case that would otherwise read as
	// a successful write and clear the pending change.
	body := map[string]any{"status": statusID}
	if err := a.z.writeJSON(ctx, "PUT", "/execution/"+url.PathEscape(executionID)+"/execute", body, nil); err != nil {
		return fmt.Errorf("set the result of %s in cycle %s: %w", testKey, execKey, err)
	}
	return nil
}

// executionID finds the execution a test has in one cycle.
//
// Traceability answers for the test rather than for the cycle, so the rows are
// filtered by cycle here. A row that carries no cycle id is read in full, as
// the read path does, because the id is what the containers are keyed by.
func (a *Adapter) executionID(ctx context.Context, cycleID, testKey string) (string, error) {
	q := url.Values{}
	q.Set("testIdOrKey", testKey)
	var resp struct {
		Executions []struct {
			Execution traceabilityExecution `json:"execution"`
		} `json:"executions"`
	}
	if err := a.z.getJSON(ctx, "/traceability/executionsByTest?"+q.Encode(), &resp); err != nil {
		return "", fmt.Errorf("find the execution of %s: %w", testKey, err)
	}
	for _, row := range resp.Executions {
		e := row.Execution
		id := jsonString(e.ID)
		rowCycle := jsonString(e.CycleID)
		if rowCycle == "" && id != "" {
			full, err := a.z.execution(ctx, id)
			if err != nil {
				continue
			}
			rowCycle = jsonString(full.CycleID)
		}
		if rowCycle == cycleID && id != "" {
			return id, nil
		}
	}
	return "", fmt.Errorf("%s has no execution in cycle %s, so it has no results there; add it to the cycle first", testKey, cycleID)
}

// CreateContainer creates a test cycle. It is the only container kind
// Essential has: there is no Test Set and no Test Plan, and creating one as a
// cycle would quietly make the wrong thing.
//
// A cycle belongs to a Jira version and the call carries none, so it is
// created in the unscheduled bucket. That is where Zephyr itself puts a cycle
// with no release, and it is the version the read path already walks, so the
// new cycle shows up in the Containers view without anything else changing.
func (a *Adapter) CreateContainer(ctx context.Context, projectKey, kind, summary string) (string, error) {
	if kind != backend.KindTestExec {
		return "", fmt.Errorf("zephyr essential has no %s to create; a cycle is its only container", kind)
	}
	name := strings.TrimSpace(summary)
	if name == "" {
		return "", fmt.Errorf("a cycle needs a name")
	}
	projectID, _, err := a.z.projectVersionIDs(ctx, projectKey)
	if err != nil {
		return "", err
	}
	var created struct {
		ID json.RawMessage `json:"id"`
	}
	body := map[string]any{"name": name, "projectId": projectID, "versionId": unscheduledVersion}
	if err := a.z.writeJSON(ctx, "POST", "/cycle", body, &created); err != nil {
		return "", fmt.Errorf("create the cycle %q: %w", name, err)
	}
	id := jsonString(created.ID)
	if id == "" {
		// The caller keys the container by this id and journals pending work
		// against it, so a create that cannot say what it made is a failure
		// rather than something to paper over with a synthesised key.
		return "", fmt.Errorf("created the cycle %q but the instance did not say what its id is", name)
	}
	return id, nil
}

// unscheduledVersion is Zephyr's bucket for work that belongs to no release.
// It is the same -1 the read path asks for.
const unscheduledVersion = "-1"

// AddTestsToContainer adds tests to a cycle, and waits for the add to land.
//
// This is the one place the bridge needs more than a mapping. Zephyr answers
// the add with a job token and the tests are not in the cycle when the call
// returns, while the commit engine takes a nil error to mean the write has
// happened. So the token is followed rather than ignored: a job that failed
// after being accepted would otherwise clear the pending change and leave the
// cycle without the tests.
func (a *Adapter) AddTestsToContainer(ctx context.Context, kind, containerKey string, testKeys []string) error {
	if kind != backend.KindTestExec {
		return fmt.Errorf("zephyr essential has no %s to add tests to", kind)
	}
	if len(testKeys) == 0 {
		return nil
	}
	projectID, versionID, err := a.z.cycleLocation(ctx, projectKeyOf(testKeys[0]), containerKey)
	if err != nil {
		return err
	}
	var started struct {
		Token string `json:"jobProgressToken"`
	}
	body := map[string]any{
		"cycleId":   containerKey,
		"projectId": projectID,
		"versionId": versionID,
		// The documented value for adding by issue key rather than by search.
		"method": "1",
		"issues": testKeys,
	}
	if err := a.z.writeJSON(ctx, "POST", "/execution/addTestsToCycle", body, &started); err != nil {
		return fmt.Errorf("add %d test(s) to cycle %s: %w", len(testKeys), containerKey, err)
	}
	if strings.TrimSpace(started.Token) == "" {
		// Whether a small add is asynchronous at all is one of the things the
		// documentation leaves open. An add that answered without a token has
		// already happened, and waiting for a job that does not exist would
		// fail a write that worked.
		return nil
	}
	return a.z.awaitJob(ctx, started.Token, containerKey)
}

// projectKeyOf is the project half of a Jira issue key. The cycle a test is
// being added to belongs to a project and a version, and neither is in the
// call, so the key is where the project comes from.
func projectKeyOf(issueKey string) string {
	if i := strings.LastIndex(issueKey, "-"); i > 0 {
		return issueKey[:i]
	}
	return issueKey
}

// cycleLocation finds the project and version a cycle belongs to.
//
// /cycle answers for one version at a time and the cycle key XTM holds is the
// cycle id alone, so the versions are walked until one of them lists it. The
// cycle's own versionId is preferred where it reports one, because a cycle can
// come back under more than one version.
func (c *Client) cycleLocation(ctx context.Context, projectKey, cycleID string) (string, string, error) {
	projectID, versionIDs, err := c.projectVersionIDs(ctx, projectKey)
	if err != nil {
		return "", "", err
	}
	for _, versionID := range versionIDs {
		q := url.Values{}
		q.Set("projectId", projectID)
		q.Set("versionId", versionID)
		var raw map[string]json.RawMessage
		if err := c.getJSON(ctx, "/cycle?"+q.Encode(), &raw); err != nil {
			return "", "", fmt.Errorf("look for cycle %s: %w", cycleID, err)
		}
		entry, ok := raw[cycleID]
		if !ok {
			continue
		}
		var cycle struct {
			VersionID json.RawMessage `json:"versionId"`
		}
		if json.Unmarshal(entry, &cycle) == nil {
			if own := jsonString(cycle.VersionID); own != "" {
				return projectID, own, nil
			}
		}
		return projectID, versionID, nil
	}
	return "", "", fmt.Errorf("no cycle %s in project %s, so there is nothing to add the tests to", cycleID, projectKey)
}

// awaitJob follows an add until the instance says it finished.
//
// A job that reports a failure is a failure: the row stays pending and the
// user is told. A job that is still running when the wait runs out is
// reported as done, with a log line, because the add was accepted and the
// worse outcome is a retry that adds every test a second time. The next sync
// shows what actually landed.
func (c *Client) awaitJob(ctx context.Context, token, cycleID string) error {
	path := "/execution/jobProgress/" + url.PathEscape(token) +
		"?type=bulk_execution_copy_move_job_progress"
	for attempt := 0; attempt < jobPollAttempts; attempt++ {
		var progress struct {
			Progress     float64 `json:"progress"`
			ErrorMessage string  `json:"errorMessage"`
		}
		if err := c.getJSON(ctx, path, &progress); err != nil {
			return fmt.Errorf("follow the add to cycle %s: %w", cycleID, err)
		}
		if msg := strings.TrimSpace(progress.ErrorMessage); msg != "" {
			return fmt.Errorf("adding the tests to cycle %s failed: %s", cycleID, msg)
		}
		if progress.Progress >= 1 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(jobPollInterval):
		}
	}
	log.Printf("xtm: zephyr: the add to cycle %s was accepted and is still running; the next sync will show what landed", cycleID)
	return nil
}

// writeJSON sends a JSON body under the Zephyr API root and decodes the
// response into out, which may be nil to discard it.
//
// It checks the body as well as the status, because an invalid authentication
// can answer 200 OK carrying an errorId: a write that trusted the status code
// would report a change as pushed that was never accepted, and the pending
// change would be cleared (I1).
func (c *Client) writeJSON(ctx context.Context, method, path string, body, out any) error {
	resp, err := c.jira.WriteJSONRaw(ctx, method, apiPath+path, body)
	if err != nil {
		return fmt.Errorf("zephyr: %s %s: %w", method, path, err)
	}
	if resp.Code >= 300 {
		return fmt.Errorf("zephyr: %s %s -> %s: %s", method, path, resp.Status,
			strings.TrimSpace(string(resp.Body)))
	}
	if err := bodyError(resp.Body); err != nil {
		return fmt.Errorf("zephyr: %s %s: %w", method, path, err)
	}
	if out == nil {
		return nil
	}
	if len(resp.Body) == 0 {
		return nil
	}
	if err := json.Unmarshal(resp.Body, out); err != nil {
		return fmt.Errorf("zephyr: decode the answer to %s %s: %w", method, path, err)
	}
	return nil
}
