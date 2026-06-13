# Manage Profiles Modal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a master-detail "Manage Profiles" modal that lists every profile, edits the selected one via the existing `ProfileForm`, and adds delete-profile as a first-class per-profile action; the topbar keeps only the switcher plus a ⚙ Manage button.

**Architecture:** A new presentational `ProfilesModal` component holds list-selection state and embeds the existing `ProfileForm` in its detail pane. `App.tsx` stays the source of truth for `profiles` / `activeId` / `defaultProfileId` and passes callbacks down (set-default, export, import, save, delete). The backend `DeleteProfile`/`ExportProfile`/`ImportProfile`/`SetDefaultProfile` bindings already exist and need no change.

**Tech Stack:** React 18 + TypeScript, Wails v2 generated bindings (`../api`), Vite. **Note:** this frontend has **no JS test runner** (only Go has tests). Per-task verification is `npx tsc --noEmit`; final verification is `npm run build` (tsc + vite) + `wails build` + a demo-profile click-through. The Go `DeleteProfile` path is already covered by `internal/profile` tests — no new Go test needed.

---

## File Structure

- **Create** `frontend/src/components/ProfilesModal.tsx` — the master-detail modal (list + embedded `ProfileForm`). One responsibility: present and orchestrate profile management; all persistence goes through props.
- **Modify** `frontend/src/components/ProfileForm.tsx` — add one optional `extraActions` slot rendered in the footer (for Export/Delete). No other change.
- **Modify** `frontend/src/App.tsx` — add `DeleteProfile` import; add `showProfiles` state, `handleSetDefaultFor`, `handleExportProfile`, `handleDeleteProfile`; make `importProfile` return the created profile; replace the topbar Profile `Menu` with a ⚙ Manage button; render `<ProfilesModal/>`; remove the four now-superseded handlers.
- **Modify** `frontend/src/App.css` — styles for the modal’s two-pane layout, list rows, star toggle, and the form’s extra-actions group.

---

## Task 1: Add an `extraActions` slot to ProfileForm

**Files:**
- Modify: `frontend/src/components/ProfileForm.tsx`

- [ ] **Step 1: Add the prop to the Props interface**

In `frontend/src/components/ProfileForm.tsx`, change the `Props` interface (lines 11–18) to add `extraActions`:

```tsx
import { useState, type ReactNode } from "react";
```

```tsx
interface Props {
  onCreated: (p: Profile) => void;
  onCancel?: () => void;
  // When set, the form edits this profile instead of creating a new one (FR-5).
  profile?: Profile;
  // Existing profiles — drives the "reuse token" option when creating (FR-5).
  profiles?: Profile[];
  // Optional extra footer controls (e.g. Export / Delete) rendered left of
  // Cancel / Save. Used by the Manage Profiles modal.
  extraActions?: ReactNode;
}
```

(The first line replaces the existing `import { useState } from "react";` at line 1.)

- [ ] **Step 2: Destructure the new prop**

Change the function signature (line 66):

```tsx
export function ProfileForm({ onCreated, onCancel, profile, profiles, extraActions }: Props) {
```

- [ ] **Step 3: Render the slot in the footer**

Replace the final actions block (lines 253–266) with:

```tsx
      <div className="form-actions form-actions-end">
        {extraActions && <div className="profile-form-extra">{extraActions}</div>}
        {onCancel && (
          <button className="btn" onClick={onCancel} disabled={saving}>
            Cancel
          </button>
        )}
        <button
          className="btn btn-primary"
          onClick={save}
          disabled={!canSave || saving}
        >
          {saving ? "Saving…" : isEdit ? "Save changes" : "Create profile"}
        </button>
      </div>
```

- [ ] **Step 4: Typecheck**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors (an unused optional prop is fine).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/ProfileForm.tsx
git commit -m "ProfileForm: add optional extraActions footer slot"
```

---

## Task 2: Create the ProfilesModal component

**Files:**
- Create: `frontend/src/components/ProfilesModal.tsx`

- [ ] **Step 1: Write the component**

Create `frontend/src/components/ProfilesModal.tsx` with exactly:

```tsx
import { useEffect, useState } from "react";
import type { Profile } from "../api";
import { ProfileForm } from "./ProfileForm";

