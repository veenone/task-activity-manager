# Getting started

This guide covers what every other guide assumes: connecting TAM to a Jira project, syncing it into the local cache, and the loop that takes a change from your screen to Jira.

## How TAM handles your data

Jira is the source of truth. TAM keeps a local copy of your project's issues, boards, sprints and reports, and reads from that copy, which is why the backlog and the board open instantly on a project of thousands of issues.

Nothing you do reaches Jira on its own. Editing a field, dragging a card, filling a sprint, drafting one, starting one, completing one, deleting one: each writes to the local cache and queues a pending change. Commit is what sends them.

A sync goes the other way, pulling what changed in Jira into the cache. Run one when you start work, and whenever someone else may have changed the project.

## Connect your first profile

A profile is one connection to one Jira project, and it is shared with Xray Test Manager: a project set up in either app is already set up in the other.

Open **File → Profiles…**, or press **Manage** next to the profile picker, then fill in the form:

- **Profile name**: any name you will recognise, for example `Platform team`.
- **Jira base URL**: the address of your Jira server, for example `https://jira.example.com`. Type `demo` here for a profile that runs on sample data.
- **Project key**: the key of the project you plan in, for example `PLAT`.
- **Personal Access Token**: a Jira personal access token. TAM stores it in the Windows Credential Manager, never in its database. **Show token** reveals what you typed, and when you already have a profile for the same server, **Reuse token from** saves pasting it again.

The rest are optional:

- **Scope JQL** narrows what syncs, for example `labels = team-a`. Anything the clause excludes never reaches the cache.
- **Requirement issue type** is the Jira issue type TAM treats as a requirement. Blank means `Requirement`. Changing it makes the next sync pull everything again.
- **Issue detail freshness (minutes)** is how long a cached issue detail is used before TAM asks Jira for it again. Blank means 10 minutes. **0** keeps details cached until you press Refresh, which is what to set if you work with no connection.
- **Confluence Rituals** and **Reports root page** are for the [Rituals](08-rituals.md) and [Reports](06-reports.md) views. Both can be added later.

Press **Test connection** to check the URL and the token, then **Create profile**.

### Try TAM without Jira

A profile whose **Jira base URL** is `demo` runs on generated data and contacts no server. The profile name, project key and token can be anything. A yellow **DEMO** chip sits next to the app name while that profile is active.

Demo mode is a safe place to learn the loop: syncing, editing, dragging cards, committing and completing a sprint all work, on data nobody minds you breaking.

## Sync the project

Press **Sync** at the right of the top bar and choose:

- **Sync changes** pulls what changed since the last sync. This is the everyday one, and it is also **File → Sync** or **Ctrl+R**.
- **Full sync** clears the cached issues and fetches everything. Use it after changing the scope JQL or the requirement issue type, or when the cache looks wrong. It is **Ctrl+Shift+R**.

The status bar at the bottom shows progress while it runs, then the issue count and when it last finished. A count like `38 of 2,943 issues` means the scope JQL, or an issue type TAM does not sync, keeps the rest out.

When a sync fails, the whole bar turns into a red ribbon saying what Jira answered and what to change: the token, the Jira URL and project key, or nothing at all when Jira itself failed. It stays until the next sync, and **Help → Diagnostics** has the log.

**Refresh**, next to Sync, is narrower: it re-reads the view you are on and fetches issue details again. It takes the same lock a sync does, so the two cannot run at once.

## The loop: edit, review, commit

When you have uncommitted work, a **N pending** button appears in the top bar.

1. Press it to open **Pending changes (N)**. Rows are grouped by issue, and each shows the field, the value before and the value after. A sprint drafted here, a card moved between columns and a sprint waiting to be completed all appear as rows of their own.

   ![The Pending changes dialog over the board, holding one status move](images/getting-started-01-pending.png)
2. **Discard** on a row throws that one change away. Nothing in Jira is touched.
3. **Discard all** reverts everything, after a confirmation.
4. **Commit (N)** sends everything that can be sent. The number is what Commit will actually push.

After a commit, a banner reports what happened: what went, what failed, and what was held back.

### When Jira has moved on

If somebody changed an issue in Jira after you edited it here, Commit holds that issue back rather than overwriting their work, and shows a card with three columns: **Base** is the value when you edited, **Mine** is your edit, **Remote** is Jira now.

- **Override** pushes your value over theirs.
- **Keep remote** drops your edit, including a move of that card.

Changes to other issues in the same commit still go.

## Settings and housekeeping

- **File → Settings…** holds the **Report export folder**, where spreadsheets and decks are written.
- **Theme** in the top bar switches between light, dark and the system setting.
- **Help → Diagnostics** shows the database paths, the build and the recent log, which is the first thing to collect when something misbehaves.
- **Help → Take the tour** walks through the view you are on. **Take the first-run tour** walks through the whole loop: sync, plan, commit.
- **Ctrl+B** shows or hides the navigation rail.
