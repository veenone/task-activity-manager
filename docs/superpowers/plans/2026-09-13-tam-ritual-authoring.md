# Ritual Authoring and Confluence Publishing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let TAM create, edit, save, and publish consistent per-sprint ritual documents locally and to Confluence, with Jira issue context included.

**Architecture:** TAM stores one local draft per profile, board, sprint, and ritual type. The frontend wizard edits that draft and renders Jira-backed template sections; an explicit publish operation creates or updates the associated Confluence page and records its page ID/version locally. Failed publishes preserve the draft and expose a retryable error.

**Tech Stack:** Go, SQLite through `core/store`, Wails app bindings, React/TypeScript, TanStack Query, Confluence Data Center REST API, existing Jira board/issue repositories.

**Spec:** `docs/superpowers/specs/2026-09-13-tam-phase-5-confluence-foundation-design.md`, extended by the approved local-first authoring design from the preceding brainstorming session

## Global Constraints

- Local draft saves must work without a Confluence connection.
- Publishing must be explicit; autosave must never publish.
- Every draft is isolated by `profile_id`, `board_id`, `sprint_id`, and `ritual_type`.
- Confluence tokens remain in the OS credential manager and never enter SQLite or exported profile JSON.
- Rendered Confluence HTML must pass through the existing sanitizer before insertion into the DOM.
- Existing read-only Rituals documents and demo profiles must continue to work.
- Jira issue lists must be scoped to the selected profile, board, and sprint.

---

### Task 1: Add the local ritual draft schema and repository

**Files:**
- Modify: `tam/internal/tamstore/tamstore.go`
- Create: `tam/internal/ritualrepo/ritualrepo.go`
- Create: `tam/internal/ritualrepo/ritualrepo_test.go`

**Interfaces:**
- Produces `Draft` with fields `ProfileID string`, `BoardID int`, `SprintID int`, `RitualType string`, `Title string`, `Remark string`, `Body string`, `IssueKeysJSON string`, `ConfluencePageID string`, `ConfluenceVersion int`, `Status string`, `UpdatedAt string`, and `PublishedAt string`.
- Produces `Get(ctx, profileID string, boardID, sprintID int, ritualType string) (Draft, error)`.
- Produces `Upsert(ctx context.Context, draft Draft) error`.
- Produces `Delete(ctx context.Context, profileID string, boardID, sprintID int, ritualType string) error`.

- [ ] **Step 1: Write failing repository tests** for an empty lookup, an upsert/read round trip, composite-key isolation, and deletion of only the selected draft.
- [ ] **Step 2: Run `go test ./tam/internal/ritualrepo`** and confirm the tests fail because the repository and schema are absent.
- [ ] **Step 3: Add schema version 9** with a `ritual_document` table keyed by `(profile_id, board_id, sprint_id, ritual_type)`, JSON issue keys, publish metadata, and an index by profile.
- [ ] **Step 4: Implement `ritualrepo.Repository`** using parameterized SQL and `INSERT ... ON CONFLICT ... DO UPDATE`.
- [ ] **Step 5: Run `go test ./tam/internal/ritualrepo`** and confirm all repository tests pass.
- [ ] **Step 6: Run `go test ./tam/...`** to verify the migration does not break existing stores.

### Task 2: Add Confluence create/update page transport

**Files:**
- Modify: `core/confluence/client.go`
- Modify: `core/confluence/client_test.go`

**Interfaces:**
- Produces `CreatePage(ctx context.Context, spaceKey, parentID, title, storageHTML string) (Page, error)`.
- Produces `UpdatePage(ctx context.Context, pageID string, version int, title, storageHTML string) (Page, error)`.

- [ ] **Step 1: Add `httptest` cases** asserting POST payload shape, parent page nesting, space key, storage representation, bearer auth, PUT version increment, and HTTP error mapping.
- [ ] **Step 2: Run `go test ./core/confluence`** and confirm the new tests fail.
- [ ] **Step 3: Implement JSON request structs** matching Confluence storage API payloads and decode returned pages through the existing model.
- [ ] **Step 4: Reject an update with a non-positive version** before making a network request.
- [ ] **Step 5: Run `go test ./core/confluence`** and confirm all transport tests pass.

### Task 3: Add draft and publish app seams

**Files:**
- Modify: `tam/app.go`
- Create: `tam/app_ritual_documents.go`
- Create: `tam/app_ritual_documents_test.go`
- Modify: `tam/api_bindings.go` or the generated binding source used by this repository

**Interfaces:**
- `GetRitualDraft(profileID string, boardID, sprintID int, ritualType string) (ritualrepo.Draft, error)`.
- `SaveRitualDraft(draft ritualrepo.Draft) error`.
- `PublishRitualDraft(profileID string, boardID, sprintID int, ritualType string) (ritualrepo.Draft, error)`.
- `ListSprintIssues(profileID string, boardID, sprintID int) ([]IssueSummary, error)`.

- [ ] **Step 1: Write app tests** for local save without credentials, missing Confluence configuration, missing credentials, create-page publish, update-page publish, and publish failure retaining the draft.
- [ ] **Step 2: Run `go test ./tam -run Ritual`** and confirm the new tests fail.
- [ ] **Step 3: Wire `ritualrepo.Repository` into `App` startup** and close it with the local store.
- [ ] **Step 4: Implement draft validation** requiring a profile, positive board/sprint IDs, one of `planning|standup|review|retro`, and non-empty title/body.
- [ ] **Step 5: Implement publishing**: load the draft, load profile Confluence config and credential, call `CreatePage` when no page ID exists, call `UpdatePage` otherwise, then persist returned page ID/version and `PublishedAt`.
- [ ] **Step 6: Include the remark and Jira issue block** in the generated storage HTML before sending it to Confluence.
- [ ] **Step 7: Run `go test ./tam -run Ritual`** and confirm all app tests pass.

