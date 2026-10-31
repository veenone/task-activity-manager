// The half of backend.Backend that is a plain Jira call on a Zephyr profile.
//
// A Zephyr test is an ordinary Jira issue, so its fields, its workflow, its
// comments, the requirements it covers and the defects raised against it are
// the Jira calls XTM already makes. internal/backend/xray is where those calls
// are already adapted to the neutral DTOs, conversions included, so this file
// delegates to it rather than restating several hundred lines of mapping.
//
// Delegation is explicit, one method at a time, and never by embedding. The
// Xray adapter also answers Xray's own endpoints under /rest/raven, which a
// Zephyr instance does not serve, and an embedded adapter would satisfy the
// interface with those too: silently, and with the compiler no longer able to
// show which methods this backend has actually decided about.
// TestTheDelegatedMethodsOnlyTalkToJira pins the boundary, so a delegated
// method that grew an Xray endpoint fails there rather than for a user.

package zephyr

import (
	"context"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/backend/xray"
)

// jiraSide is the Jira half of this backend.
//
// It is built per call rather than held, because an xray.Adapter is a pointer
// to a Jira client and nothing else: there is no state to keep, no second
// client to configure, and no field that could be left nil by a caller that
// builds an Adapter directly.
func (a *Adapter) jiraSide() *xray.Adapter { return xray.New(a.j) }

// Compile-time assertion for the one optional interface a Jira-backed adapter
// can answer cheaply: fetching bugs by key is a single JQL search.
var _ backend.BugKeyReader = (*Adapter)(nil)

// --- tests ---

func (a *Adapter) SearchTestsPage(ctx context.Context, projectKey, scopeJQL, since string, startAt, maxResults int) ([]backend.Test, int, error) {
	return a.jiraSide().SearchTestsPage(ctx, projectKey, scopeJQL, since, startAt, maxResults)
}

func (a *Adapter) ListTestsBasic(ctx context.Context, keys []string) ([]backend.TestBasic, error) {
	return a.jiraSide().ListTestsBasic(ctx, keys)
}

func (a *Adapter) GetTestFields(ctx context.Context, key string) (backend.Test, error) {
	return a.jiraSide().GetTestFields(ctx, key)
}

func (a *Adapter) SearchTestsAcrossProjects(ctx context.Context, projectKeys []string, query string, offset, limit int) ([]backend.TestBasic, int, error) {
	return a.jiraSide().SearchTestsAcrossProjects(ctx, projectKeys, query, offset, limit)
}

func (a *Adapter) GetTestMeta(ctx context.Context, key string) (backend.TestMeta, error) {
	return a.jiraSide().GetTestMeta(ctx, key)
}

func (a *Adapter) CreateTest(ctx context.Context, projectKey, summary, description, priority string, labels, components []string, extraFields map[string]any) (string, error) {
	return a.jiraSide().CreateTest(ctx, projectKey, summary, description, priority, labels, components, extraFields)
}

func (a *Adapter) UpdateIssue(ctx context.Context, key string, fields map[string]any) error {
	return a.jiraSide().UpdateIssue(ctx, key, fields)
}

func (a *Adapter) GetTestCreateFields(ctx context.Context, projectKey string) ([]backend.FieldDef, error) {
	return a.jiraSide().GetTestCreateFields(ctx, projectKey)
}

// --- concurrency ---
//
// A Zephyr test is a Jira issue, so the pending-change journal, the conflict
// pre-check and the rebase machinery work here unchanged.
//
// They cover what is keyed by an issue. A cycle, an execution and a step
// result are not issues and carry no updated field, so they have no version
// token at all; #371 Phase 5 handles those by comparing values, the way #369
// does for Xray's iteration results.

func (a *Adapter) RemoteVersion(ctx context.Context, entityType, externalKey string) (backend.VersionToken, error) {
	return a.jiraSide().RemoteVersion(ctx, entityType, externalKey)
}

func (a *Adapter) RemoteAhead(base, remote backend.VersionToken) bool {
	return a.jiraSide().RemoteAhead(base, remote)
}

// --- requirements ---

func (a *Adapter) ListRequirements(ctx context.Context, profileProjectKey string, sources []backend.RequirementSourceSpec, onProgress func(done, total int)) ([]backend.Requirement, []backend.RequirementLink, error) {
	return a.jiraSide().ListRequirements(ctx, profileProjectKey, sources, onProgress)
}

func (a *Adapter) UpdateTestRequirements(ctx context.Context, testKey string, add []string, removeLinkIDs []string) error {
	return a.jiraSide().UpdateTestRequirements(ctx, testKey, add, removeLinkIDs)
}

func (a *Adapter) ListIssueLinkTypes(ctx context.Context) ([]string, error) {
	return a.jiraSide().ListIssueLinkTypes(ctx)
}

