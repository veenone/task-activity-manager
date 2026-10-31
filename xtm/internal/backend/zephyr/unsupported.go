// Every backend.Backend method a Zephyr Essential profile does not answer yet,
// in one place.
//
// Each group names the phase that fills it, and the compiler is the checklist:
// a method moves out of this file when its phase lands. Phase 2 emptied the
// read groups, which are now in jiraside.go, steps.go and containers.go. What
// Essential does not have at all is in absent.go instead.
//
// What is left is the write path and the custom field namespaces. The writes
// wait on #371 Phase 4, whose whole purpose is to run the six endpoints
// SmartBear documents but nobody has measured, before any adapter depends on
// them.
//
// Phases are from docs/superpowers/plans/2026-10-30-zephyr-essential-bridge.md.

package zephyr

import (
	"context"

	"agile-suite/xtm/internal/backend"
)

// --- steps: writes ---
//
// TODO(xtm) #371 Phase 5: a step create is documented against
// POST /teststep/{issueId}, and the update, delete and reorder endpoints are
// documented but unproven. Phase 4 runs them against a scratch project first,
// because a wrong guess at a step write reshapes someone's test.

func (a *Adapter) CreateTestStep(ctx context.Context, key, action, data, expected string) (string, error) {
	return "", backend.ErrUnsupported
}

func (a *Adapter) UpdateTestStep(ctx context.Context, key, stepID string, fields map[string]string) error {
	return backend.ErrUnsupported
}

func (a *Adapter) DeleteTestStep(ctx context.Context, key, stepID string) error {
	return backend.ErrUnsupported
}

func (a *Adapter) MoveTestStep(ctx context.Context, key, stepID string, index int, action, data, expected string) error {
	return backend.ErrUnsupported
}

// --- custom fields ---
//
// TODO(xtm) #371 Phase 3: two namespaces, Jira's own fields and Zephyr's
// fields on an EXECUTION or a TESTSTEP entity. That is why CustomFieldDef
// grows a scope rather than ids carrying a prefix.
//
// The five resolvers below return ok=false with no error, as Kiwi's do: they
// resolve Xray-specific fields that have no counterpart here, and the commit
// engine already reads ok=false as "skip this field".

func (a *Adapter) ListCustomFields(ctx context.Context, projectKey string) ([]backend.CustomFieldDef, error) {
	return nil, backend.ErrUnsupported
}

func (a *Adapter) GetTestCustomFields(ctx context.Context, testKey string) (map[string]string, error) {
	return nil, backend.ErrUnsupported
}

func (a *Adapter) CustomFieldValue(ctx context.Context, fieldID, value string) (string, any, error) {
	return "", nil, backend.ErrUnsupported
}

func (a *Adapter) ExecTypeFieldValue(ctx context.Context, execType string) (fieldID string, value any, ok bool, err error) {
	return "", nil, false, nil
}

func (a *Adapter) CucumberScenarioFieldValue(ctx context.Context, v string) (fieldID string, value any, ok bool, err error) {
	return "", nil, false, nil
}

func (a *Adapter) CucumberTypeFieldValue(ctx context.Context, v string) (fieldID string, value any, ok bool, err error) {
	return "", nil, false, nil
}

func (a *Adapter) GenericDefinitionFieldValue(ctx context.Context, v string) (fieldID string, value any, ok bool, err error) {
	return "", nil, false, nil
}

func (a *Adapter) ConditionFieldValue(ctx context.Context, v string) (fieldID string, value any, ok bool, err error) {
	return "", nil, false, nil
}

// --- containers and runs: writes ---
//
// TODO(xtm) #371 Phase 5. Two of these are more than a mapping. Setting a run
// status needs the execution id and this instance's numeric status id, so it
// resolves both before writing; and adding tests to a cycle is asynchronous,
// returning a job token to poll, which the synchronous commit engine cannot
// wait on. Both are why the write path is a phase of its own rather than the
// second half of this one.

func (a *Adapter) CreateContainer(ctx context.Context, projectKey, kind, summary string) (string, error) {
	return "", backend.ErrUnsupported
}

func (a *Adapter) AddTestsToContainer(ctx context.Context, kind, containerKey string, testKeys []string) error {
	return backend.ErrUnsupported
}

func (a *Adapter) RemoveTestsFromContainer(ctx context.Context, kind, containerKey string, testKeys []string) error {
	return backend.ErrUnsupported
}

func (a *Adapter) SetTestRunStatus(ctx context.Context, execKey, testKey, status string) error {
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
