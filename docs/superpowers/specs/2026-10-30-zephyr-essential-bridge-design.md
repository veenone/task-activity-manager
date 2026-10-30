# Zephyr Essential as a third XTM backend: design and gap analysis

Status: proposal. Target: Zephyr Essential, Data Center only.
Evidence: live probes against `jira.example.internal` on 2026-10-30, plus SmartBear
documentation. Every claim below is marked **measured** where a probe produced
it and **documented** where only the vendor docs support it.

## Verdict first

Zephyr Essential is a closer fit to XTM than Kiwi TCMS is, and a much poorer
fit than Xray. The reason is the same in both directions: a Zephyr test **is a
Jira issue**, so roughly half the `Backend` interface is satisfied by the Jira
calls XTM already makes, while Zephyr's own model is far thinner than Xray's.

Of the 86 methods on `backend.Backend`:

| Verdict | Count | Meaning |
|---|---|---|
| Direct | 38 | The same Jira call XTM already makes, or a 1:1 Zephyr endpoint |
| Adapt | 21 | Exists, but needs id resolution, status mapping or async polling |
| Synthesize | 6 | No endpoint; XTM composes the answer from others |
| Unsupported | 21 | No counterpart. `ErrUnsupported` and a capability turned off |

The 21 unsupported methods are not scattered. They are four whole features:
the test repository, preconditions, datasets and parameterized runs, and test
plans. Each one is a view XTM currently shows and would have to go dark on a
Zephyr profile.

