# Plan: Zephyr Essential as a third XTM backend

Design and gap analysis:
[2026-10-30-zephyr-essential-bridge-design.md](../specs/2026-10-30-zephyr-essential-bridge-design.md).
Read it first. This plan assumes its verdicts rather than restating them.

Target: Zephyr Essential, Data Center only. Cloud is out of scope.

## Scale, stated up front

Kiwi TCMS, the existing second backend, is 7392 lines of adapter plus its
tests. Zephyr Essential is a closer fit than Kiwi because a test is a Jira
issue, so about 38 of the 86 interface methods are satisfied by Jira calls XTM
already makes. A realistic estimate is 3500 to 4500 lines of adapter and a
comparable weight of tests, spread over the phases below.

## Sequencing, and what blocks what

Phases 1 to 3 need nothing that does not already exist. They can start now.

Phase 4 needs a scratch project on a Zephyr instance, because six of the writes
the adapter must make are documented but unproven and cannot be confirmed by
reading. It is a phase of its own, deferred until that project exists, and it
blocks Phase 5 alone.

So the critical path is 1, 2 and 3 now, then 4 when an instance is available,
then 5 and 6. Nothing before Phase 5 waits on evidence.

## Phase 1: transport, auth and capabilities

The smallest thing that proves the shape is right.

1. `internal/backend/zephyr/` with `adapter.go` and `client.go`. No transport
   abstraction: there is one transport, and an interface with one
   implementation would be structure serving a phase that is not being built.
2. A body-validating HTTP client. An auth failure that arrives as `200` with an
   `errorId` of `"ERROR"` must surface as an error, not as success. This is the
   first test written, before any endpoint.
3. `TestConnection` as a two-part check: Jira `/myself`, then
   `/util/zephyrTestIssueType` to prove Zephyr is installed and to capture the
   Test issue type id.
4. The status vocabulary cache in `statuses.go`, read from
   `/util/testExecutionStatus` and `/util/teststepExecutionStatus` at connect
   time and held per profile. Name to id and id to name, with an explicit
   error for a name the instance does not define.
5. The capability declaration from the design document.
6. Profile plumbing: a `zephyr` value for `profile.Backend`, routed in
   `app.go`'s `backendFor`, with the credential reusing the existing Jira PAT
   path in Windows Credential Manager.

Verify: a Zephyr profile connects, reports its capabilities, and a test asserts
the status map is built from the instance rather than a constant, including a
non-default status such as `PASS WARNING`.

Gate: `go test ./internal/backend/zephyr/...`, instruction gate, ratchet.

## Phase 2: the read path

Everything a user can see without writing anything.

1. Key-to-id resolution in `ids.go`, with a per-profile cache, since every
   step call needs the Jira issue id.
2. Tests: `SearchTestsPage`, `ListTestsBasic`, `GetTestFields`,
   `SearchTestsAcrossProjects`, `GetTestMeta`. These are Jira calls; the work
   is wiring, not translation.
3. Steps: `GetTestSteps`, unwrapping `stepBeanCollection` and reading the plain
   `step`, `data` and `result` fields rather than the HTML ones.
4. Containers: `ListContainers` from `/cycle`, handling the map-keyed response
   and deciding the `-1` pseudo-cycle's treatment. `TestExecutionsForTest` from
   traceability. `GetTestRuns` for a cycle's executions.
5. Concurrency: `RemoteVersion` and `RemoteAhead` on the Jira `updated` field.
6. Metadata, workflow, requirements and bugs: all Jira, all nearly free.
7. The 21 unsupported methods return `ErrUnsupported` or an empty result, as
   Kiwi does, and the four dark views render their empty states.

Verify: against a real instance, a Zephyr profile lists tests, shows their
steps, lists cycles and shows a test's executions. A dark view explains itself
rather than looking like a failed load.

## Phase 3: custom fields, with a scope on the shared model

The namespace question is **decided**: `CustomFieldDef` grows a scope field.
Prefixing field ids inside one flat list was the alternative and was rejected,
because a prefix encodes meaning in a string every consumer then has to parse,
and the two namespaces are genuinely different things rather than one list with
awkward names.

1. Add a scope to `CustomFieldDef` in `internal/backend`, distinguishing a
   field on the Jira issue from a Zephyr field on an `EXECUTION` or a
   `TESTSTEP` entity. This is shared model code, so Xray and Kiwi both declare
   the issue scope and neither changes behaviour.
2. `ListCustomFields` reads both namespaces: Jira `/rest/api/2/field` and
   Zephyr `/customfield/entity?entityType=EXECUTION` and `=TESTSTEP`.
3. `GetTestCustomFields` and `CustomFieldValue` stay on the Jira side.
4. The six Xray-specific resolvers return `ok=false`, which the commit engine
   already treats as "skip this field".
5. The UI distinguishes the scopes rather than showing one undifferentiated
   list, because a field on a step and a field on the test are not
   interchangeable.

