// A parameterized run read from live Xray (#334).
//
// Demo mode builds a run from the test's dataset and invents the statuses.
// Live Xray already holds both, so nothing here generates anything: the
// iterations, their statuses and their step text all come off the wire.

package xray

import (
	"context"
	"strconv"

	"agile-suite/xtm/internal/backend"
)

// liveParameterizedRun assembles the run the member table expands.
//
// It costs one call per iteration on top of the two the run itself needs,
// because the 2.0 run resource lists the iterations without their steps and
// there is no collection resource to read them together. The member table asks
// for every row as it mounts, so opening an execution of parameterized tests
// pays that burst once: app_parameterized_run.go stores what comes back and
// serves later reads from the store unless the caller forces a refresh.
//
// A run with no iterations is not an error here but an ordinary unparameterized
// test, and it is reported as unsupported so the row falls back to a plain run
// with its status select intact, which is what the member table does with
// ErrUnsupported.
func (a *Adapter) liveParameterizedRun(ctx context.Context, execKey, testKey string) (backend.ParameterizedRun, error) {
	runID, iterations, err := a.c.GetRunWithIterations(ctx, execKey, testKey)
	if err != nil {
		return backend.ParameterizedRun{}, err
	}
	if len(iterations) == 0 {
		return backend.ParameterizedRun{}, backend.ErrUnsupported
	}

	out := make([]backend.RunIteration, len(iterations))
	for i, it := range iterations {
		params := make([]backend.IterationParameter, len(it.Parameters))
		for j, p := range it.Parameters {
			params[j] = backend.IterationParameter{Name: p.Name, Value: p.Value}
		}
		out[i] = backend.RunIteration{
			// Xray's own iteration id, as a string because that is what the
			// backend model and the status write both take. A single-row run
			// has no id of its own, and an empty one is how the view knows a
			// per-iteration write has nothing to address.
			ID:         iterationID(it.ID),
			Rank:       i,
			Status:     it.Status,
			Parameters: params,
		}
		if it.ID == 0 {
			// No id, no per-iteration resource to read steps from.
			continue
		}
		steps, err := a.c.GetIterationSteps(ctx, runID, it.ID)
		if err != nil {
			return backend.ParameterizedRun{}, err
		}
		out[i].Steps = make([]backend.IterationStep, len(steps))
		for j, s := range steps {
			out[i].Steps[j] = backend.IterationStep{
				// Xray reports neither a step-result id nor a per-step status
				// on this resource, so the id stays empty and the step shows
				// the iteration's status. Inventing either would put a value
				// on screen that nothing in Xray backs.
				Rank:     j,
				Status:   it.Status,
				Action:   s.Action,
				Data:     s.Data,
				Expected: s.Expected,
			}
		}
	}

	return backend.ParameterizedRun{
		ID:                runID,
		ExecutionKey:      execKey,
		TestKey:           testKey,
		DatasetSourceKind: backend.DatasetSourceTest,
		DatasetSourceKey:  testKey,
		Status:            runStatus(out),
		Iterations:        out,
	}, nil
}

func iterationID(id int) string {
	if id == 0 {
		return ""
	}
	return strconv.Itoa(id)
}

// runStatus is what Xray derives the run's own status from: its iterations.
// Anything not yet run leaves the run unfinished, and any failure fails it,
// which is the order Xray reports on the run resource itself.
func runStatus(iterations []backend.RunIteration) string {
	status := "PASS"
	for _, it := range iterations {
		switch it.Status {
		case "FAIL":
			return "FAIL"
		case "PASS":
		default:
			status = it.Status
		}
	}
	return status
}