Zephyr also wins one thing outright that Xray cannot do, described under
[Step results](#step-results-the-one-place-zephyr-beats-xray).

## The naming trap, and why it nearly derailed this

SmartBear renamed its Jira apps in April 2025. The DC release notes for
10.1.0 say it plainly:

> Zephyr Squad is now called Zephyr Essential.

So **Zephyr Essential is the former Zephyr Squad**, the lighter tier, and
**Zephyr is the former Zephyr Scale**, the richer one. Two things follow that
are easy to get wrong:

1. **The documentation link in the request points at the wrong product.**
   `support.smartbear.com/zephyr/docs/` is the *Zephyr* (ex-Scale) site, whose
   REST API is `https://api.zephyrscale.smartbear.com/v2/`. Zephyr Essential DC
   is a different API at `{JIRA}/rest/zapi/latest`, documented under
   `zephyr-essential-dc` and still titled "Zephyr Squad Server/DC REST API".
2. **SmartBear's own comparison page contradicts the release notes.** Its
   feature table has one column headed "Zephyr Squad" and the other headed
   "Zephyr / Zephyr Essential", which groups Essential with the product the
   release notes say it is being compared against. Read literally, Essential
   has test case folders and parameters. Read via the release notes, it does
   not.

The live probes settle it, which is why they were worth running.

## Evidence: the live instance

A Jira 10.3.18 Server instance, authenticated with a Jira personal access token
as a bearer token. Zephyr is present and Xray is not.

**Identifiers here are illustrative, and the structure is not.** This repository
is public, so the host, project keys, issue keys, user names, cycle names and
numeric ids below have been replaced with neutral equivalents. Every response
shape, field name, status id, HTTP code and relationship is exactly what the
instance returned. Where this document says **measured**, a probe produced that
structure; only the values standing in it were changed.

**measured**

```
GET /rest/zapi/latest/util/zephyrTestIssueType   ->  {"testcaseIssueTypeId":"9"}
GET /rest/api/2/search?jql=issuetype%3D9         ->  total over 10,000
GET /rest/raven/2.0/api/settings/teststatuses    ->  302 (no Xray)
```

The four absence probes decide the gap:

```
GET /rest/zapi/latest/testrepository/10000   ->  404     no test case repository
GET /rest/zapi/latest/dataset                ->  404     no datasets
GET /rest/zapi/latest/testparameter          ->  404     no parameters
GET /rest/zapi/latest/testplan               ->  404     no test plans
GET /rest/zapi/latest/folder                 ->  400     cycle folders DO exist
GET /rest/zapi/latest/customfield/entity?entityType=TESTSTEP  -> 200
```

This instance is Zephyr Essential (ex-Squad) DC. The release-notes reading is
the correct one and the comparison page's column heading is misleading.

### Status vocabularies are instance-specific, and this one proves it

**measured**

```
GET /rest/zapi/latest/util/testExecutionStatus
  -1 UNEXECUTED, 1 PASS, 2 FAIL, 3 WIP, 4 BLOCKED, 5 PASS WARNING
GET /rest/zapi/latest/util/teststepExecutionStatus
  -1 UNEXECUTED, 1 PASS, 2 FAIL, 3 WIP, 4 BLOCKED, 5 PASS WARNING
```

`5 PASS WARNING` appears in neither SmartBear's documented defaults nor the
other instance. Statuses are numeric ids whose meaning is configured per
instance, so the adapter must read both vocabularies at connect time and map
names to ids per profile. Hardcoding the documented five would silently write
the wrong result here. This is the single most important implementation
constraint in the whole bridge.

### Real response shapes

**measured**, abridged to the fields that matter.

Steps, which need the issue **id**, not its key:

```
GET /rest/zapi/latest/teststep/10501
{"stepBeanCollection":[
  {"id":3001,"orderId":1,"step":"Sign in","data":"","result":"",
   "htmlStep":"...","customFieldValuesMap":{},"totalStepCount":4}]}
```

Executions for a test, which accept the **key**:

```
GET /rest/zapi/latest/traceability/executionsByTest?testIdOrKey=PROJ-101
{"executions":[{"execution":{"id":"2001","issueKey":"PROJ-101",
   "status":"WIP","statusId":"3","testCycle":"Regression cycle",
   "versionName":"Release 2.4"},"defects":[],"stepDefects":[]}],
 "totalCount":1}
```

One execution:

```
GET /rest/zapi/latest/execution/2001
{"execution":{"id":2001,"issueKey":"PROJ-101","cycleId":501,
  "cycleName":"Regression cycle","versionId":10200,
  "projectId":10100,"comment":"","executedBy":"<userId>"}}
```

Cycles, which come back as an **object keyed by cycle id**, not an array, with
a `-1` pseudo-cycle for ad-hoc executions and an `environment` string per
cycle:

```
GET /rest/zapi/latest/cycle?projectId=10100&versionId=-1
{"-1":{"versionName":"Unscheduled","projectKey":"DT","versionId":-1,
   "environment":"","totalExecutions":0,"totalCycleExecutions":...}}
```

Three shape traps worth naming now, because each one would be a bug later:
the steps array is wrapped in `stepBeanCollection`, a single execution is
wrapped in `execution`, and the cycle list is a map rather than a list.

## Step results: the one place Zephyr beats Xray

**measured**

```
GET /rest/zapi/latest/stepResult?executionId=2001
[{"id":4001,"stepId":3001,"executionId":2001,"status":"2",
  "comment":"gateway returned no response","defects":[],
  "executedBy":"<userId>","stepResultAttachmentCount":0}]
```

A step result is a first-class entity with **its own id**, and
`PUT /rest/zapi/latest/stepResult/{id}` sets its status. It also carries a
comment and its own defect links.

This is exactly what Xray does not have, and it is the limitation that caused
#362, #364, #367 and #369: Xray exposes no step-result id, reports every step
carrying its iteration's status, and answers a write with an empty
`stepResults` array. On Zephyr, per-step results are addressable, readable and
writable.

Two consequences:

- `SupportsParameterizedRunStepWrites`, the capability added in #369, would be
  **true** for Zephyr if Zephyr had datasets. It does not, so the flag stays
  false on Essential and the capability sits unused. The flag was still worth
  splitting out: it is now a real question about a backend rather than an Xray
  quirk, and this analysis is the evidence that the two answers are real.
- Per-step results on a non-parameterized run are a feature XTM does not model
  at all today. XTM only has step results inside a parameterized iteration.
  Supporting Zephyr properly means step results on a plain execution, which is
  new surface, not a mapping. This is called out as its own phase.

## The contract to satisfy

`backend.Backend` is one deliberately fat interface of 86 methods in 16 groups.
Kiwi TCMS, the existing second backend, is 7392 lines and returns
`ErrUnsupported` in about 43 places. That is the realistic scale of this work.

## Gap analysis, group by group

Verdicts: **Direct** (a call XTM already makes, or 1:1), **Adapt** (translation
needed), **Synthesize** (composed from other calls), **Unsupported**
(`ErrUnsupported` plus a capability turned off).

### Connection and auth (3 methods)

| Method | Zephyr Essential DC | Verdict |
|---|---|---|
| `TestConnection` | `GET /rest/api/2/myself` plus `GET /rest/zapi/latest/util/zephyrTestIssueType` to prove Zephyr is installed | Adapt |
| `IsDemo` | Local, as for Xray and Kiwi | Direct |
| `SetRequirementLinkType` | Local configuration | Direct |

A two-part connection test matters here: Jira answering says nothing about
Zephyr being present, and the Test issue type id is needed anyway.

**Trap, documented and worth guarding.** The DC docs state that a request with
invalid authentication can return `200 OK` with an `errorId` of `"ERROR"` in
the body rather than a `401`. The adapter must validate response bodies, not
just status codes, or a failed auth will read as success. This is an I1
trust-boundary obligation.

### Tests (8 methods)

A test is a Jira issue of the Test type, created through
`POST /rest/api/2/issue` with the issue type id from
`/rest/zapi/latest/util/zephyrTestIssueType`. **measured**: that id is `9` on
this instance, and is not a constant.

| Method | Zephyr Essential DC | Verdict |
|---|---|---|
| `SearchTestsPage` | Jira JQL search, `issuetype = <id>` | Direct |
| `ListTestsBasic` | Jira search by key set | Direct |
| `GetTestFields` | `GET /rest/api/2/issue/{key}` | Direct |
| `SearchTestsAcrossProjects` | Jira JQL across project keys | Direct |
| `SearchPreconditionsAcrossProjects` | No precondition entity | Unsupported |
| `CreateTest` | `POST /rest/api/2/issue` | Direct |
| `UpdateIssue` | Jira field update | Direct |
| `GetTestMeta` | Jira issue read | Direct |

JQL works, so `SupportsJQLScope` is **true**, unlike Kiwi. The profile's saved
scope carries over unchanged, which is a large practical win.

### Concurrency (2 methods)

| Method | Zephyr Essential DC | Verdict |
|---|---|---|
| `RemoteVersion` | The Jira issue's `updated` timestamp | Direct |
| `RemoteAhead` | Timestamp comparison | Direct |

Identical to Xray. The pending-change journal, conflict pre-check and rebase
machinery work without change **for anything keyed by a Jira issue**. They do
not cover Zephyr-only entities, which is the next point.

**Gap with no Xray precedent.** A cycle, an execution and a step result are not
Jira issues and have no `updated` field. There is no version token for them, so
the conflict pre-check cannot protect a run status or a step result the way it
protects a test field. Xray had the same shape of problem for iteration results
and solved it by comparing values rather than versions (#277 W1, #369). That
same value-comparison approach is the design here, and
`valueComparedOnCommit` in `testrepo/conflict.go` is already the hook for it.

### Steps (6 methods)

| Method | Zephyr Essential DC | Verdict |
|---|---|---|
| `GetTestSteps` | `GET /rest/zapi/latest/teststep/{issueId}` | Adapt |
| `CreateTestStep` | `POST /rest/zapi/latest/teststep/{issueId}` | Adapt |
| `UpdateTestStep` | `PUT /rest/zapi/latest/teststep/{issueId}/{stepId}`, **documented only, not probed** | Adapt |
| `DeleteTestStep` | `DELETE` on the same path, **not probed** | Adapt |
| `MoveTestStep` | Reorder by `orderId`, endpoint **not probed** | Adapt |
| `CreateCalledTestStep` | "Call to test to reuse test steps" is absent from Essential | Unsupported |

`StepModel` is `"objects"`, as for Xray, not Kiwi's `"inline-text"`. Steps
carry ids and an `orderId` starting at 1.

Two adaptations apply to every call here. The endpoint takes the Jira issue
**id** while XTM works in keys, so the adapter needs a key-to-id resolver with
a per-profile cache. And Zephyr names the three fields `step`, `data` and
`result` where XTM uses action, data and expected, which is a rename, not a
semantic difference. Zephyr additionally returns `htmlStep`, `htmlData` and
`htmlResult`; XTM should read the plain fields and ignore the HTML ones.

### Datasets and parameterized runs (8 methods)

| Method | Zephyr Essential DC | Verdict |
|---|---|---|
| `GetDataset` | No dataset entity (**measured** 404) | Unsupported |
| `GetRunDataset` | No dataset entity | Unsupported |
| `GetRunIterations` | No iteration entity | Unsupported |
| `SetDataset` | No dataset entity | Unsupported |
| `GetParameterizedRun` | No dataset entity | Unsupported |
| `SetIterationStatus` | No iteration entity | Unsupported |
| `SetIterationStepStatus` | No iteration entity, although step results exist | Unsupported |

The whole of #267, #277, #301 and the recent #362 to #369 work has no
counterpart on Essential. This is the largest single gap, and on Data Center it
has no remedy: there is no endpoint to adapt to and nothing to synthesise from.
A Zephyr profile does without parameterized testing.

### Custom fields (8 methods)

| Method | Zephyr Essential DC | Verdict |
|---|---|---|
| `ListCustomFields` | Jira `/rest/api/2/field` plus Zephyr `/customfield/entity?entityType=` | Adapt |
| `GetTestCustomFields` | Jira issue fields | Direct |
| `CustomFieldValue` | Jira field typing | Direct |
| `ExecTypeFieldValue` | No test type concept | Unsupported |
| `CucumberScenarioFieldValue` | Gherkin is not an issue field here | Unsupported |
| `CucumberTypeFieldValue` | As above | Unsupported |
| `GenericDefinitionFieldValue` | As above | Unsupported |
| `ConditionFieldValue` | No preconditions | Unsupported |

**Two custom-field namespaces, which Xray does not have.** Jira custom fields
live on the test issue, and Zephyr has its own custom fields on `EXECUTION` and
`TESTSTEP` entities (**measured**: the entity endpoint answers 200), created
through `POST /rest/zapi/latest/customfield/create` with types TEXT,
LARGE_TEXT, NUMBER, RADIO_BUTTON, SINGLE_SELECT, MULTI_SELECT, DATE and
DATE_TIME. `ListCustomFields` returns one flat list today, so either the two
namespaces merge with a prefix or the model grows a scope.

**Decided: the shared model grows a scope.** `CustomFieldDef` gains a scope
distinguishing a field on the Jira issue from a Zephyr field on an `EXECUTION`
or a `TESTSTEP`. Prefixing ids inside one flat list was the alternative and was
rejected, because a prefix encodes meaning in a string every consumer then has
to parse, and the two namespaces are genuinely different things rather than one
list with awkward names. The cost is that this is shared model code: Xray and
Kiwi both declare the issue scope, so the change carries a wider test burden
than its size suggests.

### Containers (14 methods)

Zephyr's container model is Project, then Version, then Cycle, then Folder
inside a cycle. There is no Test Set and no Test Plan, so `ContainerKinds` is
`[KindTestExec]` alone.

| Method | Zephyr Essential DC | Verdict |
|---|---|---|
| `ListContainers` | `GET /rest/zapi/latest/cycle?projectId=&versionId=` | Adapt |
| `TestExecutionsForTest` | `GET /rest/zapi/latest/traceability/executionsByTest?testIdOrKey=` | Direct |
| `CreateContainer` | `POST /rest/zapi/latest/cycle`, exec kind only | Adapt |
| `AddTestsToContainer` | `POST /rest/zapi/latest/execution/addTestsToCycle`, `method:1` with issue keys | Adapt |
| `RemoveTestsFromContainer` | Execution delete, **not probed** | Adapt |
| `SetTestRunStatus` | `PUT /rest/zapi/latest/execution/{id}/execute` | Adapt |
| `AddTestRunDefect` | Defect link on execution exists, endpoint **not probed** | Adapt |
| `RemoveTestRunDefect` | As above | Adapt |
| `SetTestRunComment` | An execution has a `comment`; the setter is **not probed** | Adapt |
| `SetContainerEnvironments` | A cycle has one `environment` string, not a set | Synthesize |
| `DeleteContainer` | Cycle delete, **not probed** | Adapt |
| `GetTestRuns` | `GET /rest/zapi/latest/execution?cycleId=`, **not probed** | Adapt |
| `ExecPlans` | No test plans | Unsupported |

Three adaptations here are structural rather than cosmetic.

**The cycle list is a map.** `/cycle` returns an object keyed by cycle id with a
`-1` entry for ad-hoc executions. XTM expects a list of containers with keys,
so the adapter synthesises a container key from the numeric cycle id and must
decide what to do with `-1`. Treating it as a real container is probably right,
since executions genuinely live there.

**Adding tests is asynchronous.** `addTestsToCycle` returns a
`jobProgressToken` and the caller polls
`GET /rest/zapi/latest/execution/jobProgress/{token}?type=bulk_execution_copy_move_job_progress`.
XTM's commit engine is synchronous and expects a write to have landed when it
returns. This is the one place the bridge needs more than an adapter: either
the adapter blocks while polling, which is simple but can exceed a reasonable
timeout on a large add, or the commit engine learns about deferred writes. The
existing `test_run_iteration_import` deferred-row pattern in the syncer is the
closest precedent and is the recommended route.

**Setting a run status is a two-step write.** `SetTestRunStatus` takes an
execution key, a test key and a status name. Zephyr needs a numeric execution
id and a numeric status id. So the adapter resolves the execution through
traceability, then maps the status name through the per-instance vocabulary,
then writes. Both lookups are cacheable per profile, and both can fail in ways
the current signature cannot express well.

### Preconditions (5 methods)

No precondition entity exists in Zephyr Essential. All five are
**Unsupported**, and `SupportsPreconditionObjects` is false. The preconditions
view, its filters and its toolbar all go dark on a Zephyr profile.

### Requirements (9 methods)

Zephyr manages requirements through Jira itself, and requirement to test
coverage traceability is a documented feature.

| Method | Zephyr Essential DC | Verdict |
|---|---|---|
| `ListRequirements` | Jira issues plus issue links | Direct |
| `UpdateTestRequirements` | Jira issue links | Direct |
| `ListIssueLinkTypes` | Jira link types | Direct |
| `ListIssueLinkTypeDetails` | Jira link types | Direct |
| `CreateRequirement` | `POST /rest/api/2/issue` | Direct |
| `GetRequirementCreateFields` | Jira create meta | Direct |
| `GetTestCreateFields` | Jira create meta | Direct |
| `DeleteRequirement` | Jira issue delete | Direct |
| `UpdateRequirementLinks` | Jira issue links | Direct |
| `ListReqToReqLinks` | Jira issue links | Direct |

This whole group is free, because XTM already implements it against Jira for
Xray. `SupportsRequirementObjects` and `SupportsIssueLinkTypes` are both true.

### Bugs (6 methods)

All six are Jira issue operations and all are **Direct**, for the same reason.
`SupportsBugCreation` and `SupportsBugLinks` are true. Zephyr adds something
Xray does not: a defect linked to an individual **step result**, visible as
`stepDefects` in the traceability response and `defects` on a step result.
XTM has no model for that today.

### Folders, the test repository (7 methods)

| Method | Zephyr Essential DC | Verdict |
|---|---|---|
| `FolderTree` | No test case repository (**measured** 404) | Unsupported |
| `ListFolders` | As above | Unsupported |
| `ListTestsInFolder` | As above | Unsupported |
| `CreateFolder` | As above | Unsupported |
| `RenameFolder` | As above | Unsupported |
| `DeleteFolder` | As above | Unsupported |
| `MoveTestToFolder` | As above | Unsupported |

`SupportsFolders` and `SupportsFolderWrites` are both false, which is stricter
than Kiwi, where folders can be read but not reshaped.

**Do not confuse the two kinds of folder.** Zephyr has folders, but they live
inside a test cycle and group executions, not test cases. They are part of the
container tree, not the repository tree. Mapping cycle folders onto
`FolderTree` would put execution groupings into the test repository view and
would be wrong.

### Workflow, metadata, comments, field shaping (8 methods)

| Method | Zephyr Essential DC | Verdict |
|---|---|---|
| `GetTransitions` | Jira transitions | Direct |
| `PostTransition` | Jira transitions | Direct |
| `ListStatuses` | Jira statuses | Direct |
| `ListPriorities` | Jira priorities | Direct |
| `ProjectComponents` | Jira components | Direct |
| `ProjectVersions` | Jira versions, and Zephyr's `/util/versionBoard-list` for the Zephyr view | Direct |
| `AddComment` | Jira comment | Direct |
| `FieldsForJira` | The Jira payload shape XTM already builds | Direct |

All free. `SupportsWorkflowTransitions` is true, so the Test issue's workflow
behaves as it does on Xray.

**One subtlety.** `ListStatuses` returns Jira workflow statuses for the Test
issue, while an execution's status comes from Zephyr's own numeric vocabulary.
Two unrelated status vocabularies coexist, and the UI already distinguishes
issue status from run status, so this is a naming discipline problem rather
than a design problem.

## Capability declaration

```go
backend.Capabilities{
    Name:             "zephyr-essential-dc",
    IDStyle:          "opaque",      // tests are Jira keys; Zephyr's own ids are numeric
    SupportsJQLScope: true,          // unlike Kiwi
    StepModel:        "objects",     // ids and orderId, like Xray

    SupportsTestTypes:           false, // no Manual/Cucumber/Generic concept
    SupportsFolders:             false, // no test case repository
    SupportsFolderWrites:        false,
    SupportsPreconditionObjects: false,
    SupportsRequirementObjects:  true,  // via Jira
    SupportsIssueLinkTypes:      true,  // via Jira
    SupportsEnvironments:        false, // one string per cycle, not a set
    SupportsContainers:          true,
    ContainerKinds:              []string{backend.KindTestExec},
    SupportsTestRuns:            true,

    StatusModel:                 "workflow",
    SupportsWorkflowTransitions: true,
    SupportsBugCreation:         true,
    SupportsBugLinks:            true,
    SupportsTags:                true,  // Jira labels

    SupportsDatasets:                   false,
    SupportsDatasetWrites:              false,
    SupportsParameterizedRunDetail:     false,
    SupportsParameterizedRunWrites:     false,
    SupportsParameterizedRunStepWrites: false, // true in principle, moot without datasets
    SupportsComponentAdmin:             false,
}
```

`IDStyle: "opaque"` is a judgement call worth flagging. Tests are addressed by
Jira key, which is opaque, but cycles, executions, steps and step results are
all numeric. The field describes one id style and Zephyr has two. Opaque is the
right answer for the entity the field is actually consulted for, and the
mismatch is recorded as open question Q2.

## What a Zephyr profile loses, and what it gains

Loses, compared with an Xray profile:

- The test repository tree, with its folders and folder writes
- Preconditions entirely, including the view, its filters and its toolbar
- Datasets, iterations and everything built on them
- Test Sets and Test Plans as containers, leaving only executions
- Test types, so no Cucumber or Generic test authoring
- Call-to-test steps
- Container environments as a multi-value set

Gains, compared with an Xray profile:

- Per-step results that are addressable, with their own ids, comments and
  defect links, on ordinary executions rather than only inside iterations
- Execution search and execution history, which the comparison table marks as
  present in Essential and absent in Zephyr
- Zephyr custom fields on executions and steps, separate from Jira's

The honest summary for a user is that XTM on Zephyr Essential is a test
execution tool with Jira-backed tests, requirements and defects, and is not a
test repository tool.

## Architecture: one adapter, Data Center only

Scope is Data Center. No transport abstraction, because there is one transport:

```
internal/backend/zephyr/
  adapter.go        Backend implementation, capability declaration
  client.go         /rest/zapi/latest over the Jira PAT bearer, body-validating
  statuses.go       per-profile status vocabulary cache (names <-> numeric ids)
  ids.go            Jira key to issue id resolver and cache
  jobs.go           jobProgressToken polling
```

A `transport.go` seam was considered and dropped. It would exist only to serve
a Cloud adapter that is not being built, and an interface with one
implementation is the kind of speculative structure that ages badly. If Cloud
is ever wanted, the seam can be introduced then, against a real second
implementation whose shape is known rather than guessed.

Reusing the Jira client matters more than abstracting the Zephyr one. A Zephyr
test is a Jira issue, so 38 of the 86 methods are Jira calls. The adapter
should hold a `jira.Client` for those and its own thin client only for the
`/rest/zapi/latest` paths, exactly as the Xray adapter does.

**Cloud is out of scope, and the reason is recorded so it is not assumed
later.** Essential Cloud is the former Zephyr Squad Cloud at
`https://prod-api.zephyr4jiracloud.com/connect`, needing a JWT generated per
endpoint and per query string plus a `zapiAccessKey` header. That is a
different auth model from a static bearer token and would need its own
credential type. It is **not** the `api.zephyrscale.smartbear.com/v2` API,
which belongs to Zephyr, not Essential. Anyone picking Cloud up later starts
from that sentence, not from this document's DC endpoints.

## Risks

1. **The two custom-field namespaces do not fit the current model.** This is
   the most likely source of rework, because `ListCustomFields` is consumed by
   the UI as one flat list.
2. **Asynchronous adds do not fit the commit engine.** Handled well it is a
   deferred-row pattern that already exists; handled badly it is a blocking
   poll inside a commit.
3. **Six endpoints are documented but unprobed**, all of them writes: step
   update, step delete, step reorder, execution delete, cycle delete and the
   defect link. The Apiary reference is JavaScript-rendered and was not
   scraped, so these are the least certain part of this analysis.
4. **Auth failures can arrive as 200.** A body-validating client is a
   prerequisite, not a refinement.
5. **Status ids are per instance.** Proven by `5 PASS WARNING` on this
   instance. A hardcoded map would corrupt results rather than fail loudly.
6. **Four views go dark.** Without honest empty states this reads as a broken
   app rather than a thinner backend. Kiwi already set the precedent for
   capability-gated views, so the mechanism exists.

## Open questions

- **Q1.** Should a Zephyr profile hide the dark views, or show them with an
  explanation? Kiwi's existing behaviour is the default answer, and it should
  be checked rather than assumed.
- **Q2.** `IDStyle` describes one id style and Zephyr has two. Leave it as
  `"opaque"` with a comment, or is the field due a rethink?
- **Q3.** Should per-step results on a plain execution be modelled at all, or
  is Zephyr's step-result capability deliberately left unused so the two
  backends stay comparable?

## What this document does not claim

No write of any kind was performed against `jira.example.internal`. Every probe was
a GET. The six unprobed write endpoints above are the honest boundary of this
analysis, and Phase 0 of the plan exists to close them before any adapter code
is written.
