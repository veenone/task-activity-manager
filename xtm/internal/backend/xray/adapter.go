// Package xray adapts the existing *jira.Client (Xray Server/DC + Jira DC REST)
// to the neutral backend.Backend interface. It is a thin delegating shim: each
// method forwards to the wrapped client and maps between the jira.* shapes and
// the neutral backend.* DTOs. No business logic lives here.
package xray

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/jira"
)

// Adapter implements backend.Backend by wrapping a *jira.Client.
type Adapter struct {
	c *jira.Client
}

// New wraps an existing Jira/Xray client as a backend.Backend.
func New(c *jira.Client) *Adapter { return &Adapter{c: c} }

// Compile-time assertion that the adapter satisfies the interface.
var _ backend.Backend = (*Adapter)(nil)
var _ backend.ComponentManager = (*Adapter)(nil)
var _ backend.PlanDatasetReader = (*Adapter)(nil)

// --- connection / auth ---

func (a *Adapter) TestConnection(ctx context.Context) (*backend.User, error) {
	u, err := a.c.TestConnection(ctx)
	if err != nil {
		return nil, err
	}
	return toUser(u), nil
}

func (a *Adapter) IsDemo() bool { return a.c.IsDemo() }

func (a *Adapter) SetRequirementLinkType(name string) { a.c.SetRequirementLinkType(name) }

// --- tests ---

func (a *Adapter) SearchTestsPage(ctx context.Context, projectKey, scopeJQL, since string, startAt, maxResults int) ([]backend.Test, int, error) {
	tests, total, err := a.c.SearchTestsPage(ctx, projectKey, scopeJQL, since, startAt, maxResults)
	if err != nil {
		return nil, 0, err
	}
	return toTests(tests), total, nil
}

func (a *Adapter) ListTestsBasic(ctx context.Context, keys []string) ([]backend.TestBasic, error) {
	tb, err := a.c.ListTestsBasic(ctx, keys)
	if err != nil {
		return nil, err
	}
	return toTestBasics(tb), nil
}

