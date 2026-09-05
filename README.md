# agile-suite

Desktop tools for Jira Data Center that share one code spine:

- **Xray Test Manager** (`xtm/`): manage Xray test cases at scale. See
  `xtm/README.md`.
- **Task Activity Manager** (`tam/`): agile task management for scrum
  masters, product owners, and team members (tasks, epics, stories, bugs,
  requirements). It shares connection profiles with XTM and is currently at
  the foundation scaffold stage (shell, Profiles dialog, placeholder views).

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

## Syncing XTM from upstream

XTM is still developed in `veenone/xray-testcase-manager`. Pull its commits
in with `.\scripts\sync-xtm-upstream.ps1`, which merges upstream `main`
into `xtm/` with a subtree-shifted merge and leaves the result uncommitted.
Resolve conflicts, then run XTM's Go and Vitest suites before committing.
