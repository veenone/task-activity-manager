// Per-step results on an ordinary execution: a step result that does not hang
// off a dataset.
//
// XTM modelled step results only inside a parameterized iteration, because
// Xray only has them there. Zephyr Essential has no datasets at all and its
// step results are first-class instead: each one has its own id, its own
// status and its own comment, on any execution (#371 Phase 6, Q3 of the design
// document).
//
// This is the one thing Zephyr does better than Xray, and it is why the
// SupportsParameterizedRunStepWrites flag added by #369 could not carry it:
// that flag answers "can a step inside an iteration be written", and on
// Essential there is no iteration to ask about.

package backend

import "context"

// RunStepResult is one step's recorded result on one test's execution.
//
// The identity that matters is ID, the result's own id on the backend. The
// step it belongs to is DefinitionID, which is the step on the test and is
// NOT what a write addresses: two executions of the same test have two
// results for the same step.
type RunStepResult struct {
	ID           string `json:"id"`
	DefinitionID string `json:"definitionId"`
	Rank         int    `json:"rank"`
	Status       string `json:"status"`
	Action       string `json:"action"`
	Data         string `json:"data"`
	Expected     string `json:"expected"`
	// Comment is what the executor recorded against this step. It is read
	// only: the write that sets it is one of the endpoints #371 Phase 4 has
	// still to measure.
	Comment string `json:"comment"`
	// Uncommitted marks a result journalled locally that has not reached the
	// backend yet, exactly as it does on RunIteration (#366). A backend never
	// sets it; the store does, so a local pick does not read as a value the
	// server already holds.
	Uncommitted bool `json:"uncommitted"`
}

// RunStepResultManager is the optional capability for reading and writing step
// results on a plain execution.
//
// It is deliberately NOT part of Backend, for the reason TestPreconditionReader
// and PreconditionStreamer are not. Backend is already 87 methods, and only a
// backend whose step results are addressable can answer this at all: Xray
// exposes no step-result id, reports every step carrying its iteration's
// status, and answers a step write with an empty stepResults array, which is
// what #362, #364, #367 and #369 each cost a session to establish. Widening
// Backend would hand every other adapter two more methods it can only refuse.
//
// Callers gate on Capabilities().SupportsRunStepResults and type-assert.
type RunStepResultManager interface {
	// GetRunStepResults reads one test's step results in one execution,
	// ordered by the step's position on the test.
	GetRunStepResults(ctx context.Context, execKey, testKey string) ([]RunStepResult, error)
	// SetRunStepStatus records a result against one step result, addressed by
	// its own id rather than by the step it belongs to.
	SetRunStepStatus(ctx context.Context, execKey, testKey, stepResultID, status string) error
}