Because this touches a shared type it carries a wider test burden than its size
suggests: Xray and Kiwi must both still report their fields unchanged.

Verify: a field defined in Zephyr on a step and a field defined in Jira on the
test both reach the UI, are distinguishable by scope, and Xray's and Kiwi's
existing field tests pass untouched.

## Phase 4: write endpoint verification (deferred)

**Blocked on a scratch project.** No production code in this phase. Its whole
purpose is to turn six documented but unproven writes into measured facts
before any adapter depends on them, and that cannot be done by reading.

The six, all writes, none confirmed:

| Operation | Documented endpoint | Needed by |
|---|---|---|
| Update a step | `PUT /rest/zapi/latest/teststep/{issueId}/{stepId}` | `UpdateTestStep` |
| Delete a step | `DELETE` on the same path | `DeleteTestStep` |
| Reorder steps | by `orderId`, path unknown | `MoveTestStep` |
| Delete an execution | unknown | `RemoveTestsFromContainer` |
| Delete a cycle | unknown | `DeleteContainer` |
| Link a defect to an execution | unknown | `AddTestRunDefect`, `RemoveTestRunDefect` |

Also to settle here:

1. Whether `addTestsToCycle` returns a job token for a single test or only for
   bulk, and how long a realistic add takes to settle.
2. What `/cycle` returns for a project with no versions, and whether the `-1`
   pseudo-cycle is always present.
3. Whether a step result can be created, or only updated once an execution
   exists.
4. Whether `PUT /stepResult/{id}` takes the step result's own `id` or its
   `stepId`. The docs label these two inconsistently and say to test it, so
   this is a known unknown rather than an oversight.

Why a scratch project and not the live instance: every item is a write and
several are deletes, so running them against real executions would corrupt
test records. When the time comes, either a throwaway Jira project with Zephyr
enabled on the existing instance, or a SmartBear trial. Reading the
JavaScript-rendered Apiary reference with a browser would shorten the list but
cannot replace the probes, because the documentation is exactly what is in
doubt.

Verify: every row above is replaced in `agents/project/domain-context.md` with
a real request and response, or the method is reclassified as unsupported with
the evidence for that.

## Phase 5: the write path

Blocked on Phase 4 for the six endpoints above. The rest can be built before
it, because `SetTestRunStatus` and `addTestsToCycle` both have documented
request and response shapes.

1. `SetTestRunStatus` as the documented two-step write: resolve the execution
   id through traceability, map the status name to this instance's numeric id,
   then `PUT /execution/{id}/execute`.
2. `AddTestsToContainer` with the asynchronous job token. Follow the existing
   deferred-row pattern in the syncer, the one `test_run_iteration_import`
   already uses, rather than blocking inside a commit.
3. `CreateContainer` for a cycle.
4. Steps create, then update, delete and reorder once Phase 4 proves them.
5. `DeleteContainer` and `RemoveTestsFromContainer`, likewise.
6. Run-level defects and comments, likewise.
7. Value-based conflict handling for cycles, executions and step results, which
   have no version token. `valueComparedOnCommit` in `testrepo/conflict.go` is
   the existing hook, added for #369.

Verify: a status set locally is journalled, shows as not committed, and reaches
Zephyr on Commit. A bulk add of several tests to a cycle completes without
blocking the commit, and its outcome is reported per test.

## Phase 6: step results on a plain execution

New surface, not a mapping, and the one thing Zephyr does better than Xray.
Still an open question, recorded as Q3 in the design document.

Zephyr step results have their own ids, comments and defect links, and exist on
ordinary executions. XTM models step results only inside a parameterized
iteration. Doing this properly means a step-result concept that does not depend
on a dataset.

Deliberately last, because like Phase 3 it changes shared model code rather
than adding an adapter, and unlike Phase 3 nothing else depends on it.

## Documentation, moving with the behaviour

Per P10, not as a final phase:

- The user guide gains a Zephyr section covering what a Zephyr profile can and
  cannot do, with the four dark views named plainly.
- `agents/project/xtm-architecture.md` gains Zephyr beside Xray and Kiwi.
- `agents/project/domain-context.md` carries the measured API facts, and Phase
  4 is where the six unproven writes join them.
- The changelog entry says what a Zephyr profile supports, not that a backend
  was added.

## What this plan does not do

- It does not promise feature parity with an Xray profile, which is impossible
  on Essential: no repository, no preconditions, no datasets, no test plans.
- It does not migrate data between backends. A compatibility bridge means XTM
  can talk to Zephyr, not that tests move from Xray to Zephyr. If migration is
  wanted, that is separate work and deserves its own issue.
- It does not target Zephyr (ex-Scale), whose richer model would close most of
  the gaps. That is a different product and a different decision.
- It does not cover Essential Cloud. That is the former Squad Cloud, with a
  JWT per endpoint and query string and its own credential type, and it is
  explicitly out of scope. The design document records what it would take so
  nobody assumes this plan's endpoints apply there.
