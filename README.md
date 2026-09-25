# agile-suite

Desktop tools for Jira Data Center that share one code spine:

- **Xray Test Manager** (`xtm/`): manage Xray test cases at scale. See
  `xtm/README.md`.
- **Task Activity Manager** (`tam/`): agile task management for scrum
  masters, product owners, and team members (tasks, epics, stories, bugs,
  requirements). It shares connection profiles with XTM, syncs a project's
  issues into a local cache, and shows them in a Backlog grid with a
  read-only detail panel. Plan 1b adds local edits and drafts, a journal,
  Commit with conflict detection, and an Activity tab. Plan 1c adds Excel
  import to drafts, cross-project links, and requirement creation. Phase 2
  adds an Epics view showing each epic with its stories, tasks, bugs, and
  requirements, and lets an issue be moved under one. Phase 3a adds a
  read-only Boards view: a project's boards, their columns, and their
  sprints, synced from Jira's Agile API. Phase 3b makes the board
  writable, journaling a dragged or keyboard-moved card offline and
  pushing it on Commit like every other TAM write. Phase 3c adds the
  sprint ceremonies: starting and completing a sprint, moving several
  selected cards into one at once, and the detail panel's sprint field.

Both are Go + Wails + React apps that sync Jira into a local SQLite cache and
push edits back on commit.

## Frontend workspaces

The three React packages (`frontend/core`, `xtm/frontend`, `tam/frontend`)
are npm workspaces sharing one lock file at the repo root. Run `npm install`
once at the root, then:

```bash
npm test --workspaces --if-present       # every Vitest suite
npm run typecheck --workspaces --if-present   # every workspace's type check
```

## Remotes

One remote: `veenone/task-activity-manager` on GitHub. `git push` reaches it
and nothing else, and a pull request merged there needs no follow-up.

The repository used to push to two more: `veenone/xray-testcase-manager`,
which became a separate project after Phase 3a and now has its own `main`,
and a mirror on a home Gitea. `scripts/sync-remotes.ps1` existed to keep the
three in step and is gone with them. A clone from that era still carries the
fan-out in `git remote -v`; `git remote set-url --push origin
git@github.com:veenone/task-activity-manager.git` clears it.

XTM releases are tagged `xtm/vX.Y.Z` (see `xtm/README.md`).
