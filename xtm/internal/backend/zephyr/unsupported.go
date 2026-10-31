// Every backend.Backend method a Zephyr Essential profile does not answer yet,
// in one place, so the four Phase 1 methods in adapter.go stay readable.
//
// Each group names the phase that fills it, and the compiler is the checklist:
// a method moves out of this file when its phase lands. What Essential does
// not have at all is in absent.go instead.
//
// Phases are from docs/superpowers/plans/2026-10-30-zephyr-essential-bridge.md.

package zephyr

import (
	"context"

	"agile-suite/xtm/internal/backend"
)

// --- tests ---
//
// TODO(xtm) #371 Phase 2: a Zephyr test is a Jira issue of the Test type, so
// these are the Jira searches and reads XTM already makes. The Test issue type
// id comes from the client's own probe, not from a name.

func (a *Adapter) SearchTestsPage(ctx context.Context, projectKey, scopeJQL, since string, startAt, maxResults int) ([]backend.Test, int, error) {
	return nil, 0, backend.ErrUnsupported
}

func (a *Adapter) ListTestsBasic(ctx context.Context, keys []string) ([]backend.TestBasic, error) {
	return nil, backend.ErrUnsupported
}

func (a *Adapter) GetTestFields(ctx context.Context, key string) (backend.Test, error) {
	return backend.Test{}, backend.ErrUnsupported
}

func (a *Adapter) SearchTestsAcrossProjects(ctx context.Context, projectKeys []string, query string, offset, limit int) ([]backend.TestBasic, int, error) {
	return nil, 0, backend.ErrUnsupported
}

func (a *Adapter) SearchPreconditionsAcrossProjects(ctx context.Context, projectKeys []string, query string, offset, limit int) ([]backend.Precondition, int, error) {
	return nil, 0, backend.ErrUnsupported
}

func (a *Adapter) CreateTest(ctx context.Context, projectKey, summary, description, priority string, labels, components []string, extraFields map[string]any) (string, error) {
	return "", backend.ErrUnsupported
}

func (a *Adapter) UpdateIssue(ctx context.Context, key string, fields map[string]any) error {
	return backend.ErrUnsupported
}

func (a *Adapter) GetTestMeta(ctx context.Context, key string) (backend.TestMeta, error) {
	return backend.TestMeta{}, backend.ErrUnsupported
}

// --- concurrency ---
//
// TODO(xtm) #371 Phase 2: the Jira issue's updated field, as Xray uses.

func (a *Adapter) RemoteVersion(ctx context.Context, entityType, externalKey string) (backend.VersionToken, error) {
	return "", backend.ErrUnsupported
}

func (a *Adapter) RemoteAhead(base, remote backend.VersionToken) bool {
	// Any change counts as remote moving ahead, which is Kiwi's rule rather than
	// Xray's timestamp comparison. It errs towards reporting a conflict that is
	// not one, never towards committing over someone else's write (I2). Phase 2
	// replaces it with the Jira updated comparison, once RemoteVersion returns
	// one.
	if base == "" || remote == "" {
		return false
	}
	return base != remote
}

// --- steps ---
//
// TODO(xtm) #371 Phase 2 reads them, from the stepBeanCollection envelope and
// by the issue id rather than the key. The writes wait for Phase 5, which
// waits for Phase 4 to prove the three endpoints SmartBear documents but
// nobody has run.

func (a *Adapter) GetTestSteps(ctx context.Context, key string) ([]backend.Step, error) {
	return nil, backend.ErrUnsupported
}

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