func (a *Adapter) ListIssueLinkTypeDetails(ctx context.Context) ([]backend.IssueLinkType, error) {
	return a.jiraSide().ListIssueLinkTypeDetails(ctx)
}

func (a *Adapter) CreateRequirement(ctx context.Context, projectKey, issueType, summary, description, priority, components, fixVersions string, extraFields map[string]any) (string, error) {
	return a.jiraSide().CreateRequirement(ctx, projectKey, issueType, summary, description, priority, components, fixVersions, extraFields)
}

func (a *Adapter) GetRequirementCreateFields(ctx context.Context, projectKey, issueType string) ([]backend.FieldDef, error) {
	return a.jiraSide().GetRequirementCreateFields(ctx, projectKey, issueType)
}

func (a *Adapter) DeleteRequirement(ctx context.Context, requirementKey string) error {
	return a.jiraSide().DeleteRequirement(ctx, requirementKey)
}

func (a *Adapter) UpdateRequirementLinks(ctx context.Context, fromKey string, add []string, removeLinkIDs []string) error {
	return a.jiraSide().UpdateRequirementLinks(ctx, fromKey, add, removeLinkIDs)
}

func (a *Adapter) ListReqToReqLinks(ctx context.Context, reqKeys []string) ([]backend.ReqToReqLink, error) {
	return a.jiraSide().ListReqToReqLinks(ctx, reqKeys)
}

// --- bugs ---
//
// A defect on a step result is Zephyr's own and has no counterpart here; that
// is #371 Phase 6. A defect on the test issue is an ordinary Jira issue link.

func (a *Adapter) ListBugs(ctx context.Context, testProjectKey string, testKeys []string, issueType string, onProgress func(done, total int)) ([]backend.Bug, []backend.BugLink, error) {
	return a.jiraSide().ListBugs(ctx, testProjectKey, testKeys, issueType, onProgress)
}

func (a *Adapter) ListProjectBugs(ctx context.Context, projKey, issueType string) ([]backend.Bug, error) {
	return a.jiraSide().ListProjectBugs(ctx, projKey, issueType)
}

// ListBugsByKeys answers backend.BugKeyReader, which the syncer uses when a
// workspace's bugs live in a different backend from its tests.
func (a *Adapter) ListBugsByKeys(ctx context.Context, keys []string) ([]backend.Bug, error) {
	return a.jiraSide().ListBugsByKeys(ctx, keys)
}

func (a *Adapter) GetFieldDefs(ctx context.Context, projectKey, issueType string) ([]backend.FieldDef, error) {
	return a.jiraSide().GetFieldDefs(ctx, projectKey, issueType)
}

func (a *Adapter) CreateBug(ctx context.Context, projectKey, issueType, summary, description, priority string, labels []string, extraFields map[string]any) (string, error) {
	return a.jiraSide().CreateBug(ctx, projectKey, issueType, summary, description, priority, labels, extraFields)
}

func (a *Adapter) CreateBugLink(ctx context.Context, testKey, bugKey string) error {
	return a.jiraSide().CreateBugLink(ctx, testKey, bugKey)
}

func (a *Adapter) GetBugDetail(ctx context.Context, bugKey string) (backend.BugDetail, error) {
	return a.jiraSide().GetBugDetail(ctx, bugKey)
}

// --- workflow and metadata ---
//
// These statuses are the Jira workflow's, which is a different vocabulary from
// the execution statuses in statuses.go. A test issue moves through a Jira
// workflow; a run carries a Zephyr status id.

func (a *Adapter) GetTransitions(ctx context.Context, key, currentStatus string) ([]backend.Transition, error) {
	return a.jiraSide().GetTransitions(ctx, key, currentStatus)
}

func (a *Adapter) PostTransition(ctx context.Context, key, transitionID string, fields map[string]any) error {
	return a.jiraSide().PostTransition(ctx, key, transitionID, fields)
}

func (a *Adapter) ListStatuses(ctx context.Context, projectKey string) ([]string, error) {
	return a.jiraSide().ListStatuses(ctx, projectKey)
}

func (a *Adapter) ListPriorities(ctx context.Context, projectKey string) ([]string, error) {
	return a.jiraSide().ListPriorities(ctx, projectKey)
}

func (a *Adapter) ProjectComponents(ctx context.Context, projectKey string) ([]string, error) {
	return a.jiraSide().ProjectComponents(ctx, projectKey)
}

func (a *Adapter) ProjectVersions(ctx context.Context, projectKey string) ([]string, error) {
	return a.jiraSide().ProjectVersions(ctx, projectKey)
}

// --- comments ---

func (a *Adapter) AddComment(ctx context.Context, issueKey, body string) error {
	return a.jiraSide().AddComment(ctx, issueKey, body)
}

// --- field payload shaping ---

func (a *Adapter) FieldsForJira(updates map[string]string) map[string]any {
	return a.jiraSide().FieldsForJira(updates)
}
