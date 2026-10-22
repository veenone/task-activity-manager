// Writing an iteration's result (#336).
//
// The transport was established by #267 Phase 4 against a live instance and
// is recorded in agents/project/domain-context.md: the status goes in the
// body of the iteration resource, and Xray answers with the id alone.

package jira

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// SetIterationStatus writes one iteration's status.
//
// Xray answers `{"id":<id>,"stepResults":[]}`, the id of the iteration written
// and nothing else, so there is no new state to read back: a caller that wants
// the result re-reads the run. The run's own status moves on its own, because
// Xray derives it from the iterations, which is why nothing here writes it.
//
// The status is sent in the body. `.../iteration/{id}/status?status=PASS` is
// not a route, despite the run resource having one of that shape.
func (c *Client) SetIterationStatus(ctx context.Context, runID string, iterationID int, status string) error {
	if isDemoURL(c.baseURL) {
		return nil
	}
	path := testRunV2Path + url.PathEscape(runID) + "/iteration/" + strconv.Itoa(iterationID)
	if err := c.put(ctx, path, map[string]any{"status": status}); err != nil {
		return fmt.Errorf("write iteration %d of run %s as %s: %w", iterationID, runID, status, err)
	}
	return nil
}

// RunID resolves the id of a test's run inside an execution, for a caller that
// addresses the run's own resources without reading it first. missing is true
// where the execution has no run for that test, which is not an error.
func (c *Client) RunID(ctx context.Context, execKey, testKey string) (id string, missing bool, err error) {
	if isDemoURL(c.baseURL) {
		return "", true, nil
	}
	runID, missing, err := c.testRunID(ctx, testRunPath(execKey, testKey))
	if err != nil {
		return "", false, fmt.Errorf("look up the run of %s in %s: %w", testKey, execKey, err)
	}
	return runID, missing, nil
}
