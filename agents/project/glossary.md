# Glossary

One name per concept, for code identifiers, briefs, docs, and commit
messages. Each entry says what the thing is, not what it does, and lists
the words to stop using for it. User-facing copy follows the product
wording and is exempt. A quality ratchet may count avoided terms in agent
and developer prose.

## Process

**Contract**:
An architecture entry in `AGENTS.project.md`: what it owns, the sanctioned
path, forbidden bypasses, and its gate.
_Avoid_: architecture rule, architecture convention

**Gate**:
A command that fails a commit, push, or CI run when a rule breaks.
_Avoid_: blocking check, guard script

**Brief**:
The requirements file handed to a subagent for one task.
_Avoid_: task prompt, instructions file

**Spec**:
A design document approved before a plan is written.
_Avoid_: design doc

**Seam**:
The interface a test exercises; the highest one that reaches the behavior.
_Avoid_: test boundary, layer under test

**Proven red**:
A new test that was run against the pre-change code and shown to fail there.
_Avoid_: verified failing, shown failing

## agile-suite

**TAM**:
Task Activity Manager, the app in `tam/`. Syncs a Jira project's issues and
shows them in the Backlog, Epics, Boards, Sprints, Reports and Rituals views.
_Avoid_: task manager, the activity app

**XTM**:
Xray Test Manager, the app in `xtm/`. A separate product line in the same
repository, with its own releases and its own default branch upstream.
_Avoid_: the test app, xray manager

**Profile**:
One connection to one Jira project, holding its URL, scope, and the ids its
credentials are stored under. Everything local is keyed by profile.
_Avoid_: connection, workspace, account

**Journal**:
The pending-change log. A local edit is journaled and reaches Jira on Commit;
the local store is a cache plus this journal, never authoritative.
_Avoid_: queue, outbox, drafts

**Ritual**:
One of the four agile ceremonies a sprint has: planning, standup, review,
retro. A ritual document is the local record of one, published to Confluence;
the done agreement below is a document synced the same way without being a
ritual.
_Avoid_: ceremony, meeting, event

**Done agreement**:
What a team agreed has to be true before a piece of work counts as done. A
ritual document, one per board and optionally one per sprint holding that
sprint's additions. It decides nothing: `donerule`, `backend.IsDone` and
`lib/unfinished.ts` each answer whether an issue reads as finished from
Jira's data, and `donerule`'s package comment distinguishes all four.
_Avoid_: definition of done, DoD, done rule, done criteria

**Sprint report**:
The reconstruction of one sprint from its issues' changelog: committed, added,
removed, completed, carried over.
_Avoid_: burndown, sprint summary

**Zephyr Essential**:
The former Zephyr Squad, SmartBear's lighter Jira test management tier,
renamed April 2025. Its Data Center API is `{JIRA}/rest/zapi/latest`. It has
no test case repository, datasets or test plans.
_Avoid_: Zephyr, Zephyr Squad, Zephyr Scale, Essential

**Zephyr**:
The former Zephyr Scale, SmartBear's richer tier, whose API is
`api.zephyrscale.smartbear.com/v2`. A different product from
[Zephyr Essential](#zephyr-essential) with a different API and a wider feature
set, so the bare word names this one and never the family.
_Avoid_: Zephyr Scale, TM4J

**Cycle**:
Zephyr's only container kind, holding executions and grouping them in folders.
It is the Zephyr counterpart of a Test Execution, not of a Test Plan or Test
Set, neither of which Zephyr Essential has.
_Avoid_: test cycle, sprint, run group

**Step result**:
One step's outcome within one execution. In Zephyr it is an entity with its
own id, comment and defect links, addressable at `PUT /stepResult/{id}`. In
Xray no such id exists and a step reports its iteration's status instead,
which is why a step result cannot be written there.
_Avoid_: step status, step execution, per-step result
