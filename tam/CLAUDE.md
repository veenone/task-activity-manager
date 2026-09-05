# CLAUDE.md

Task Activity Manager (TAM) is the agile task-management app of the suite:
Jira DC tasks, epics, stories, bugs, and requirements for scrum masters,
product owners, and team members. It shares connection profiles and the
Windows Credential Manager entries with Xray Test Manager through
`core/profile` and the shared `profiles.db`. The design lives in
`docs/superpowers/specs/2026-09-04-tam-foundation-design.md`; the Outline
collection "Task Activity Manager" mirrors it.

## Status

Plan 1a (issues, read path): sync by project into `tam.db`, the Backlog
grid, and a read-only detail panel, on the demo dataset or a live Jira DC.
Plan 1b adds the journal, create and edit, Commit, Excel import, and links.

## Layout

    main.go              Wails entry point, window, menu
    app.go               App struct: startup, profiles, settings
    app_issues.go        the issue methods: sync, list, detail, per-profile settings
    internal/tamstore/   TAM's own SQLite file (schema version 2: issue, issue_link, sync_state, profile_setting)
    internal/backend/    IssueBackend seam and DTOs; backend/jira on core/jira, backend/demo on internal/demo
    internal/demo/       the Acme Platform (PLAT) dataset behind a "demo" profile
    internal/issuerepo/  the store layer: issue cache, detail cache, links, sync state, profile settings
    internal/syncer/     the paging engine; emits tam:sync-progress through app_issues.go
    internal/suiteprofiles/  which shared profiles TAM shows, demo detection, validation
    frontend/            React app on @agile-suite/core (see ../frontend/core)
      src/api.ts         typed access to the bindings; plain shapes for fixtures
      src/queries/       TanStack Query keys, hooks, and the post-sync invalidation
      src/contexts/      SyncContext on the shared sync reducer
      src/components/    BacklogView, IssueTable, IssueDetailPanel, ProfilesModal, AboutModal
      wailsjs/           GENERATED bindings, do not hand-edit

## Commands

    wails dev                      # run with hot reload
    wails build                    # build/bin/task-activity-manager.exe
    go test ./internal/...         # Go tests
    cd frontend; npx vitest run    # frontend tests
    cd frontend; npm run build     # tsc + vite build

`npm install` runs at the repo root (npm workspaces). `frontend:install` in
wails.json does that for you.

## Conventions

Same as XTM's: logic in `internal/`, `app.go` only adapts it to Wails; Jira
is the system of record; credentials go to the OS credential manager only;
`TODO(tam): desc` marks planned work. TAM creates Jira profiles, which core
stores with backend `xray`; Kiwi profiles from XTM are hidden. UI text uses
no em dashes.

Sync scope is `project = KEY AND issuetype in (Task, Epic, Story, Bug, <requirement
type>)` plus the profile's scope JQL; incremental syncs add `updated >=` the last
sync minus an hour. The requirement type name is the per-profile setting
`requirement_issue_type`. The Sprint and Epic Link field shapes are marked
`NOTE(tam)` in `internal/backend/jira/fields.go` until verified on a real
instance.
