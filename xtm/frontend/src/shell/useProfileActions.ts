import { ExportProfile, ImportProfile, UpdateProfileToken, DeleteProfile, errMsg } from "../api";
import type { Profile } from "../api";
import { invalidateCapabilities } from "../features";
import type { AppState } from "./useAppState";

// useProfileActions holds profile import, export, delete and save, the
// after-edit refreshes, the New Test panel, and the shared ritual a bulk
// dialog runs when it finishes.
export function useProfileActions(s: AppState) {
  const { profiles, setProfiles, activeId, setActiveId, defaultProfileId, setDefaultProfileId, prompt, notice, isOpen, closeModal, setShowNewTest, setNewTestFolder, setSelectedKey, setSelectedSet, setEditingProfile, setDetailVersion, reloadPending, refreshProfileData } = s;

  // afterMutation is the shared "a modal finished mutating" ritual (spec §6.2):
  // refresh the profile lists, re-seed the open detail panel, reload the pending
  // journal, optionally drop the bulk selection, and close the active modal. It
  // replaces the block copy-pasted into every bulk modal's onComplete/onCancel.
  // (The import modal and the review/requirements cancels keep their own shapes
  // — their rituals genuinely differ.)
  function afterMutation(opts?: { clearSelection?: boolean }) {
    refreshProfileData();
    setDetailVersion((v) => v + 1);
    reloadPending();
    if (opts?.clearSelection) setSelectedSet(new Set());
    closeModal();
  }

  // exportProfile writes a profile's config (no credential) to a file the user
  // picks (FR-5.5).
  async function exportProfile(id: string) {
    if (!id) return;
    try {
      const path = await ExportProfile(id);
      if (path) await notice({ title: "Profile exported", message: path });
    } catch (e) {
      await notice({
        title: "Export failed",
        message: errMsg(e),
        tone: "error",
      });
    }
  }

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
      await notice({
        title: "Import failed",
        message: errMsg(e),
        tone: "error",
      });
      return null;
    }
  }

  // deleteProfile removes a profile (its token + cached data are purged by the
  // backend). The Manage Profiles modal confirms first. If the active profile is
  // deleted, switches to the default (if still set) or the first remaining
  // profile. (FR-5.3)
  async function deleteProfile(id: string) {
    if (!profiles.some((p) => p.id === id)) return;
    try {
      await DeleteProfile(id);
    } catch (e) {
      await notice({
        title: "Delete failed",
        message: errMsg(e),
        tone: "error",
      });
      return;
    }
    const remaining = profiles.filter((p) => p.id !== id);
    setProfiles(remaining);
    if (defaultProfileId === id) setDefaultProfileId("");
    if (activeId === id) {
      const next =
        defaultProfileId && defaultProfileId !== id
          ? defaultProfileId
          : (remaining[0]?.id ?? "");
      setActiveId(next);
      setSelectedKey(null);
      refreshProfileData();
      reloadPending();
    }
    if (remaining.length === 0) closeModal();
  }

  // handleCreated handles both a newly-created profile and an edited one: it
  // replaces the existing entry when the id is already known, otherwise appends.
  // After an edit, the cached data may have been cleared (project/URL change),
  // so the views are refreshed. Also drops any cached capabilities for this
  // id -- an edit may have flipped the profile's Backend (Xray<->Kiwi), and
  // without this the UI would keep gating on the old backend's capabilities
  // until an app restart (useCapabilities is keyed on profileId, which is
  // unchanged across an edit).
  function handleCreated(p: Profile) {
    invalidateCapabilities(p.id);
    setProfiles((prev) =>
      prev.some((x) => x.id === p.id)
        ? prev.map((x) => (x.id === p.id ? p : x))
        : [...prev, p],
    );
    setActiveId(p.id);
    // handleCreated is wired to both the form modal (onCreated) and the
    // Profiles manager (onSaved). Close only the form — the old code did
    // setShowForm(false), a no-op when the manager (or nothing) was open, so
    // saving from the manager must leave it open.
    if (isOpen("form")) closeModal();
    setEditingProfile(null);
    setSelectedKey(null);
    refreshProfileData();
    setDetailVersion((v) => v + 1);
    reloadPending();
  }

  // Called by TestDetail after a successful inline edit. Refreshes the
  // grid (so it shows the new value) and the pending list. Deliberately
  // does NOT bump detailVersion — TestDetail already has the new value in
  // its own local state, and re-fetching mid-edit would risk clobbering
  // a field the user is still typing in.
  function handleEdited() {
    refreshProfileData();
    reloadPending();
  }

  // openNewTest launches the create panel, pre-filling a folder when one is
  // passed (per-folder action) or when a folder is the active browse filter.
  function openNewTest(folderId: string) {
    setSelectedKey(null);
    setNewTestFolder(folderId);
    setShowNewTest(true);
  }

  // handleTestCreated closes the panel, refreshes pending + list, and opens the
  // freshly-created (still uncommitted) Test in the detail panel.
  function handleTestCreated(tempKey: string) {
    setShowNewTest(false);
    setSelectedKey(tempKey);
    handleEdited(); // refreshes the pending list and the table
  }

  return { afterMutation, exportProfile, importProfile, deleteProfile, handleCreated, handleEdited, openNewTest, handleTestCreated };
}