func (a *Adapter) SearchTestsAcrossProjects(ctx context.Context, projectKeys []string, query string, offset, limit int) ([]backend.TestBasic, int, error) {
	tb, total, err := a.c.SearchTestsAcrossProjects(ctx, projectKeys, query, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	return toTestBasics(tb), total, nil
}

func (a *Adapter) SearchPreconditionsAcrossProjects(ctx context.Context, projectKeys []string, query string, offset, limit int) ([]backend.Precondition, int, error) {
	pcs, total, err := a.c.SearchPreconditionsAcrossProjects(ctx, projectKeys, query, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	return toPreconditions(pcs), total, nil
}

func (a *Adapter) GetTestFields(ctx context.Context, key string) (backend.Test, error) {
	t, err := a.c.GetTestFields(ctx, key)
	if err != nil {
		return backend.Test{}, err
	}
	return toTest(t), nil
}

func (a *Adapter) CreateTest(ctx context.Context, projectKey, summary, description, priority string, labels, components []string, extraFields map[string]any) (string, error) {
	return a.c.CreateTest(ctx, projectKey, summary, description, priority, labels, components, extraFields)
}

func (a *Adapter) UpdateIssue(ctx context.Context, key string, fields map[string]any) error {
	return a.c.UpdateIssue(ctx, key, fields)
}

func (a *Adapter) GetTestMeta(ctx context.Context, key string) (backend.TestMeta, error) {
	m, err := a.c.GetTestMeta(ctx, key)
	if err != nil {
		return backend.TestMeta{}, err
	}
	return toTestMeta(m), nil
}

// --- concurrency ---

// RemoteVersion returns the issue's current `updated` timestamp as an opaque
// VersionToken. entityType is ignored — Xray versions every entity by its
// Jira issue `updated` field.
func (a *Adapter) RemoteVersion(ctx context.Context, entityType, externalKey string) (backend.VersionToken, error) {
	upd, err := a.c.GetIssueUpdated(ctx, externalKey)
	if err != nil {
		return "", err
	}
	return backend.VersionToken(upd), nil
}

// RemoteAhead reports whether remote's Jira `updated` timestamp is strictly
// later than base's. On parse failure it is permissive (returns false) so a
// malformed remote string can't manufacture a phantom conflict.
func (a *Adapter) RemoteAhead(base, remote backend.VersionToken) bool {
	bt, ok1 := parseJiraTime(string(base))
	rt, ok2 := parseJiraTime(string(remote))
	if !ok1 || !ok2 {
		return false
	}
	return rt.After(bt)
}

// --- steps ---

func (a *Adapter) GetTestSteps(ctx context.Context, key string) ([]backend.Step, error) {
	steps, err := a.c.GetTestSteps(ctx, key)
	if err != nil {
		return nil, err
	}
	return toSteps(steps), nil
}

func (a *Adapter) CreateTestStep(ctx context.Context, key, action, data, expected string) (string, error) {
	return a.c.CreateTestStep(ctx, key, action, data, expected)
}

func (a *Adapter) UpdateTestStep(ctx context.Context, key, stepID string, fields map[string]string) error {
	return a.c.UpdateTestStep(ctx, key, stepID, fields)
}

func (a *Adapter) DeleteTestStep(ctx context.Context, key, stepID string) error {
	return a.c.DeleteTestStep(ctx, key, stepID)
}

func (a *Adapter) MoveTestStep(ctx context.Context, key, stepID string, index int, action, data, expected string) error {
	return a.c.MoveTestStep(ctx, key, stepID, index, action, data, expected)
}

func (a *Adapter) CreateCalledTestStep(ctx context.Context, key, calledTestKey, calledTestID string) (string, error) {
	return a.c.CreateCalledTestStep(ctx, key, calledTestKey, calledTestID)
}

// --- custom fields ---

func (a *Adapter) ListCustomFields(ctx context.Context, projectKey string) ([]backend.CustomFieldDef, error) {
	defs, err := a.c.ListCustomFields(ctx, projectKey)
	if err != nil {
		return nil, err
	}
	return toCustomFieldDefs(defs), nil
}

func (a *Adapter) GetTestCustomFields(ctx context.Context, testKey string) (map[string]string, error) {
	return a.c.GetTestCustomFields(ctx, testKey)
}

func (a *Adapter) CustomFieldValue(ctx context.Context, fieldID, value string) (string, any, error) {
	return a.c.CustomFieldValue(ctx, fieldID, value)
}

func (a *Adapter) ExecTypeFieldValue(ctx context.Context, execType string) (fieldID string, value any, ok bool, err error) {
	return a.c.ExecTypeFieldValue(ctx, execType)
}

// ConditionFieldValue resolves the Xray precondition condition custom field.
func (a *Adapter) ConditionFieldValue(ctx context.Context, v string) (string, any, bool, error) {
	return a.c.ConditionFieldValue(ctx, v)
}

func (a *Adapter) CucumberScenarioFieldValue(ctx context.Context, v string) (string, any, bool, error) {
	return a.c.CucumberScenarioFieldValue(ctx, v)
}

func (a *Adapter) CucumberTypeFieldValue(ctx context.Context, v string) (string, any, bool, error) {
	return a.c.CucumberTypeFieldValue(ctx, v)
}

func (a *Adapter) GenericDefinitionFieldValue(ctx context.Context, v string) (string, any, bool, error) {
	return a.c.GenericDefinitionFieldValue(ctx, v)
}

// --- containers ---

func (a *Adapter) ListContainers(ctx context.Context, projectKey string, onProgress func(done, total int)) ([]backend.Container, []backend.ContainerLink, error) {
	cs, links, err := a.c.ListContainers(ctx, projectKey, onProgress)
	if err != nil {
		return nil, nil, err
	}
	return toContainers(cs), toContainerLinks(links), nil
}

func (a *Adapter) TestExecutionsForTest(ctx context.Context, testKey string) ([]backend.Container, []backend.ContainerLink, error) {
	cs, links, err := a.c.TestExecutionsForTest(ctx, testKey)
	if err != nil {
		return nil, nil, err
	}
	return toContainers(cs), toContainerLinks(links), nil
}

func (a *Adapter) CreateContainer(ctx context.Context, projectKey, kind, summary string) (string, error) {
	return a.c.CreateContainer(ctx, projectKey, kind, summary)
}

func (a *Adapter) AddTestsToContainer(ctx context.Context, kind, containerKey string, testKeys []string) error {
	return a.c.AddTestsToContainer(ctx, kind, containerKey, testKeys)
}

func (a *Adapter) RemoveTestsFromContainer(ctx context.Context, kind, containerKey string, testKeys []string) error {
	return a.c.RemoveTestsFromContainer(ctx, kind, containerKey, testKeys)
}

func (a *Adapter) SetTestRunStatus(ctx context.Context, execKey, testKey, status string) error {
	return a.c.SetTestRunStatus(ctx, execKey, testKey, status)
}

func (a *Adapter) AddTestRunDefect(ctx context.Context, execKey, testKey, bugKey string) error {
	return a.c.AddTestRunDefect(ctx, execKey, testKey, bugKey)
}

func (a *Adapter) RemoveTestRunDefect(ctx context.Context, execKey, testKey, bugKey string) error {
	return a.c.RemoveTestRunDefect(ctx, execKey, testKey, bugKey)
}

func (a *Adapter) SetTestRunComment(ctx context.Context, execKey, testKey, comment string) error {
	return a.c.SetTestRunComment(ctx, execKey, testKey, comment)
}

func (a *Adapter) SetContainerEnvironments(ctx context.Context, execKey string, envs []string) error {
	return a.c.SetContainerEnvironments(ctx, execKey, envs)
}

func (a *Adapter) DeleteContainer(ctx context.Context, kind, containerKey string) error {
	return a.c.DeleteContainer(ctx, kind, containerKey)
}

func (a *Adapter) GetTestRuns(ctx context.Context, execKey string) ([]backend.TestRun, error) {
	runs, err := a.c.GetTestRuns(ctx, execKey)
	if err != nil {
		return nil, err
	}
	return toTestRuns(runs), nil
}

func (a *Adapter) ExecPlans(ctx context.Context, execKey string) ([]string, error) {
	return a.c.ExecPlans(ctx, execKey)
}

// --- preconditions ---

func (a *Adapter) ListPreconditions(ctx context.Context, projectKey string, onProgress func(done, total int)) ([]backend.Precondition, map[string][]string, error) {
	pcs, membership, err := a.c.ListPreconditions(ctx, projectKey, onProgress)
	if err != nil {
		return nil, nil, err
	}
	return toPreconditions(pcs), membership, nil
}

// ListPreconditionsStream satisfies backend.PreconditionStreamer by delegating
// to the Jira client's streaming walk, translating each batch into the neutral
// backend types.
func (a *Adapter) ListPreconditionsStream(
	ctx context.Context,
	projectKey string,
	onProgress func(stage string, done, total int),
	onBatch func(pre []backend.Precondition, links map[string][]string) error,
) error {
	staged := func(stage string, done, total int) {
		if onProgress != nil {
			onProgress(preconditionStage(stage), done, total)
		}
	}
	return a.c.ListPreconditionsStream(ctx, projectKey, staged,
		func(jp []jira.Precondition, links map[string][]string) error {
			return onBatch(toPreconditions(jp), links)
		})
}

// preconditionStage translates the Jira client's stage name into the neutral
// one. Written out rather than passed through so the two packages drifting
// apart is a compile error here instead of an unlabelled bar in the UI.
func preconditionStage(s string) string {
	switch s {
	case jira.PreconditionStageFinding:
		return backend.PreconditionStageFinding
	case jira.PreconditionStageLinking:
		return backend.PreconditionStageLinking
	}
	return ""
}

// ListTestPreconditions implements backend.TestPreconditionReader. Xray exposes
// the association from the test side, so one Test's Preconditions can be read
// without walking the project.
func (a *Adapter) ListTestPreconditions(ctx context.Context, testKey string) ([]backend.Precondition, error) {
	pcs, err := a.c.ListTestPreconditions(ctx, testKey)
	if err != nil {
		return nil, err
	}
	return toPreconditions(pcs), nil
}

func (a *Adapter) CreatePrecondition(ctx context.Context, projectKey, summary, ptype, description string, extraFields map[string]any) (string, error) {
	return a.c.CreatePrecondition(ctx, projectKey, summary, ptype, description, extraFields)
}

func (a *Adapter) GetPreconditionCreateFields(ctx context.Context, projectKey string) ([]backend.FieldDef, error) {
	fields, err := a.c.GetPreconditionCreateFields(ctx, projectKey)
	if err != nil {
		return nil, err
	}
	return toFieldDefs(fields), nil
}

func (a *Adapter) GetTestCreateFields(ctx context.Context, projectKey string) ([]backend.FieldDef, error) {
	fields, err := a.c.GetTestCreateFields(ctx, projectKey)
	if err != nil {
		return nil, err
	}
	return toFieldDefs(fields), nil
}

func (a *Adapter) UpdateTestPreconditions(ctx context.Context, testKey string, add, remove []string) error {
	return a.c.UpdateTestPreconditions(ctx, testKey, add, remove)
}

func (a *Adapter) DeletePrecondition(ctx context.Context, preconditionKey string) error {
	return a.c.DeletePrecondition(ctx, preconditionKey)
}

// --- requirements ---

func (a *Adapter) ListRequirements(ctx context.Context, profileProjectKey string, sources []backend.RequirementSourceSpec, onProgress func(done, total int)) ([]backend.Requirement, []backend.RequirementLink, error) {
	reqs, links, err := a.c.ListRequirements(ctx, profileProjectKey, fromRequirementSourceSpecs(sources), onProgress)
	if err != nil {
		return nil, nil, err
	}
	return toRequirements(reqs), toRequirementLinks(links), nil
}

func (a *Adapter) UpdateTestRequirements(ctx context.Context, testKey string, add []string, removeLinkIDs []string) error {
	return a.c.UpdateTestRequirements(ctx, testKey, add, removeLinkIDs)
}

func (a *Adapter) ListIssueLinkTypes(ctx context.Context) ([]string, error) {
	return a.c.ListIssueLinkTypes(ctx)
}

func (a *Adapter) ListIssueLinkTypeDetails(ctx context.Context) ([]backend.IssueLinkType, error) {
	raw, err := a.c.ListIssueLinkTypeDetails(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]backend.IssueLinkType, len(raw))
	for i, t := range raw {
		out[i] = backend.IssueLinkType{Name: t.Name, Inward: t.Inward, Outward: t.Outward}
	}
	return out, nil
}

func (a *Adapter) CreateRequirement(ctx context.Context, projectKey, issueType, summary, description, priority, components, fixVersions string, extraFields map[string]any) (string, error) {
	return a.c.CreateRequirement(ctx, projectKey, issueType, summary, description, priority, components, fixVersions, extraFields)
}

func (a *Adapter) GetRequirementCreateFields(ctx context.Context, projectKey, issueType string) ([]backend.FieldDef, error) {
	fields, err := a.c.GetRequirementCreateFields(ctx, projectKey, issueType)
	if err != nil {
		return nil, err
	}
	return toFieldDefs(fields), nil
}

func (a *Adapter) DeleteRequirement(ctx context.Context, requirementKey string) error {
	return a.c.DeleteRequirement(ctx, requirementKey)
}

func (a *Adapter) UpdateRequirementLinks(ctx context.Context, fromKey string, add []string, removeLinkIDs []string) error {
	return a.c.UpdateRequirementLinks(ctx, fromKey, add, removeLinkIDs)
}

func (a *Adapter) ListReqToReqLinks(ctx context.Context, reqKeys []string) ([]backend.ReqToReqLink, error) {
	links, err := a.c.ListReqToReqLinks(ctx, reqKeys)
	if err != nil {
		return nil, err
	}
	return toReqToReqLinks(links), nil
}

// --- bugs ---

func (a *Adapter) ListBugs(ctx context.Context, testProjectKey string, testKeys []string, issueType string, onProgress func(done, total int)) ([]backend.Bug, []backend.BugLink, error) {
	bugs, links, err := a.c.ListBugs(ctx, testProjectKey, testKeys, issueType, onProgress)
	if err != nil {
		return nil, nil, err
	}
	return toBugs(bugs), toBugLinks(links), nil
}

func (a *Adapter) ListProjectBugs(ctx context.Context, projKey, issueType string) ([]backend.Bug, error) {
	bugs, err := a.c.ListProjectBugs(ctx, projKey, issueType)
	if err != nil {
		return nil, err
	}
	return toBugs(bugs), nil
}

// ListBugsByKeys implements backend.BugKeyReader by delegating to the Jira
// client's key lookup.
func (a *Adapter) ListBugsByKeys(ctx context.Context, keys []string) ([]backend.Bug, error) {
	bugs, err := a.c.ListBugsByKeys(ctx, keys)
	if err != nil {
		return nil, err
	}
	return toBugs(bugs), nil
}

func (a *Adapter) GetFieldDefs(ctx context.Context, projectKey, issueType string) ([]backend.FieldDef, error) {
	fields, err := a.c.GetFieldDefs(ctx, projectKey, issueType)
	if err != nil {
		return nil, err
	}
	return toFieldDefs(fields), nil
}

func (a *Adapter) CreateBug(ctx context.Context, projectKey, issueType, summary, description, priority string, labels []string, extraFields map[string]any) (string, error) {
	return a.c.CreateBug(ctx, projectKey, issueType, summary, description, priority, labels, extraFields)
}

func (a *Adapter) CreateBugLink(ctx context.Context, testKey, bugKey string) error {
	return a.c.CreateBugLink(ctx, testKey, bugKey)
}

func (a *Adapter) GetBugDetail(ctx context.Context, bugKey string) (backend.BugDetail, error) {
	d, err := a.c.GetBugDetail(ctx, bugKey)
	if err != nil {
		return backend.BugDetail{}, err
	}
	return toBugDetail(d), nil
}

// --- folders ---

func (a *Adapter) FolderTree(ctx context.Context, projectKey string) (backend.FolderTreeResult, error) {
	tree, err := a.c.FolderTree(ctx, projectKey)
	if err != nil {
		return backend.FolderTreeResult{}, err
	}
	return toFolderTreeResult(tree), nil
}

func (a *Adapter) ListFolders(ctx context.Context, projectKey string) ([]backend.Folder, error) {
	folders, err := a.c.ListFolders(ctx, projectKey)
	if err != nil {
		return nil, err
	}
	return toFolders(folders), nil
}

func (a *Adapter) ListTestsInFolder(ctx context.Context, projectKey, folderID string) ([]string, error) {
	return a.c.ListTestsInFolder(ctx, projectKey, folderID)
}

func (a *Adapter) CreateFolder(ctx context.Context, projectKey, parentPath, name string) error {
	return a.c.CreateFolder(ctx, projectKey, parentPath, name)
}

func (a *Adapter) RenameFolder(ctx context.Context, projectKey, path, newName string) error {
	return a.c.RenameFolder(ctx, projectKey, path, newName)
}

func (a *Adapter) DeleteFolder(ctx context.Context, projectKey, path string) error {
	return a.c.DeleteFolder(ctx, projectKey, path)
}

func (a *Adapter) MoveTestToFolder(ctx context.Context, projectKey, testKey, folderID string) error {
	return a.c.MoveTestToFolder(ctx, projectKey, testKey, folderID)
}

// --- workflow ---

func (a *Adapter) GetTransitions(ctx context.Context, key, currentStatus string) ([]backend.Transition, error) {
	ts, err := a.c.GetTransitions(ctx, key, currentStatus)
	if err != nil {
		return nil, err
	}
	return toTransitions(ts), nil
}

func (a *Adapter) PostTransition(ctx context.Context, key, transitionID string, fields map[string]any) error {
	return a.c.PostTransition(ctx, key, transitionID, fields)
}

// --- metadata ---

func (a *Adapter) ListStatuses(ctx context.Context, projectKey string) ([]string, error) {
	return a.c.ListStatuses(ctx, projectKey)
}

func (a *Adapter) ListPriorities(ctx context.Context, projectKey string) ([]string, error) {
	return a.c.ListPriorities(ctx, projectKey)
}

func (a *Adapter) ProjectComponents(ctx context.Context, projectKey string) ([]string, error) {
	return a.c.ProjectComponents(ctx, projectKey)
}

func (a *Adapter) ProjectVersions(ctx context.Context, projectKey string) ([]string, error) {
	return a.c.ProjectVersions(ctx, projectKey)
}

// --- comments ---

func (a *Adapter) AddComment(ctx context.Context, issueKey, body string) error {
	return a.c.AddComment(ctx, issueKey, body)
}

// --- field payload shaping ---

func (a *Adapter) FieldsForJira(updates map[string]string) map[string]any {
	return jira.FieldsForJira(updates)
}

// --- capabilities ---

// Capabilities reports the full Xray feature set. Xray (Server/DC) supports
// every capability the app models today except tags.
func (a *Adapter) Capabilities() backend.Capabilities {
	caps := backend.Capabilities{
		Name:                        "xray",
		IDStyle:                     "opaque",
		SupportsJQLScope:            true,
		StepModel:                   "objects",
		SupportsStepWrites:          true,
		SupportsTestTypes:           true,
		SupportsFolders:             true,
		SupportsFolderWrites:        true,
		SupportsPreconditionObjects: true,
		SupportsRequirementObjects:  true,
		SupportsIssueLinkTypes:      true,
		SupportsEnvironments:        true,
		RunStatuses:                 backend.DefaultRunStatuses,
		SupportsContainers:          true,
		ContainerKinds:              []string{jira.KindTestSet, jira.KindTestPlan, jira.KindTestExec},
		SupportsTestRuns:            true,
		StatusModel:                 "workflow",
		SupportsWorkflowTransitions: true,
		SupportsBugCreation:         true,
		SupportsBugLinks:            true,
		SupportsTags:                false,
		SupportsComponentAdmin:      true,
		SupportsDatasets:            true,
		SupportsDatasetWrites:       true,
	}
	// Reading a parameterized run works on any Xray that answers the 2.0 run
	// resource (#334), and an iteration's status writes back through the same
	// resource (#336). Per-step results are the exception and stay refused: the
	// read gives no step-result id, so a per-step write has nothing to address.
	caps.SupportsParameterizedRunDetail = true
	caps.SupportsParameterizedRunWrites = true
	caps.SupportsParameterizedRunStepWrites = false
	return caps
}

// GetDataset reads the test's Xray dataset (#267). Xray answers 404 below
// 5.1, which the client turns into an empty dataset.
func (a *Adapter) GetDataset(ctx context.Context, testKey string) (backend.Dataset, error) {
	ds, err := a.c.GetDataset(ctx, testKey)
	if err != nil {
		return backend.Dataset{}, err
	}
	return toBackendDataset(ds), nil
}

// GetRunDataset reads what a Test Execution defines for this test, which is
// what will actually run there (#277 W3).
func (a *Adapter) GetRunDataset(ctx context.Context, execKey, testKey string) (backend.Dataset, error) {
	ds, err := a.c.GetRunDataset(ctx, execKey, testKey)
	if err != nil {
		return backend.Dataset{}, err
	}
	return toBackendDataset(ds), nil
}

// GetPlanDataset reads only the Plan-level definition. The CSV transport does
// not carry list metadata, so the neutral DTO keeps those fields empty.
func (a *Adapter) GetPlanDataset(ctx context.Context, planKey, testKey string) (backend.Dataset, error) {
	ds, err := a.c.GetPlanDataset(ctx, planKey, testKey)
	if err != nil {
		return backend.Dataset{}, err
	}
	return toBackendDataset(ds), nil
}

// toBackendDataset maps the client's table onto the neutral one.
func toBackendDataset(ds jira.Dataset) backend.Dataset {
	params := make([]backend.DatasetParameter, len(ds.Parameters))
	for i, p := range ds.Parameters {
		params[i] = backend.DatasetParameter{
			Name: p.Name, Combinatorial: p.Combinatorial, ID: p.ID,
			Type: p.Type, Options: p.Options, ListScope: p.ListScope, ListID: p.ListID,
		}
	}
	return backend.Dataset{Parameters: params, Rows: ds.Rows}
}

// GetRunIterations reads what each row of the dataset did in this execution
// (#267 Phase 4).
func (a *Adapter) GetRunIterations(ctx context.Context, execKey, testKey string) ([]backend.RunIteration, error) {
	iters, err := a.c.GetRunIterations(ctx, execKey, testKey)
	if err != nil {
		return nil, err
	}
	out := make([]backend.RunIteration, len(iters))
	for i, it := range iters {
		params := make([]backend.IterationParameter, len(it.Parameters))
		for j, p := range it.Parameters {
			params[j] = backend.IterationParameter{Name: p.Name, Value: p.Value}
		}
		id := ""
		if it.ID != 0 {
			id = strconv.Itoa(it.ID)
		}
		out[i] = backend.RunIteration{ID: id, Rank: i, Status: it.Status, Parameters: params, Steps: []backend.IterationStep{}}
	}
	return out, nil
}

// SetDataset replaces the test's own dataset (#277 W1). The client resolves
// the key to the numeric issue id the dataset resource addresses, so nothing
// above this layer has to carry a Jira id.
func (a *Adapter) SetDataset(ctx context.Context, testKey string, d backend.Dataset) error {
	params := make([]jira.DatasetParameter, len(d.Parameters))
	for i, p := range d.Parameters {
		params[i] = jira.DatasetParameter{
			Name: p.Name, Combinatorial: p.Combinatorial, ID: p.ID,
			Type: p.Type, Options: p.Options, ListScope: p.ListScope, ListID: p.ListID,
		}
	}
	return a.c.SetDataset(ctx, testKey, jira.Dataset{Parameters: params, Rows: d.Rows})
}

func (a *Adapter) GetParameterizedRun(ctx context.Context, execKey, testKey string) (backend.ParameterizedRun, error) {
	if a.c == nil {
		return backend.ParameterizedRun{}, backend.ErrUnsupported
	}
	if !a.c.IsDemo() {
		return a.liveParameterizedRun(ctx, execKey, testKey)
	}
	input, err := a.c.ParameterizedRunInput(ctx, execKey, testKey)
	if errors.Is(err, jira.ErrParameterizedRunUnavailable) {
		return backend.ParameterizedRun{}, backend.ErrUnsupported
	}
	if err != nil {
		return backend.ParameterizedRun{}, err
	}
	iterations, err := backend.ExpandDataset(toBackendDataset(input.Dataset), 1000)
	if err != nil {
		return backend.ParameterizedRun{}, err
	}
	statuses := []string{"PASS", "FAIL", "TO\u0044O"}
	for index := range iterations {
		iteration := &iterations[index]
		iteration.ID = fmt.Sprintf("%s-%s-iteration-%03d", execKey, testKey, index+1)
		iteration.Status = statuses[index%len(statuses)]
		values := make(map[string]string, len(iteration.Parameters))
		for _, parameter := range iteration.Parameters {
			values[parameter.Name] = parameter.Value
		}
		iteration.Steps = make([]backend.IterationStep, len(input.Steps))
		for stepIndex, step := range input.Steps {
			status := iteration.Status
			if iteration.Status == "FAIL" && stepIndex < len(input.Steps)-1 {
				status = "PASS"
			}
			iteration.Steps[stepIndex] = backend.IterationStep{
				ID: fmt.Sprintf("%s-result-%03d", iteration.ID, stepIndex+1), DefinitionID: step.ID,
				Rank: stepIndex, Status: status,
				Action:   substituteDatasetValues(step.Action, values),
				Data:     substituteDatasetValues(step.Data, values),
				Expected: substituteDatasetValues(step.Expected, values),
			}
		}
	}
	return backend.ParameterizedRun{
		ID: fmt.Sprintf("%s-%s-run", execKey, testKey), ExecutionKey: execKey, TestKey: testKey,
		DatasetSourceKind: backend.DatasetSourceTest, DatasetSourceKey: testKey,
		Status: "FAIL", Iterations: iterations,
	}, nil
}

// SetIterationStatus writes one iteration's status back to Xray (#336). The
// run's own status is left alone: Xray derives it from the iterations and
// moves it itself, so writing it here would be XTM guessing at a rule it does
// not own.
func (a *Adapter) SetIterationStatus(ctx context.Context, execKey, testKey, iterationID, status string) error {
	if a.c == nil {
		return backend.ErrUnsupported
	}
	if a.c.IsDemo() {
		return nil
	}
	id, err := strconv.Atoi(iterationID)
	if err != nil || id == 0 {
		// A single-row run has no iteration id of its own, so there is nothing
		// to address a write to. #334 reports that case with an empty id.
		return fmt.Errorf("iteration %q has no id to write to", iterationID)
	}
	runID, missing, err := a.c.RunID(ctx, execKey, testKey)
	if err != nil {
		return err
	}
	if missing {
		return fmt.Errorf("%s has no run in %s to write to", testKey, execKey)
	}
	return a.c.SetIterationStatus(ctx, runID, id, status)
}

// SetIterationStepStatus stays unsupported on live (#336). The per-iteration
// read answers with steps that carry no id of their own, and the write
// response's stepResults array comes back empty, so there is no step-result
// id anywhere to address this to. Demo keeps its own ids and so keeps working.
func (a *Adapter) SetIterationStepStatus(ctx context.Context, execKey, testKey, iterationID, stepResultID, status string) error {
	if a.c != nil && a.c.IsDemo() {
		return nil
	}
	return backend.ErrUnsupported
}

func substituteDatasetValues(value string, parameters map[string]string) string {
	for name, replacement := range parameters {
		value = strings.ReplaceAll(value, "${"+name+"}", replacement)
	}
	return value
}

func toBackendComponent(c jira.Component) backend.Component {
	return backend.Component{ID: c.ID, Name: c.Name, Description: c.Description,
		LeadName: c.LeadName, LeadDisplayName: c.LeadDisplayName, AssigneeType: c.AssigneeType}
}

// ProjectComponentDetails lists the project's components with their ids.
func (a *Adapter) ProjectComponentDetails(ctx context.Context, projectKey string) ([]backend.Component, error) {
	list, err := a.c.ProjectComponentDetails(ctx, projectKey)
	if err != nil {
		return nil, err
	}
	out := make([]backend.Component, 0, len(list))
	for _, c := range list {
		out = append(out, toBackendComponent(c))
	}
	return out, nil
}

// CreateComponent creates a component in projectKey.
func (a *Adapter) CreateComponent(ctx context.Context, projectKey string, in backend.ComponentInput) (backend.Component, error) {
	c, err := a.c.CreateComponent(ctx, jira.ComponentInput{Project: projectKey, Name: in.Name,
		Description: in.Description, LeadUserName: in.LeadUserName, AssigneeType: in.AssigneeType})
	return toBackendComponent(c), err
}

// UpdateComponent edits a component.
func (a *Adapter) UpdateComponent(ctx context.Context, id string, in backend.ComponentInput) (backend.Component, error) {
	c, err := a.c.UpdateComponent(ctx, id, jira.ComponentInput{Name: in.Name,
		Description: in.Description, LeadUserName: in.LeadUserName, AssigneeType: in.AssigneeType})
	return toBackendComponent(c), err
}

// DeleteComponent deletes a component, optionally moving its issues first.
func (a *Adapter) DeleteComponent(ctx context.Context, id, moveIssuesTo string) error {
	return a.c.DeleteComponent(ctx, id, moveIssuesTo)
}

// ComponentIssueCount is how many issues carry a component.
func (a *Adapter) ComponentIssueCount(ctx context.Context, id string) (int, error) {
	return a.c.ComponentIssueCount(ctx, id)
}

// SearchUsers finds Jira users for the component lead picker.
func (a *Adapter) SearchUsers(ctx context.Context, query string) ([]backend.User, error) {
	users, err := a.c.SearchUsers(ctx, query)
	if err != nil {
		return nil, err
	}
	out := make([]backend.User, 0, len(users))
	for _, u := range users {
		out = append(out, backend.User{Name: u.Name, DisplayName: u.DisplayName, Email: u.Email})
	}
	return out, nil
}
