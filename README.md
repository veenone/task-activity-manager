# agile-suite

Desktop tools for Jira Data Center that share one code spine:

- **Xray Test Manager** (`xtm/`): manage Xray test cases at scale. See
  `xtm/README.md`.
- **Task Activity Manager** (`tam/`): agile task management (in development).

Both are Go + Wails + React apps that sync Jira into a local SQLite cache and
push edits back on commit.

## Syncing XTM from upstream

XTM is still developed in `veenone/xray-testcase-manager`. Pull its commits
in with `.\scripts\sync-xtm-upstream.ps1`, which merges upstream `main`
into `xtm/` with a subtree-shifted merge and leaves the result uncommitted.
Resolve conflicts, then run XTM's Go and Vitest suites before committing.
