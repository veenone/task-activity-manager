# TAM actions follow the token's Jira permissions

Issue #516.

## Why

TAM offers every action to everyone. An account that may not manage sprints, log work or attach files finds out when Jira answers 403 at Commit, after the change has sat in the journal looking like work that will land.

## What it is

Each sync reads what the token may do in the profile's project and stores the answer. An action whose permission Jira denied is disabled, and says which permission is missing.

| Jira permission | Actions |
|---|---|
| Manage Sprints | Create, edit, start, complete, delete sprint |
| Schedule Issues | Rank, move an issue to a sprint |
| Create Issues | New issue, new subtask, import |
| Edit Issues | Field edits in the detail panel |
| Transition Issues | Moving a card between board columns |
| Assign Issues | Changing the assignee |
| Link Issues | Add link |
| Work On Issues | Log work, in the panel and the Worklog view |
| Create Attachments | Attach files |

## Goals

- A user sees what they cannot do before they try it.
- A gap in what TAM knows never takes away something the user can do.
- The journal does not collect a change Jira is certain to refuse.

## Not in this version

- Board administration: column limits and creating a board. It is not a project permission, and Data Center has no public endpoint that reports it.
- Per-issue checks. An issue security scheme can still produce a 403 at Commit.
- Projects other than the profile's own. A card from another project on a programme board is unknown, so its actions stay enabled.

## Approaches

**What an unknown answer means.**

1. Allowed (chosen). No sync yet, a failed read, a permission Jira did not list, and an issue in another project all leave the action enabled. Jira still refuses at Commit, which is today's behaviour.
2. Denied. Safer on paper, but an instance that names a permission differently, or one failed request, would lock a user out of work they are allowed to do, with no way to override it.

**Where the check lives.**

1. In the bindings and in the frontend (chosen). The frontend disables the control and gives the reason. The binding refuses the same action, so a stale page or a keyboard path the frontend missed cannot journal it.
2. Frontend only. Less code, but the rule would hold only where someone remembered to wire it.

**Where the answer is stored.**

1. A profile setting holding JSON, by project (chosen). It is small, read whole, and `PurgeProfile` already sweeps `profile_setting`, so it needs no migration and no new purge entry.
2. A table. Nothing here needs a query a table would serve.

## Design

### Jira

`GET /rest/api/2/mypermissions?projectKey=KEY` answers every permission by key with `havePermission`. The `permissions` filter was ignored by the instance recorded in the domain playbook, so TAM reads the full list and keeps the nine keys it acts on.

The key for Manage Sprints is `MANAGE_SPRINTS_PERMISSION`, confirmed against a live `mypermissions` answer on 2026-11-19. If an instance ever names it differently, the key is absent from the answer, which reads as unknown and leaves the sprint actions enabled.

### Backend

`backend.PermissionBackend` is an optional interface with one method, `Permissions(ctx, projectKey)`. The Jira backend implements it. The demo backend does not, so demo mode stores nothing and allows everything.

`tam/internal/permission` holds the keys, their names, and `Set`, whose `Allows` and `Check` apply the unknown-is-allowed rule.

### Sync

`Engine.Sync` reads the permissions after the issue types. A failed read or write is logged and does not fail the sync. `PutPermissions` does not write an empty answer, so the previous one stays.

### Bindings

`GetPermissions(profileID)` answers the profile's project, the stored set, and the permission names. Each gated binding calls `allow` or `allowOnIssue` before it journals. Two exceptions:

- A draft sprint can be edited and deleted without Manage Sprints. It is local until Commit.
- A dry-run import is not gated. It writes nothing.

### Frontend

`useDenial(profileId)` returns a function from a permission, and optionally an issue key, to the reason the action is held back, or an empty string. Components disable the control on a non-empty answer and put it in the `title`; forms with a hint line show it there. The query is invalidated when a sync finishes.

A board card stays draggable while either a column move or a sprint move is open to it. A drop of the kind it may not make is refused by the binding with the reason.

## Left to the builder

Wording of the reason, and which forms show it as text beside the button.
