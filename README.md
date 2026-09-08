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
  pushing it on Commit like every other TAM write.

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

The repository lives on three remotes, all equal: `veenone/task-activity-manager`
and `veenone/xray-testcase-manager` on GitHub, and `achmarah/xray-test-manager`
on the home Gitea. On a fresh clone, run `.\scripts\sync-remotes.ps1 -Setup`
once (it points `origin` at task-activity-manager and makes it push to all
three); from then on `git push` reaches every remote. After merging a pull
request on GitHub, run `.\scripts\sync-remotes.ps1` to fast-forward the
others. Merge Dependabot pull requests on task-activity-manager only.

XTM releases are tagged `xtm/vX.Y.Z` (see `xtm/README.md`).