func (a *Adapter) CreateCalledTestStep(ctx context.Context, key, calledTestKey, calledTestID string) (string, error) {
	return "", backend.ErrUnsupported
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

// --- containers and runs ---
//
// TODO(xtm) #371 Phase 2 reads cycles and executions. A cycle is the only
// container kind Essential has, and /cycle answers an object keyed by cycle id
// rather than an array. Writing a result is Phase 5, and adding tests to a
// cycle is asynchronous there, so it follows the deferred-row pattern in the
// syncer rather than blocking a commit.

func (a *Adapter) ListContainers(ctx context.Context, projectKey string, onProgress func(done, total int)) ([]backend.Container, []backend.ContainerLink, error) {
	return nil, nil, backend.ErrUnsupported
}

func (a *Adapter) TestExecutionsForTest(ctx context.Context, testKey string) ([]backend.Container, []backend.ContainerLink, error) {
	return nil, nil, backend.ErrUnsupported
}

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

func (a *Adapter) SetContainerEnvironments(ctx context.Context, execKey string, envs []string) error {
	return backend.ErrUnsupported
}

func (a *Adapter) DeleteContainer(ctx context.Context, kind, containerKey string) error {
	return backend.ErrUnsupported
}

func (a *Adapter) GetTestRuns(ctx context.Context, execKey string) ([]backend.TestRun, error) {
	return nil, backend.ErrUnsupported
}

// --- requirements ---
//
// TODO(xtm) #371 Phase 2: requirements are Jira issues and the links are Jira
// issue links, so all of this is the Jira path XTM already has.

func (a *Adapter) ListRequirements(ctx context.Context, profileProjectKey string, sources []backend.RequirementSourceSpec, onProgress func(done, total int)) ([]backend.Requirement, []backend.RequirementLink, error) {
	return nil, nil, backend.ErrUnsupported
}

func (a *Adapter) UpdateTestRequirements(ctx context.Context, testKey string, add []string, removeLinkIDs []string) error {
	return backend.ErrUnsupported
}

func (a *Adapter) ListIssueLinkTypes(ctx context.Context) ([]string, error) {
	return nil, backend.ErrUnsupported
}

func (a *Adapter) ListIssueLinkTypeDetails(ctx context.Context) ([]backend.IssueLinkType, error) {
	return nil, backend.ErrUnsupported
}

func (a *Adapter) CreateRequirement(ctx context.Context, projectKey, issueType, summary, description, priority, components, fixVersions string, extraFields map[string]any) (string, error) {
	return "", backend.ErrUnsupported
}

func (a *Adapter) GetRequirementCreateFields(ctx context.Context, projectKey, issueType string) ([]backend.FieldDef, error) {
	return nil, backend.ErrUnsupported
}

func (a *Adapter) GetTestCreateFields(ctx context.Context, projectKey string) ([]backend.FieldDef, error) {
	return nil, backend.ErrUnsupported
}

func (a *Adapter) DeleteRequirement(ctx context.Context, requirementKey string) error {
	return backend.ErrUnsupported
}

func (a *Adapter) UpdateRequirementLinks(ctx context.Context, fromKey string, add []string, removeLinkIDs []string) error {
	return backend.ErrUnsupported
}

func (a *Adapter) ListReqToReqLinks(ctx context.Context, reqKeys []string) ([]backend.ReqToReqLink, error) {
	return nil, backend.ErrUnsupported
}

// --- bugs ---
//
// TODO(xtm) #371 Phase 2: a defect is a Jira issue, filed and linked the Jira
// way. Only a defect on a step result is Zephyr's own, and that is Phase 6.

func (a *Adapter) ListBugs(ctx context.Context, testProjectKey string, testKeys []string, issueType string, onProgress func(done, total int)) ([]backend.Bug, []backend.BugLink, error) {
	return nil, nil, backend.ErrUnsupported
}

func (a *Adapter) ListProjectBugs(ctx context.Context, projKey, issueType string) ([]backend.Bug, error) {
	return nil, backend.ErrUnsupported
}

func (a *Adapter) GetFieldDefs(ctx context.Context, projectKey, issueType string) ([]backend.FieldDef, error) {
	return nil, backend.ErrUnsupported
}

func (a *Adapter) CreateBug(ctx context.Context, projectKey, issueType, summary, description, priority string, labels []string, extraFields map[string]any) (string, error) {
	return "", backend.ErrUnsupported
}

func (a *Adapter) CreateBugLink(ctx context.Context, testKey, bugKey string) error {
	return backend.ErrUnsupported
}

func (a *Adapter) GetBugDetail(ctx context.Context, bugKey string) (backend.BugDetail, error) {
	return backend.BugDetail{}, backend.ErrUnsupported
}

// --- workflow and metadata ---
//
// TODO(xtm) #371 Phase 2: Jira's transitions, statuses, priorities, components
// and versions, unchanged. The statuses here are the Jira workflow's, which is
// a different vocabulary from the execution statuses in statuses.go.

func (a *Adapter) GetTransitions(ctx context.Context, key, currentStatus string) ([]backend.Transition, error) {
	return nil, backend.ErrUnsupported
}

func (a *Adapter) PostTransition(ctx context.Context, key, transitionID string, fields map[string]any) error {
	return backend.ErrUnsupported
}

func (a *Adapter) ListStatuses(ctx context.Context, projectKey string) ([]string, error) {
	return nil, backend.ErrUnsupported
}

func (a *Adapter) ListPriorities(ctx context.Context, projectKey string) ([]string, error) {
	return nil, backend.ErrUnsupported
}

func (a *Adapter) ProjectComponents(ctx context.Context, projectKey string) ([]string, error) {
	return nil, backend.ErrUnsupported
}

func (a *Adapter) ProjectVersions(ctx context.Context, projectKey string) ([]string, error) {
	return nil, backend.ErrUnsupported
}

// --- comments ---
//
// TODO(xtm) #371 Phase 2: a Jira comment on the test issue.

func (a *Adapter) AddComment(ctx context.Context, issueKey, body string) error {
	return backend.ErrUnsupported
}

// --- field payload shaping ---
//
// TODO(xtm) #371 Phase 2: the Jira issue update payload, as Xray builds it.

func (a *Adapter) FieldsForJira(updates map[string]string) map[string]any {
	return nil
}
