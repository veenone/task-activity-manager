// The four features Zephyr Essential does not have, confirmed absent by a 404
// rather than assumed: the test case repository, test plans, datasets with
// their parameters, and preconditions. None of this will ever be implemented
// against Essential, which is why it is not in unsupported.go beside the
// methods that are only waiting for their phase.
//
// Their capability flags are off in adapter.go, so nothing should reach these
// methods at all. They answer backend.ErrUnsupported rather than an empty
// result because a caller that ignored the flag deserves to be told, not shown
// an empty list that reads as "this project has none".
//
// Analysis:
// docs/superpowers/specs/2026-10-30-zephyr-essential-bridge-design.md

package zephyr

import (
	"context"

	"agile-suite/xtm/internal/backend"
)

// --- datasets and parameterized runs: not in Essential ---
//
// /dataset and /testparameter both answer 404, so there is no dataset to read
// and no iteration to report. That is not a step-result limitation: Zephyr's
// step results are first-class, with their own ids, comments and defect links,
// which is the one thing it does better than Xray. They simply do not hang off
// a dataset, and Phase 6 is where they become a surface of their own instead
// of a parameterized-run flag.

func (a *Adapter) GetDataset(ctx context.Context, testKey string) (backend.Dataset, error) {
	return backend.Dataset{}, backend.ErrUnsupported
}

func (a *Adapter) GetRunDataset(ctx context.Context, execKey, testKey string) (backend.Dataset, error) {
	return backend.Dataset{}, backend.ErrUnsupported
}

func (a *Adapter) GetRunIterations(ctx context.Context, execKey, testKey string) ([]backend.RunIteration, error) {
	return nil, backend.ErrUnsupported
}

func (a *Adapter) SetDataset(ctx context.Context, testKey string, d backend.Dataset) error {
	return backend.ErrUnsupported
}

func (a *Adapter) GetParameterizedRun(ctx context.Context, execKey, testKey string) (backend.ParameterizedRun, error) {
	return backend.ParameterizedRun{}, backend.ErrUnsupported
}

func (a *Adapter) SetIterationStatus(ctx context.Context, execKey, testKey, iterationID, status string) error {
	return backend.ErrUnsupported
}

func (a *Adapter) SetIterationStepStatus(ctx context.Context, execKey, testKey, iterationID, stepResultID, status string) error {
	return backend.ErrUnsupported
}

// --- test plans: not in Essential ---
//
// /testplan answers 404. A cycle is the only container kind here, so an
// execution belongs to no plan.

func (a *Adapter) ExecPlans(ctx context.Context, execKey string) ([]string, error) {
	return nil, backend.ErrUnsupported
}

// --- preconditions: not in Essential ---

func (a *Adapter) ListPreconditions(ctx context.Context, projectKey string, onProgress func(done, total int)) ([]backend.Precondition, map[string][]string, error) {
	return nil, nil, backend.ErrUnsupported
}

func (a *Adapter) CreatePrecondition(ctx context.Context, projectKey, summary, ptype, description string, extraFields map[string]any) (string, error) {
	return "", backend.ErrUnsupported
}

func (a *Adapter) GetPreconditionCreateFields(ctx context.Context, projectKey string) ([]backend.FieldDef, error) {
	return nil, backend.ErrUnsupported
}

func (a *Adapter) UpdateTestPreconditions(ctx context.Context, testKey string, add, remove []string) error {
	return backend.ErrUnsupported
}

func (a *Adapter) DeletePrecondition(ctx context.Context, preconditionKey string) error {
	return backend.ErrUnsupported
}

func (a *Adapter) SearchPreconditionsAcrossProjects(ctx context.Context, projectKeys []string, query string, offset, limit int) ([]backend.Precondition, int, error) {
	return nil, 0, backend.ErrUnsupported
}

// --- test calls: not in Essential ---
//
// "Call to test to reuse test steps" is an Xray feature and is absent from
// Essential, so a step here always holds manual content. steps.go reads one
// accordingly.

func (a *Adapter) CreateCalledTestStep(ctx context.Context, key, calledTestKey, calledTestID string) (string, error) {
	return "", backend.ErrUnsupported
}

// --- folders: not in Essential ---
//
// /testrepository answers 404: there is no test case tree. Zephyr does have
// folders, but they group executions inside a cycle, so they are not what this
// group means and SupportsFolders stays off.

func (a *Adapter) FolderTree(ctx context.Context, projectKey string) (backend.FolderTreeResult, error) {
	return backend.FolderTreeResult{}, backend.ErrUnsupported
}

func (a *Adapter) ListFolders(ctx context.Context, projectKey string) ([]backend.Folder, error) {
	return nil, backend.ErrUnsupported
}

func (a *Adapter) ListTestsInFolder(ctx context.Context, projectKey, folderID string) ([]string, error) {
	return nil, backend.ErrUnsupported
}

func (a *Adapter) CreateFolder(ctx context.Context, projectKey, parentPath, name string) error {
	return backend.ErrUnsupported
}

func (a *Adapter) RenameFolder(ctx context.Context, projectKey, path, newName string) error {
	return backend.ErrUnsupported
}

func (a *Adapter) DeleteFolder(ctx context.Context, projectKey, path string) error {
	return backend.ErrUnsupported
}

func (a *Adapter) MoveTestToFolder(ctx context.Context, projectKey, testKey, folderID string) error {
	return backend.ErrUnsupported
}

// --- the Xray-specific field resolvers: nothing here to resolve ---
//
// Each of these resolves a value to the instance-specific Jira custom field
// Xray keeps it in. Essential has no test type, no Gherkin or generic test
// body and no preconditions, so there is no field to resolve to and never
// will be.
//
// They answer ok=false with no error rather than ErrUnsupported, as Kiwi's do:
// the commit engine reads ok=false as "this backend has no such field, skip
// it" and carries on with the rest of the change, which is right here. An
// error would fail a commit over a field the user never set.

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
