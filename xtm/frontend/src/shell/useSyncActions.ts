import { SyncProfile, SyncProfileFull, CreateFolder, RenameFolder, DeleteFolder, SetShowCoverage, SyncTests, errMsg } from "../api";
import { TOUR_VERSION } from "../tour/steps";
import type { AppState } from "./useAppState";

// useSyncActions holds the Test Repository folder actions, the three sync
// entry points with their shared gate, and the Coverage tab toggle.
export function useSyncActions(s: AppState) {
  const { activeId, showCoverage, setShowCoverage, prompt, confirm, notice, syncStatus, canSync, beginSync, failSync, endSync, view, setView, selectedFolder, setSelectedFolder, startTour, tourSeenVersion, setDetailVersion, reloadPending, refreshProfileData } = s;

  // --- Test Repository folder management (FR-13.3) ---

  async function createFolder(parentPath: string) {
    if (!activeId) return;
    const name = await prompt({
      title: parentPath
        ? `New subfolder under ${parentPath}`
        : "New top-level folder",
      placeholder: "Folder name",
      submitLabel: "Create",
    });
    if (!name || !name.trim()) return;
    try {
      await CreateFolder(activeId, parentPath, name.trim());
      refreshProfileData();
      reloadPending();
    } catch (e) {
      await notice({
        title: "Create folder failed",
        message: errMsg(e),
        tone: "error",
      });
    }
  }

  async function renameFolder(path: string, currentName: string) {
    if (!activeId) return;
    const name = await prompt({
      title: `Rename folder "${currentName}"`,
      defaultValue: currentName,
      submitLabel: "Rename",
    });
    if (name === null || !name.trim() || name.trim() === currentName) return;
    try {
      await RenameFolder(activeId, path, name.trim());
      if (selectedFolder === path || selectedFolder.startsWith(path + "/")) {
        setSelectedFolder("");
      }
      refreshProfileData();
      reloadPending();
    } catch (e) {
      await notice({
        title: "Rename failed",
        message: errMsg(e),
        tone: "error",
      });
    }
  }

  async function deleteFolder(path: string) {
    if (!activeId) return;
    if (
      !(await confirm({
        title: "Delete folder",
        message: `Delete folder "${path}"? It must be empty.`,
        confirmLabel: "Delete",
        danger: true,
      }))
    )
      return;
    try {
      await DeleteFolder(activeId, path);
      if (selectedFolder === path) setSelectedFolder("");
      refreshProfileData();
      reloadPending();
    } catch (e) {
      await notice({
        title: "Delete failed",
        message: errMsg(e),
        tone: "error",
      });
    }
  }

  // Shared gate for the three sync entry points (SyncContext's canSync is the
  // single source of truth now). Returns false — with a "commit in progress"
  // notice when a commit is what's blocking — if a sync may not start. A sync
  // already running is a silent no-op, matching the pre-refactor behaviour.
  async function ensureCanSync(): Promise<boolean> {
    if (!activeId) return false;
    if (!canSync) {
      if (syncStatus === "committing") {
        await notice({
          title: "Commit in progress",
          message:
            "A commit is still running. Please wait for it to finish before syncing.",
          tone: "info",
        });
      }
      return false;
    }
    return true;
  }

  async function doSync(full: boolean) {
    if (!(await ensureCanSync())) return;
    beginSync({
      initialProgress: { phase: "", fetched: 0, total: 0, done: false },
      clearError: true,
    });
    try {
      await (full ? SyncProfileFull(activeId) : SyncProfile(activeId));
      refreshProfileData();
      setDetailVersion((v) => v + 1);
      // Start the tour on the FIRST successful sync rather than at launch:
      // before a sync the grid is empty, so half the steps would spotlight
      // elements with no data behind them and teach nothing (-335).
      if (tourSeenVersion < TOUR_VERSION)
        startTour("browse", () => setView("browse"));
    } catch (e) {
      failSync(errMsg(e));
    } finally {
      endSync();
    }
  }

  function runSync() {
    doSync(false);
  }

  // runFullSync forces a full re-pull, ignoring the incremental watermark, so
  // the Test Repository folder membership (skipped on routine resyncs) is
  // refreshed. It can be slow on large projects, so confirm first.
  async function runFullSync() {
    if (!(await ensureCanSync())) return;
    if (
      !(await confirm({
        title: "Full resync",
        message:
          "A full resync re-pulls every test and re-maps Test Repository folders. " +
          "It can take a while on large projects. Continue?",
        confirmLabel: "Continue",
        danger: false,
      }))
    ) {
      return;
    }
    doSync(true);
  }

  // syncTests does a targeted pull of test cases and folder membership, giving
  // the Browse view a quick refresh without running the full sync pipeline
  // (RND_P_4TFINT_05-260). Unlike doSync it shows no initial bar and leaves any
  // prior error banner in place (it reports failures via a toast instead).
  async function syncTests() {
    if (!(await ensureCanSync())) return;
    beginSync();
    try {
      await SyncTests(activeId);
      refreshProfileData();
    } catch (e) {
      await notice({ title: "Sync failed", message: errMsg(e), tone: "error" });
    } finally {
      endSync();
    }
  }

  // toggleCoverage shows/hides the opt-in Coverage tab and persists it. Leaving
  // the Coverage view when hiding so a hidden view isn't left on screen.
  async function toggleCoverage() {
    const next = !showCoverage;
    setShowCoverage(next);
    if (!next && view === "coverage") setView("browse");
    try {
      await SetShowCoverage(next);
    } catch (e) {
      console.error("set show coverage:", errMsg(e));
    }
  }

  return { createFolder, renameFolder, deleteFolder, ensureCanSync, doSync, runSync, runFullSync, syncTests, toggleCoverage };
}
