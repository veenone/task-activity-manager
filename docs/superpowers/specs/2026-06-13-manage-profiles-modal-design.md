# Manage Profiles modal (with delete profile)

**Status:** Approved (design) — 2026-06-13
**Area:** Profile management UI (`frontend/`), no new backend logic

## Problem

Profiles are currently managed through a topbar `<select>` switcher plus a
"Profile" dropdown `Menu` (Set-default / Edit / Scope / Token / Export / Import /
New) and the standalone `ProfileForm` modal for create/edit. Two gaps:

1. **No way to delete a profile from the UI.** The backend already supports it
   fully — `app.go:DeleteProfile(id)` removes the profile, deletes its
   OS-stored credential, purges cached test data, and clears the default-profile
   setting if it pointed at the deleted profile. It is exported in `api.ts` but
   nothing calls it.
2. The dropdown menu is a flat list of seven actions with no overview of all
   profiles at once. There is no single place to see, compare, and manage every
   profile.

## Goal

A dedicated **Manage Profiles** modal that lists every profile and edits the
selected one in a master-detail layout, with **delete** as a first-class
per-profile action. The topbar keeps only the profile switcher plus a button to
open the modal; all management actions move into the modal.

Non-goals: changing how profiles or credentials are stored; multi-select / bulk
profile operations; reworking `ProfileForm`'s field set or validation.

## Approach

### Topbar (`frontend/src/App.tsx`)

- Remove the "Profile" dropdown `Menu` (lines ~908–961).
- The topbar-left zone keeps: brand · DEMO chip · profile `<select>` switcher ·
  a new **⚙ Manage** button that opens the modal (`showProfiles` state).
- Every action from the removed menu (set default, edit, scope, token, export,
  import, new) is reachable from the modal instead, simplifying the topbar.

### Modal — master-detail (`frontend/src/components/ProfilesModal.tsx`, new)

Uses the existing `.modal-overlay` > `.modal` shell. Two panes:

```
┌ Manage Profiles ───────────────────────────────┐
│ LIST (left)          │ DETAIL (right)           │
│ ★ Prod (PROJ)        │  <ProfileForm, embedded> │
│   Staging (STG)   ◄  │   Name / URL / Key /     │
│   Demo (DEMO)        │   Scope / Token          │
│                      │   [Test connection]      │
│ [+ New]  [Import…]   │   [Export] [Delete] [Save]│
└──────────────────────┴──────────────────────────┘
```

**List (left):**
- Each row: a star toggle (★ = current default, ☆ = click to set this profile as
  default), the profile name, and `(PROJECT_KEY)`. A DEMO profile shows the DEMO
  chip.
- The selected row is highlighted; selecting a row loads it in the detail pane.
- Footer: `+ New` (clears detail to a blank create form) and `Import…` (calls
  `ImportProfile()`, then selects the imported profile).

**Detail (right):**
- Embeds the existing `ProfileForm`, lightly adapted to render inline rather than
  as its own modal block. Token and Scope are already fields on the form, so the
  old "Set token" / "Set scope" menu actions are absorbed.
- Footer actions: `Test connection` (existing), `Export` (per selected profile,
  calls `ExportProfile(id)`), `Delete` (red, destructive, confirms), `Save`
  (existing create/update via `ProfileForm`).

### Delete behavior & edge cases

Calls the existing `DeleteProfile(id)`. UI rules:

- **Confirm first:** "Delete profile 'NAME'? This removes its stored token and
  all cached test data. This cannot be undone." (window.confirm — works in
  WebView2; `window.prompt` does not, but confirm/alert do.)
- **Deleting the active profile:** after a successful delete, switch the app's
  `activeId` to another profile — prefer the (possibly newly-cleared) default,
  else the first remaining profile.
- **Deleting the last profile:** close the modal; the app falls back to its
  existing empty state (the standalone create form at `App.tsx:882`).
- **Default cleared automatically** by the backend; the star clears in the list.

### State & reuse

- `ProfilesModal` owns: which row is selected, and whether the detail shows the
  selected profile or a blank "new" form. It receives `profiles`, `activeId`,
  `defaultProfileId`, and callbacks (`onChanged`, `onActiveChange`,
  `onDefaultChange`, `onClose`) so `App.tsx` remains the source of truth and the
  topbar switcher stays in sync.
- `ProfileForm` gains small optional props so its footer can host Export/Delete
  and render inline (e.g. `embedded?: boolean`, `onDeleted?`, `extraActions?`).
  All existing validation, connection test, reuse-token, and cache-clear-warning
  logic is reused unchanged.
- `App.tsx`: replace the `Menu` with the ⚙ button + `showProfiles` state; keep
  the existing `editingProfile` / `handleCreated` wiring, now driven from the
  modal.

## Testing

- The `DeleteProfile` backend path already has coverage; add a focused Go test
  only if a gap is found.
- New UI verified via `npm run build` (tsc + vite) and `wails build`, plus a
  demo-profile click-through: open modal, switch selection, set default via star,
  edit + save, export, delete (active, default, and last-profile cases).

## Decisions locked

- Sequencing: design the modal first; **delete ships as part of the modal** (not
  a standalone dropdown item).
- Edit flow: **master-detail** (list left, form right) — chosen over
  list-then-swap and stacked-modal.
- Set-default mechanism: **star toggle in the list row**.
- Export: **per-profile button in the detail footer** (not a row kebab).