### Task 4: Build Jira-backed ritual templates

**Files:**
- Create: `tam/frontend/src/lib/ritualTemplates.ts`
- Create: `tam/frontend/src/lib/ritualTemplates.test.ts`
- Modify: `tam/frontend/src/api.ts`

**Interfaces:**
- `type RitualType = "planning" | "standup" | "review" | "retro"`.
- `buildRitualTemplate(type: RitualType, context: RitualTemplateContext): RitualTemplate`.
- `RitualTemplate` contains `title`, `remarkPrompt`, `sections`, and `issueKeys`.

- [ ] **Step 1: Write pure tests** for all four ritual types, empty issue lists, completed/uncompleted issue grouping, sprint goal insertion, and stable issue ordering.
- [ ] **Step 2: Implement templates** with editable headings and concise prompts:
  - Planning: goal, priorities, capacity, risks.
  - Standup: done, next, blockers.
  - Review: delivered, unfinished, feedback.
  - Retrospective: keep, improve, try, actions.
- [ ] **Step 3: Insert issue links using the selected profile’s Jira URL and issue keys.**
- [ ] **Step 4: Run `npm.cmd test --workspace tam/frontend -- --run src/lib/ritualTemplates.test.ts`** and confirm all tests pass.

### Task 5: Implement the ritual wizard UI

**Files:**
- Create: `tam/frontend/src/components/RitualWizard.tsx`
- Create: `tam/frontend/src/components/RitualWizard.test.tsx`
- Modify: `tam/frontend/src/components/RitualsView.tsx`
- Modify: `tam/frontend/src/App.css`

**Interfaces:**
- `RitualWizard` accepts `profileId`, `boardId`, `sprintId`, `ritualType`, `initialDraft`, `issues`, `onSaved`, and `onPublished`.
- Wizard steps are `context`, `content`, `issues`, and `review`.

- [ ] **Step 1: Write component tests** for step navigation, local save, unsaved-change indicator, issue selection, validation, and publish success/failure states.
- [ ] **Step 2: Implement the context step** with ritual type, title, sprint goal, and a required short remark.
- [ ] **Step 3: Implement the content step** with editable template sections and plain-text/Markdown-safe editing controls.
- [ ] **Step 4: Implement the issues step** with profile-scoped Jira issue search/list, selected-key toggles, and clear issue counts.
- [ ] **Step 5: Implement the review step** showing the exact remark, rendered content, issue links, destination page state, and separate **Save draft** and **Publish to Confluence** actions.
- [ ] **Step 6: Add keyboard focus management, `aria-current` step state, disabled states during writes, and retryable publish errors.
- [ ] **Step 7: Run the focused wizard tests** and confirm they pass.

### Task 6: Add per-sprint document management to Rituals

**Files:**
- Modify: `tam/frontend/src/components/RitualsView.tsx`
- Modify: `tam/frontend/src/components/ProfileForm.tsx`
- Modify: `tam/frontend/src/queries/keys.ts`
- Create or modify: `tam/frontend/src/queries/rituals.ts`
- Modify: `tam/frontend/src/App.css`

- [ ] **Step 1: Add query hooks** for draft reads, draft saves, publishing, and profile/board/sprint issue lists.
- [ ] **Step 2: Make board and sprint scope explicit** in the Rituals toolbar and association list.
- [ ] **Step 3: Add **New ritual** actions for each ritual type and selected sprint.
- [ ] **Step 4: Show document state per ritual**: Draft, Published, Unpublished changes, or Publish failed.
- [ ] **Step 5: Keep existing read-only Confluence pages selectable** and offer “Create managed draft” without overwriting the remote page.
- [ ] **Step 6: Add a compact management list** grouped by sprint and ritual type, with open/edit, delete local draft, and publish actions.
- [ ] **Step 7: Run Rituals component tests and the frontend typecheck.**

### Task 7: Add demo data and migration cleanup

**Files:**
- Modify: `core/confluence/demo.go`
- Modify: `tam/app_rituals.go`
- Modify: `tam/frontend/src/App.test.tsx`
- Create: `tam/frontend/src/components/RitualWizard.demo.test.tsx`

- [ ] **Step 1: Add deterministic demo drafts** for at least two sprint IDs and all four ritual types, with Jira issue links and remarks.
- [ ] **Step 2: Ensure demo publishing stays local and visibly reports “Demo mode”; it must not contact a network endpoint.
- [ ] **Step 3: Verify demo documents render with the same wizard and template paths as configured profiles.
- [ ] **Step 4: Run the demo and App tests.**

### Task 8: Full verification and documentation

**Files:**
- Modify: `docs/superpowers/specs/2026-09-13-tam-phase-5-confluence-foundation-design.md`
- Modify: `docs/superpowers/plans/2026-09-13-tam-phase-5-gaps.md`
- Modify: `README.md` if the user-facing workflow is documented there

- [ ] **Step 1: Update the Phase 5 specification** with local-first drafts, wizard authoring, issue inclusion, and create/update publishing.
- [ ] **Step 2: Add manual verification steps** for two sprints, four ritual types, offline draft recovery, duplicate publication, and a failed Confluence request.
- [ ] **Step 3: Run the complete gate:**
  - `cd core && go vet ./... && go test ./... -count=1`
  - `cd xtm && go vet ./... && go test ./internal/... -count=1`
  - `cd tam && go vet ./... && go test ./... -count=1`
  - `npm run typecheck --workspaces --if-present`
  - `npm test --workspaces --if-present`
  - `cd tam && wails build`
- [ ] **Step 4: Check `git status --short --untracked-files=no`** and confirm only intended implementation/documentation files changed.