interface Props {
  profiles: Profile[];
  activeId: string;
  defaultProfileId: string;
  onClose: () => void;
  // Toggle the launch-default for a profile (clears it if already default).
  onSetDefault: (id: string) => void;
  // Export a profile's config (no credential) to a file.
  onExport: (id: string) => void;
  // Import a profile from a file; resolves to the created profile or null.
  onImport: () => Promise<Profile | null>;
  // Persist a created/edited profile (replace-or-append + refresh in App).
  onSaved: (p: Profile) => void;
  // Delete a profile after confirming; resolves true if it was deleted.
  onDelete: (id: string) => Promise<boolean>;
}

// ProfilesModal is the master-detail profile manager: a list of every profile
// on the left (with a star to set the launch default) and the selected
// profile's ProfileForm on the right. New / Import create or bring in profiles;
// Export / Delete act on the selected one. App owns the profile state, so every
// mutation flows through the callback props.
export function ProfilesModal({
  profiles,
  activeId,
  defaultProfileId,
  onClose,
  onSetDefault,
  onExport,
  onImport,
  onSaved,
  onDelete,
}: Props) {
  const [selectedId, setSelectedId] = useState(
    activeId || profiles[0]?.id || "",
  );
  const [creating, setCreating] = useState(false);

  // Keep the selection valid: if the selected profile is deleted (and we're not
  // mid-create), fall back to the active profile, else the first one.
  useEffect(() => {
    if (creating) return;
    if (!profiles.some((p) => p.id === selectedId)) {
      setSelectedId(activeId || profiles[0]?.id || "");
    }
  }, [profiles, selectedId, creating, activeId]);

  const selected = profiles.find((p) => p.id === selectedId) ?? null;

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div
        className="modal profiles-modal"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="profiles-modal-head">
          <h2>Manage Profiles</h2>
          <button className="btn btn-ghost" onClick={onClose} title="Close">
            ✕
          </button>
        </div>

        <div className="profiles-modal-body">
          <div className="profiles-list">
            <ul className="profiles-list-items">
              {profiles.map((p) => (
                <li
                  key={p.id}
                  className={`profiles-list-row${
                    !creating && p.id === selectedId
                      ? " profiles-list-row-selected"
                      : ""
                  }`}
                  onClick={() => {
                    setCreating(false);
                    setSelectedId(p.id);
                  }}
                >
                  <button
                    className="profiles-star"
                    title={
                      defaultProfileId === p.id
                        ? "Default on launch — click to clear"
                        : "Set as default on launch"
                    }
                    onClick={(e) => {
                      e.stopPropagation();
                      onSetDefault(p.id);
                    }}
                  >
                    {defaultProfileId === p.id ? "★" : "☆"}
                  </button>
                  <span className="profiles-row-name">{p.name}</span>
                  <span className="profiles-row-key muted">
                    ({p.projectKey})
                  </span>
                </li>
              ))}
            </ul>
            <div className="profiles-list-footer">
              <button
                className="btn"
                onClick={() => setCreating(true)}
                title="Create a new profile"
              >
                + New
              </button>
              <button
                className="btn"
                onClick={async () => {
                  const p = await onImport();
                  if (p) {
                    setCreating(false);
                    setSelectedId(p.id);
                  }
                }}
                title="Import a profile from a file"
              >
                Import…
              </button>
            </div>
          </div>

          <div className="profiles-detail">
            {creating ? (
              <ProfileForm
                profiles={profiles}
                onCreated={(p) => {
                  onSaved(p);
                  setCreating(false);
                  setSelectedId(p.id);
                }}
                onCancel={() => setCreating(false)}
              />
            ) : selected ? (
              <ProfileForm
                key={selected.id}
                profile={selected}
                profiles={profiles}
                onCreated={(p) => {
                  onSaved(p);
                  setSelectedId(p.id);
                }}
                extraActions={
                  <>
                    <button
                      className="btn"
                      onClick={() => onExport(selected.id)}
                      title="Export this profile (without its token)"
                    >
                      Export
                    </button>
                    <button
                      className="btn btn-danger"
                      onClick={() => onDelete(selected.id)}
                      title="Delete this profile"
                    >
                      Delete
                    </button>
                  </>
                }
              />
            ) : (
              <p className="muted">Select a profile, or create a new one.</p>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Typecheck**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors. (The component is not rendered yet; it just has to compile.)

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/ProfilesModal.tsx
git commit -m "Add ProfilesModal master-detail component"
```

---

## Task 3: Wire ProfilesModal into App and remove the Profile dropdown

**Files:**
- Modify: `frontend/src/App.tsx`

- [ ] **Step 1: Import DeleteProfile and ProfilesModal**

In the `../api` import list in `frontend/src/App.tsx`, add `DeleteProfile` next to the other profile bindings (it is exported from `api.ts` but not yet imported here). Then add the component import near the other component imports (next to `import { ProfileForm } from "./components/ProfileForm";` at line 46):

```tsx
import { ProfilesModal } from "./components/ProfilesModal";
```

- [ ] **Step 2: Add the showProfiles state**

Next to `const [showForm, setShowForm] = useState(false);` (line 93), add:

```tsx
  const [showProfiles, setShowProfiles] = useState(false);
```

- [ ] **Step 3: Parameterize exportProfile by id**

Replace the existing `exportProfile` function (lines 548–558) with an id-taking version:

```tsx
  // exportProfile writes a profile's config (no credential) to a file the user
  // picks (FR-5.5).
  async function exportProfile(id: string) {
    if (!id) return;
    try {
      const path = await ExportProfile(id);
      if (path) window.alert(`Profile exported to:\n${path}`);
    } catch (e) {
      window.alert(`Export failed: ${errMsg(e)}`);
    }
  }
```

- [ ] **Step 4: Make importProfile return the created profile**

Replace the existing `importProfile` function (lines 562–581) with a version that returns `Profile | null` so the modal can select the imported profile:

```tsx
  // importProfile creates a profile from a chosen config file, then prompts for
  // its PAT (the credential isn't part of the exported file) (FR-5.5). Returns
  // the created profile, or null if cancelled.
  async function importProfile(): Promise<Profile | null> {
    try {
      const p = await ImportProfile();
      if (!p.id) return null; // cancelled
      setProfiles((prev) => [...prev, p]);
      setActiveId(p.id);
      setSelectedKey(null);
      const token = await prompt({
        title: `Enter the Personal Access Token for "${p.name}"`,
        placeholder: "Paste token (or cancel to set it later)",
        password: true,
        submitLabel: "Save token",
      });
      if (token && token.trim()) {
        await UpdateProfileToken(p.id, token.trim());
      }
      return p;
    } catch (e) {
      window.alert(`Import failed: ${errMsg(e)}`);
      return null;
    }
  }
```

- [ ] **Step 5: Add set-default-by-id and delete handlers**

Immediately after `importProfile` (which now ends around the same area), add two handlers:

```tsx
  // setDefaultFor toggles the launch-default for a specific profile (clears it
  // if it's already the default), used by the Manage Profiles modal.
  async function setDefaultFor(id: string) {
    const next = defaultProfileId === id ? "" : id;
    try {
      await SetDefaultProfile(next);
      setDefaultProfileId(next);
    } catch (e) {
      console.error("set default profile:", errMsg(e));
    }
  }

  // deleteProfile confirms, then removes a profile (its token + cached data are
  // purged by the backend). If the active profile is deleted, switches to the
  // default (if still set) or the first remaining profile. Resolves true when a
  // delete happened. (FR-5.3)
  async function deleteProfile(id: string): Promise<boolean> {
    const target = profiles.find((p) => p.id === id);
    if (!target) return false;
    if (
      !window.confirm(
        `Delete profile "${target.name}"? This removes its stored token and all ` +
          `cached test data. This cannot be undone.`,
      )
    ) {
      return false;
    }
    try {
      await DeleteProfile(id);
    } catch (e) {
      window.alert(`Delete failed: ${errMsg(e)}`);
      return false;
    }
    const remaining = profiles.filter((p) => p.id !== id);
    setProfiles(remaining);
    if (defaultProfileId === id) setDefaultProfileId("");
    if (activeId === id) {
      const next =
        defaultProfileId && defaultProfileId !== id
          ? defaultProfileId
          : remaining[0]?.id ?? "";
      setActiveId(next);
      setSelectedKey(null);
      setRefreshKey((k) => k + 1);
      reloadPending();
    }
    if (remaining.length === 0) setShowProfiles(false);
    return true;
  }
```

- [ ] **Step 6: Remove the four superseded handlers**

Delete these now-unused functions (all were only called by the Profile dropdown being removed): `toggleDefault` (lines ~524–535), `setToken` (lines ~583–600), and the `editScope` and `editActiveProfile` handlers (search for `function editScope` and `function editActiveProfile`). Their capabilities now live in the modal (`setDefaultFor`) and in `ProfileForm` (token + scope are form fields).

Before deleting each, confirm it has no other caller:

Run: `cd frontend && grep -n "toggleDefault\|editScope\|editActiveProfile\|\bsetToken\b" src/App.tsx`
Expected: after removing the Profile `Menu` (next step) the only matches are the definitions themselves. If any other caller exists, keep that function.

- [ ] **Step 7: Replace the Profile dropdown Menu with a Manage button**

Replace the entire `<Menu label="Profile" … />` block (lines 908–961) with:

```tsx
          <button
            className="btn topbar-manage"
            onClick={() => setShowProfiles(true)}
            title="Manage profiles — add, edit, set default, export, delete"
          >
            ⚙ Manage
          </button>
```

(Leave the `<select className="profile-select">` switcher above it unchanged. Leave the `Menu` import — it is still used by other menus.)

- [ ] **Step 8: Render ProfilesModal**

Next to the existing `{showForm && ( … )}` block (around line 1333), add:

```tsx
      {showProfiles && (
        <ProfilesModal
          profiles={profiles}
          activeId={activeId}
          defaultProfileId={defaultProfileId}
          onClose={() => setShowProfiles(false)}
          onSetDefault={setDefaultFor}
          onExport={exportProfile}
          onImport={importProfile}
          onSaved={handleCreated}
          onDelete={deleteProfile}
        />
      )}
```

- [ ] **Step 9: Typecheck**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors. If tsc reports an unused symbol (e.g. a still-referenced removed handler), fix the reference per Step 6’s grep.

- [ ] **Step 10: Commit**

```bash
git add frontend/src/App.tsx
git commit -m "Wire ProfilesModal into topbar; add delete-profile; drop Profile dropdown"
```

---

## Task 4: Style the modal

**Files:**
- Modify: `frontend/src/App.css`

- [ ] **Step 1: Append the styles**

Add to the end of `frontend/src/App.css`:

```css
/* Manage Profiles modal */
.modal.profiles-modal {
  width: min(820px, 92vw);
  max-width: 820px;
}
.profiles-modal-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.profiles-modal-head h2 {
  margin: 0;
}
.profiles-modal-body {
  display: flex;
  gap: 16px;
  align-items: stretch;
}
.profiles-list {
  display: flex;
  flex-direction: column;
  width: 240px;
  flex: 0 0 240px;
  border-right: 1px solid var(--card-border, #2a3a4f);
  padding-right: 12px;
}
.profiles-list-items {
  list-style: none;
  margin: 0;
  padding: 0;
  flex: 1;
  overflow-y: auto;
  max-height: 60vh;
}
.profiles-list-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 8px;
  border-radius: 6px;
  cursor: pointer;
}
.profiles-list-row:hover {
  background: var(--hover-bg, rgba(127, 127, 127, 0.12));
}
.profiles-list-row-selected,
.profiles-list-row-selected:hover {
  background: var(--accent-soft, rgba(56, 132, 222, 0.18));
}
.profiles-row-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.profiles-row-key {
  font-size: 12px;
}
.profiles-star {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 15px;
  line-height: 1;
  padding: 0 2px;
  color: var(--accent, #f5b301);
}
.profiles-list-footer {
  display: flex;
  gap: 8px;
  padding-top: 10px;
}
.profiles-detail {
  flex: 1;
  min-width: 0;
}
.profile-form-extra {
  margin-right: auto;
  display: flex;
  gap: 8px;
}
```

- [ ] **Step 2: Typecheck (CSS is not type-checked, but confirm the build still compiles JS)**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/App.css
git commit -m "Style the Manage Profiles modal"
```

---

## Task 5: Full build + manual verification

**Files:** none (verification only)

- [ ] **Step 1: Production frontend build**

Run: `cd frontend && npm run build`
Expected: `tsc` clean, then `vite build` prints `✓ built` with no errors.

- [ ] **Step 2: Full Wails build**

Run (PowerShell): `cd C:\projects\xray-test-manager; $env:Path += ";$env:USERPROFILE\go\bin"; wails build`
Expected: `Built '…\xray-test-manager.exe'` with no compile errors. (The `KnownStructs` / `Not found: time.Time` stderr lines are normal binding-gen logging.)

- [ ] **Step 3: Manual click-through on a demo profile**

Launch `build\bin\xray-test-manager.exe`. Create or open a profile whose Jira URL is `demo`. Then verify each:
  1. Topbar shows the profile `<select>` + a `⚙ Manage` button; the old "Profile" dropdown is gone.
  2. Click **⚙ Manage** → modal opens with the profile list (left) and the selected profile’s form (right).
  3. Click a different row → its form loads on the right.
  4. Click a row’s **☆** → it becomes **★** and the others clear; click **★** again → default cleared. (Re-open the app later to confirm the launch default, optional.)
  5. **+ New** → blank create form; Cancel returns to the selected profile.
  6. Edit a field, **Save changes** → list reflects the new name; the topbar switcher updates.
  7. **Export** on a profile → save dialog appears; saving writes a `.json`.
  8. **Delete** a non-active profile → confirm dialog → it disappears from the list.
  9. **Delete the active profile** → app switches to the default (or first remaining) profile; views refresh.
  10. **Delete the last remaining profile** → modal closes and the onboarding create form appears.

- [ ] **Step 4: Commit (only if Step 3 surfaced fixes)**

If any manual fix was needed, commit it with a descriptive message. Otherwise nothing to commit — the feature is complete.

---

## Self-Review notes

- **Spec coverage:** topbar (Task 3 Step 7) · master-detail modal (Task 2) · embedded ProfileForm reuse (Task 1 + Task 2) · star-to-default (Task 2 + Task 3 Step 5) · per-profile Export & Delete (Task 2 + Task 3 Steps 3/5) · delete edge-cases active/default/last (Task 3 Step 5) · `+ New` / `Import…` (Task 2 + Task 3 Step 4) · styling (Task 4) · verification incl. all delete cases (Task 5 Step 3). All spec sections map to a task.
- **No new backend logic:** `DeleteProfile`, `ExportProfile`, `ImportProfile`, `SetDefaultProfile`, `UpdateProfileToken` bindings already exist and are exported from `api.ts`.
- **Type consistency:** `ProfilesModal` prop names (`onSetDefault`, `onExport`, `onImport`, `onSaved`, `onDelete`) match the App handlers wired in Task 3 Step 8 (`setDefaultFor`, `exportProfile`, `importProfile`, `handleCreated`, `deleteProfile`). `onImport` returns `Promise<Profile | null>` (Task 3 Step 4) matching the modal’s `await onImport()`. `extraActions: ReactNode` (Task 1) matches the JSX passed in Task 2.
