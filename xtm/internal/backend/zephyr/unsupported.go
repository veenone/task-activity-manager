// Every backend.Backend method a Zephyr Essential profile does not answer yet,
// in one place.
//
// Each group names the phase that fills it, and the compiler is the checklist:
// a method moves out of this file when its phase lands. Phase 2 emptied the
// read groups, which are now in jiraside.go, steps.go and containers.go. What
// Essential does not have at all is in absent.go instead.
//
// What is left is the write path. It waits on #371 Phase 4, whose whole
// purpose is to run the six endpoints SmartBear documents but nobody has
// measured, before any adapter depends on them.
//
// Phases are from docs/superpowers/plans/2026-10-30-zephyr-essential-bridge.md.

package zephyr

import (
	"context"

	"agile-suite/xtm/internal/backend"
)

// --- steps: the writes that reshape an existing step ---
//
// TODO(xtm) #371 Phase 4: the update, delete and reorder endpoints are
// documented and unproven, and Phase 4 runs them against a scratch project
// first, because a wrong guess at any of them reshapes a step someone wrote.
//
// The create moved to steps.go (#378): appending a step can be inspected and
// removed, and a publish cannot write a test's steps without it.

func (a *Adapter) UpdateTestStep(ctx context.Context, key, stepID string, fields map[string]string) error {
	return backend.ErrUnsupported
}

func (a *Adapter) DeleteTestStep(ctx context.Context, key, stepID string) error {
	return backend.ErrUnsupported
}

func (a *Adapter) MoveTestStep(ctx context.Context, key, stepID string, index int, action, data, expected string) error {
	return backend.ErrUnsupported
}

// --- containers and runs: the writes that are still unproven ---
//
// TODO(xtm) #371 Phase 5 landed the three whose request shapes are documented:
// a run result, a cycle, and the tests in one, all in write.go. What is left
// here is every write SmartBear documents without a measured request or
// response, and all but one of them is a delete.
//
// They wait on Phase 4, which needs a scratch project. A wrong guess at these
// removes someone's executions or their cycle, and unlike a wrong guess at a
// create there is nothing to inspect afterwards.

func (a *Adapter) RemoveTestsFromContainer(ctx context.Context, kind, containerKey string, testKeys []string) error {
	return backend.ErrUnsupported
}

func (a *Adapter) AddTestRunDefect(ctx context.Context, execKey, testKey, bugKey string) error {
	return backend.ErrUnsupported
}

func (a *Adapter) RemoveTestRunDefect(ctx context.Context, execKey, testKey, bugKey string) error {
	return backend.ErrUnsupported
}

func (a *Adapter) SetTestRunComment(ctx context.Context, execKey, testKey, comment string) error {
	return backend.ErrUnsupported
}

// SetContainerEnvironments has no counterpart to write to: a cycle carries one
// environment name rather than a set, and SupportsEnvironments is off because
// of it. containers.go still reads the name, so a cycle's environment is
// visible even though it cannot be edited here.
func (a *Adapter) SetContainerEnvironments(ctx context.Context, execKey string, envs []string) error {
	return backend.ErrUnsupported
}

func (a *Adapter) DeleteContainer(ctx context.Context, kind, containerKey string) error {
	return backend.ErrUnsupported
}
