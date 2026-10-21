// Iterations of a parameterized run (#267 Phase 4).
//
// A test with a dataset runs once per row, and Xray records each of those
// runs separately, with its own status and the values it ran with. The run's
// own status is derived from them.

package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// RunIteration is one run of a parameterized test: the values it runs with
// and how it went.
type RunIteration struct {
	// ID is the iteration's own id, and what a status write is addressed to.
	// Zero where Xray reported no iteration of its own, which is what it does
	// for a dataset of a single row.
	ID         int                 `json:"id"`
	Status     string              `json:"status"`
	Parameters []RunIterationParam `json:"parameters"`
}

// RunIterationParam is one parameter value an iteration runs with.
type RunIterationParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// testRunV2Path reads a run on the 2.0 resource, the only one that carries
// iterations. The 1.0 resource returns the same run without them.
const testRunV2Path = "/rest/raven/2.0/api/testrun/"

// GetRunIterations returns the iterations of a test's run inside an
// execution, in the order they run, and nothing for a run that is not
// parameterized (#267 Phase 4).
//
// Three shapes come back from one call, and the differences are Xray's:
//
//   - More than one dataset row: an `iterations` array, each entry with its
//     own id, status and values.
//   - Exactly one row: no iterations at all, and a flat `parameters` list
//     instead. That is still a parameterized run and is reported here as one
//     iteration carrying the run's status, or the commonest parameterized run
//     would read as having none. It has no id, because Xray gave it none.
//   - No dataset: neither field, and no iterations.
//
// The iterations belong to the run rather than to the dataset: a run keeps
// the parameters it was created with, so a dataset edited afterwards does not
// change a run that already exists. Nothing here reads the cached dataset for
// that reason.
//
// A test with no run in the execution has no iterations and says so, without
// the retry resolveTestRunID uses where a run is expected to appear.
func (c *Client) GetRunIterations(ctx context.Context, execKey, testKey string) ([]RunIteration, error) {
	_, iterations, err := c.GetRunWithIterations(ctx, execKey, testKey)
	return iterations, err
}

// GetRunWithIterations is GetRunIterations plus the run id the iterations
// belong to, for a caller that then reads each iteration's steps: the steps
// resource is addressed by run id, and resolving it twice would spend a
// second lookup on an answer this call already had (#334).
//
// The id is empty where there is no run, which is also when there are no
// iterations.
func (c *Client) GetRunWithIterations(ctx context.Context, execKey, testKey string) (string, []RunIteration, error) {
	if isDemoURL(c.baseURL) {
		return "", nil, nil
	}
	runID, missing, err := c.testRunID(ctx, testRunPath(execKey, testKey))
	if err != nil {
		return "", nil, fmt.Errorf("look up the run of %s in %s: %w", testKey, execKey, err)
	}
	if missing {
		return "", nil, nil
	}

	body, err := c.getBytes(ctx, testRunV2Path+url.PathEscape(runID))
	if err != nil {
		return "", nil, fmt.Errorf("read the run of %s in %s: %w", testKey, execKey, err)
	}
	var run struct {
		Status     string              `json:"status"`
		Parameters []RunIterationParam `json:"parameters"`
		Iterations []RunIteration      `json:"iterations"`
	}
	if err := json.Unmarshal(body, &run); err != nil {
		return "", nil, fmt.Errorf("decode the run of %s in %s: %w", testKey, execKey, err)
	}

	if len(run.Iterations) > 0 {
		return runID, run.Iterations, nil
	}
	if len(run.Parameters) > 0 {
		return runID, []RunIteration{{Status: run.Status, Parameters: run.Parameters}}, nil
	}
	return runID, nil, nil
}
