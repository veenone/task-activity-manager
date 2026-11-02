package zephyr

import (
	"context"
	"fmt"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/jira"
)

// Adapter implements backend.Backend for a Zephyr Essential Data Center
// instance.
//
// It holds two clients because Zephyr splits in two: a test is an ordinary
// Jira issue, so its fields, workflow, comments, requirements and defects are
// the Jira calls XTM already makes, and only cycles, executions, steps and
// step results live behind /rest/zapi/latest. Wrapping the existing Jira
// client rather than reimplementing those reads is what keeps this backend
// smaller than the Kiwi one.
//
// Phase 1 (#371) implemented the connection test and the capability
// declaration, Phase 2 the read path: the Jira half in jiraside.go, steps in
// steps.go, cycles and executions in containers.go. What is left answers
// backend.ErrUnsupported until the phase that fills it, in unsupported.go, or
// permanently, in absent.go.
type Adapter struct {
	j *jira.Client
	z *Client
}

// New pairs a Jira client with a Zephyr client for the same instance. Both
// authenticate with the same personal access token.
func New(j *jira.Client, z *Client) *Adapter { return &Adapter{j: j, z: z} }

// Compile-time assertion that the adapter satisfies the interface.
var _ backend.Backend = (*Adapter)(nil)

// --- connection / auth ---

// TestConnection is a two-part check, and both parts matter.
//
// Jira answering /myself proves only that the token works. It says nothing
// about whether Zephyr is installed, so a Zephyr profile pointed at a plain
// Jira would otherwise connect cleanly and then fail on every call that
// touches a cycle. Asking for the Test issue type settles it and captures the
// id at the same time.
//
// Both status vocabularies are read here too. They are needed by everything
// that reports or writes a result, and an instance that cannot answer them is
// better refused at connect time than one endpoint at a time later.
func (a *Adapter) TestConnection(ctx context.Context) (*backend.User, error) {
	u, err := a.j.TestConnection(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := a.z.TestIssueTypeID(ctx); err != nil {
		return nil, err
	}
	// And that the type Zephyr named is the one the Jira searches look for.
	// Without this the profile connects, syncs, and reports a project with no
	// tests in it; see verifyTestIssueTypeName.
	if err := a.verifyTestIssueTypeName(ctx); err != nil {
		return nil, err
	}
	if _, err := a.z.ExecutionStatuses(ctx); err != nil {
		return nil, fmt.Errorf("read this instance's execution statuses: %w", err)
	}
	if _, err := a.z.StepStatuses(ctx); err != nil {
		return nil, fmt.Errorf("read this instance's step statuses: %w", err)
	}
	if u == nil {
		return nil, nil
	}
	return &backend.User{Name: u.Name, DisplayName: u.DisplayName, Email: u.Email}, nil
}

// runStatusNames is the result vocabulary to offer for this instance.
//
// Zephyr configures its statuses per instance, so the names come from the
// vocabulary read at connect. Capabilities() takes no context and must not
// make a request, and app.go's GetCapabilities does not promise a connection
// test ran on this same backend first, so a cold cache falls back to the six
// the probed instance declared. Offering Xray's names instead would let a user
// pick a result Zephyr has no id for and find out only at commit.
func (a *Adapter) runStatusNames() []string {
	// Capabilities() is the one method callers reach for without having built
	// a Zephyr client, so it answers rather than panicking.
	if a.z == nil {
		return documentedRunStatuses
	}
	if v, ok := a.z.execStatuses.peek(); ok {
		if names := v.Names(); len(names) > 0 {
			return names
		}
	}
	return documentedRunStatuses
}

// documentedRunStatuses is what the probed instance returned. It is a fallback
// for a capability read that happens before the connection test, not an
// assumption about every instance: an instance that renamed one of these is
// read correctly as soon as the vocabulary has been fetched, and a name it
// does not define is refused by IDFor rather than written as a guess.
var documentedRunStatuses = []string{"UNEXECUTED", "PASS", "FAIL", "WIP", "BLOCKED", "PASS WARNING"}

// IsDemo is always false. There is no offline Zephyr instance to generate, as
// there is for Kiwi.
func (a *Adapter) IsDemo() bool { return false }

// SetRequirementLinkType configures the Jira issue-link type used when linking
// a test to a requirement. Requirements are Jira issues here, so this is the
// Jira client's setting unchanged.
func (a *Adapter) SetRequirementLinkType(name string) { a.j.SetRequirementLinkType(name) }

// --- capabilities ---

// Capabilities declares what a Zephyr Essential profile can do.
//
// The four absences are the ones the probes confirmed by 404, not guesses:
// there is no test case repository, no preconditions, no datasets or
// parameters, and no test plans. Each is a view that goes dark on a Zephyr
// profile, which is what these flags are for. Zephyr's folders group
// executions inside a cycle rather than organising test cases, so they are not
// the folder tree SupportsFolders promises and the flag stays off.
//
// The parameterized-run flags are off for the same reason: Essential has no
// datasets, so there is no iteration to report. That is not because step
// results are missing. Zephyr's step results are first-class, with their own
// ids, comments and defect links, which is the one thing it does better than
// Xray; they just do not hang off a dataset. Phase 6 is where that becomes a
// capability of its own rather than a parameterized-run flag.
func (a *Adapter) Capabilities() backend.Capabilities {
	return backend.Capabilities{
		Name:             "zephyr-essential-dc",
		IDStyle:          "opaque",
		SupportsJQLScope: true,
		StepModel:        "objects",

		// Steps read as objects and do not write: CreateTestStep is in
		// unsupported.go, bundled with update, delete and reorder by plan
		// Phase 5 item 4. Nothing can publish tests here until it lands
		// (#378).
		SupportsStepWrites: false,

		SupportsTestTypes:           false,
		SupportsFolders:             false,
		SupportsFolderWrites:        false,
		SupportsPreconditionObjects: false,
		SupportsRequirementObjects:  true,
		SupportsIssueLinkTypes:      true,
		SupportsEnvironments:        false,

		SupportsContainers: true,
		ContainerKinds:     []string{backend.KindTestExec},
		SupportsTestRuns:   true,
		RunStatuses:        a.runStatusNames(),

		StatusModel:                 "workflow",
		SupportsWorkflowTransitions: true,
		SupportsBugCreation:         true,
		SupportsBugLinks:            true,
		SupportsTags:                true,

		SupportsDatasets:      false,
		SupportsDatasetWrites: false,

		// Step results are addressable here, on any execution. This is the one
		// capability Essential has and Xray does not: a step result carries
		// its own id, its own status and its own comment, so #371 Phase 6 gave
		// XTM somewhere to put them. The parameterized trio below stays off
		// because there is no dataset, which is a different question.
		SupportsRunStepResults: true,

		SupportsParameterizedRunDetail:     false,
		SupportsParameterizedRunWrites:     false,
		SupportsParameterizedRunStepWrites: false,

		SupportsComponentAdmin: false,
	}
}
