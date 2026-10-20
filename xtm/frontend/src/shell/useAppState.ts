import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Health, SetTourSeenVersion, errMsg, isDemoUrl, demoVariant } from "../api";
import type { HealthInfo, Profile, PendingChange, CommitResult } from "../api";
import { useCapabilities } from "../features";
import { clearViewState } from "../lib/viewState";
import { useProfile } from "../contexts/ProfileContext";
import { useSync } from "../contexts/SyncContext";
import { useSelection } from "../contexts/SelectionContext";
import { useNav } from "../contexts/NavContext";
import { useModal } from "../contexts/ModalContext";
import { usePendingChanges } from "../queries/pending";
import { useSyncState, useFolders, useComponents, useGroupContainers } from "../queries/app";
import { invalidateProfileData } from "../queries/invalidate";
import { keys } from "../queries/keys";
import { visibleViews } from "../views";
import { usePrompt } from "../components/usePrompt";
import { useConfirm } from "../components/useConfirm";
import { useNotice } from "../components/useNotice";
import { useTour } from "@agile-suite/core";
import { TOURS, TOUR_VERSION } from "../tour/steps";

// useAppState gathers what the shell knows: the profile, sync, nav, selection
// and modal contexts, the profile-scoped queries, the backend health check,
// the sidebar width, the pending journal indexed by test, and the views the
// active backend supports. The reset effects that tie the contexts together
// live here too.
export function useAppState() {
  const [health, setHealth] = useState<HealthInfo | null>(null);

  // Active-workspace state now lives in ProfileContext (spec §5.1). setTheme /
  // setDefault keep their former App-local names (chooseTheme / setDefaultFor)
  // so existing call sites are unchanged.
  const {
    profiles,
    setProfiles,
    activeId,
    setActiveId,
    defaultProfileId,
    setDefaultProfileId,
    theme,
    showCoverage,
    setShowCoverage,
    showNavRail: navRail,
    setShowNavRail: setNavRail,
    loadingProfiles,
    activeProfile,
    setTheme: chooseTheme,
    setDefault: setDefaultFor,
    reloadProfiles,
  } = useProfile();
  // Which onboarding tour version this user has already been through.
  // TOUR_VERSION means "seen"; anything lower means it is still owed.
  const [tourSeenVersion, setTourSeenVersion] = useState(TOUR_VERSION);
  const { prompt } = usePrompt();
  const { confirm } = useConfirm();
  const { notice } = useNotice();
  // The ~16 modal-visibility booleans collapse to one reducer in ModalContext
  // (spec §5.5); isOpen/openModal/closeModal replace the showX/setShowX pairs.
  const { isOpen, openModal, closeModal } = useModal();
  // editingProfile stays App-local — it's the form modal's edit target (FR-5),
  // meaningful only while the "form" modal is open.
  const [editingProfile, setEditingProfile] = useState<Profile | null>(null);

  // The sync/commit lifecycle + its mutual-exclusion invariant now live in
  // SyncContext (spec §5.2). `pulling` is the old early-release `syncing` flag
  // (Sync button); status/can* replace the syncRunningRef + committing guards.
  const {
    status: syncStatus,
    pulling,
    progress,
    syncError,
    canSync,
    canCommit,
    canSwitchProfile,
    beginSync,
    failSync,
    endSync,
    beginCommit,
    endCommit,
  } = useSync();
  const prevProfileRef = useRef<string>("");

  // View routing + browse grouping + the New Test panel now live in NavContext
  // (spec §5.4). The cross-context reset effects (clearing the sidebar + bulk
  // selection on profile / group-by change) stay in App — they coordinate Nav
  // with SelectionContext.
  const {
    view,
    setView,
    groupBy,
    setGroupBy,
    selectedFolder,
    setSelectedFolder,
    selectedContainer,
    setSelectedContainer,
    selectedComponent,
    setSelectedComponent,
    showNewTest,
    setShowNewTest,
    newTestFolder,
    setNewTestFolder,
    openContainer,
  } = useNav();

  // Browse selection lives in SelectionContext (spec §5.3); TestTable now reads
  // the toggle actions directly from it. App keeps the value + raw setters for
  // its composite handlers (profile-change reset, applyCreatedRemap remap, the
  // bulk modals' testKeys snapshot + onComplete clears, the open detail row).
  const { selectedKey, setSelectedKey, selectedSet, setSelectedSet } =
    useSelection();
  const [detailVersion, setDetailVersion] = useState(0);

  const queryClient = useQueryClient();
  const pendingQuery = usePendingChanges(activeId);
  const pendingChanges = pendingQuery.data ?? [];
  // App-shell profile-scoped loads (Phase 4b), refreshed by invalidateProfileData.
  const syncState = useSyncState(activeId).data ?? null;
  const folders = useFolders(activeId).data ?? [];
  const groupContainers = useGroupContainers(activeId, groupBy).data ?? [];
  const components = useComponents(activeId, groupBy).data ?? [];
  const [lastCommitResult, setLastCommitResult] = useState<CommitResult | null>(
    null,
  );

  // The onboarding tour (-335). Steps target Browse-only elements and the tour
  // can be replayed from any view, so it switches to Browse before starting.
  const { start: startTour } = useTour({
    tours: TOURS,
    version: TOUR_VERSION,
    markSeen: SetTourSeenVersion,
    onFinish: () => setTourSeenVersion(TOUR_VERSION),
  });

  // Resizeable browse sidebar (FR-11): drag the divider to widen it for long
  // folder names / deep nesting; the width persists across sessions.
  const [sidebarWidth, setSidebarWidth] = useState<number>(() => {
    const saved = Number(localStorage.getItem("xtm.sidebarWidth"));
    return saved >= 160 && saved <= 640 ? saved : 240;
  });
  useEffect(() => {
    localStorage.setItem("xtm.sidebarWidth", String(sidebarWidth));
  }, [sidebarWidth]);

  function startSidebarResize(e: React.MouseEvent) {
    e.preventDefault();
    const startX = e.clientX;
    const startW = sidebarWidth;
    const onMove = (ev: MouseEvent) =>
      setSidebarWidth(
        Math.min(640, Math.max(160, startW + ev.clientX - startX)),
      );
    const onUp = () => {
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
    };
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
  }

  // First: check whether the backend started up cleanly. Startup can take a
  // moment (e.g. a slow first-run migration), during which Health() reports
  // ok:false with an EMPTY error because the service layer isn't wired yet.
  // Treat that (and a Health() call that throws because the runtime isn't
  // ready) as "still starting" and retry, so a slow start doesn't render a
  // false "Backend failed to start". A non-empty error, or ok:true, is
  // definitive and stops the polling.
  useEffect(() => {
    let cancelled = false;
    let tries = 0;
    const maxTries = 40; // ~10s at 250ms
    const check = () => {
      Health()
        .then((h) => {
          if (cancelled) return;
          if (!h.ok && !h.error && tries < maxTries) {
            tries++;
            setTimeout(check, 250);
            return;
          }
          setHealth(h);
        })
        .catch((e) => {
          if (cancelled) return;
          if (tries < maxTries) {
            tries++;
            setTimeout(check, 250);
            return;
          }
          setHealth({
            ok: false,
            error: `Health check itself failed: ${errMsg(e)}`,
            dbPath: "",
            logPath: "",
          });
        });
    };
    check();
    return () => {
      cancelled = true;
    };
  }, []);

  // Load profiles once the backend reports healthy. ProfileContext owns the
  // profile/settings state; App keeps only the tour version (onboarding state)
  // from the returned Settings.
  useEffect(() => {
    if (!health || !health.ok) return;
    reloadProfiles().then((s) => {
      if (s) setTourSeenVersion(s.tourSeenVersion ?? 0);
    });
  }, [health, reloadProfiles]);

  // The sync:progress and spellcheck:progress subscriptions moved into
  // SyncProvider (spec §5.2); the reducer there owns the button-release and
  // status-bar behaviour they used to drive.

  // Pending changes grouped by parent Test key — drives the dirty markers
  // in the grid and the per-field dot in the detail panel. test_step rows
  // are bucketed under their parent Test so the row dot lights up for step
  // edits too, and the detail panel can render per-step dirty markers.
  const pendingByTestKey = useMemo(() => {
    const m = new Map<string, PendingChange[]>();
    for (const p of pendingChanges) {
      let testKey: string | null = null;
      if (
        p.entityType === "test_case" ||
        p.entityType === "test_step_order" ||
        p.entityType === "precondition_set" ||
        p.entityType === "requirement_set"
      ) {
        // Test-level changes keyed by the bare Test key.
        testKey = p.entityKey;
      } else if (
        p.entityType.startsWith("test_step") ||
        p.entityType === "custom_field"
      ) {
        // test_step* and custom_field all key as "<testKey>:<suffix>" — bucket
        // them under the parent Test so the grid + detail dirty markers cover
        // step edits and custom field edits alike.
        const colon = p.entityKey.indexOf(":");
        if (colon > 0) testKey = p.entityKey.substring(0, colon);
      }
      if (!testKey) continue;
      const arr = m.get(testKey);
      if (arr) arr.push(p);
      else m.set(testKey, [p]);
    }
    return m;
  }, [pendingQuery.data]);

  // reloadPending invalidates the pending query rather than manually refetching,
  // so the ~35 call sites stay a one-liner. The query loads on mount and on
  // activeId change, so no dedicated load effect is needed.
  const reloadPending = useCallback(() => {
    if (activeId) {
      queryClient.invalidateQueries({ queryKey: keys.pending(activeId) });
    }
  }, [queryClient, activeId]);

  // refreshProfileData is the single "refresh all profile lists" signal that
  // mutation handlers call: it invalidates every profile-scoped query family so
  // the affected lists refetch (Phase 4c — the legacy refreshKey counter is gone).
  const refreshProfileData = useCallback(() => {
    invalidateProfileData(queryClient, activeId);
  }, [queryClient, activeId]);

  // Refresh the sync summary and folder tree when the active profile changes
  // or a sync finishes.
  // Clear folder + container + component + row selection when the profile changes.
  // Also drop the previous profile's view session state so it isn't inherited.
  useEffect(() => {
    if (prevProfileRef.current && prevProfileRef.current !== activeId) {
      clearViewState(prevProfileRef.current);
    }
    prevProfileRef.current = activeId;
    setSelectedFolder("");
    setSelectedContainer("");
    setSelectedComponent("");
    setSelectedSet(new Set());
  }, [activeId]);

  // The group-by sidebar lists (containers / components) now load via
  // useGroupContainers / useComponents (Phase 4b). These effects keep only the
  // side effect the queries can't express: resetting the chosen container /
  // component whenever the dimension or profile changes, so the grid doesn't
  // keep filtering by a now-hidden key.
  useEffect(() => {
    setSelectedContainer("");
  }, [activeId, groupBy]);

  useEffect(() => {
    setSelectedComponent("");
  }, [activeId, groupBy]);

  const isDemo = isDemoUrl(activeProfile?.jiraUrl);
  const demoVar = demoVariant(activeProfile?.jiraUrl);
  // Gates the Xray-shaped UI below to what the active profile's backend
  // actually supports (Kiwi, etc.) — defaultCapabilities (all true) while
  // loading, so an Xray profile is never affected (P6.2a).
  const caps = useCapabilities(activeId);

  // The tab bar and the navigation rail render this same list, so a view can
  // never appear in one and not the other (#171).
  const shownViews = useMemo(
    () =>
      visibleViews({
        supportsPreconditionObjects: caps.supportsPreconditionObjects,
        supportsRequirementObjects: caps.supportsRequirementObjects,
        supportsComponentAdmin: caps.supportsComponentAdmin,
        supportsDatasets: caps.supportsDatasets,
        showCoverage,
      }),
    [caps, showCoverage],
  );

  // A sync is in flight when the main Sync is running (syncing) OR any partial
  // per-view refresh is emitting progress. Both a full pull and a partial sync
  // write to the store keyed by the active profile, so switching profiles while
  // either runs would race the in-flight writes and land stale progress events
  // on the newly-selected profile. SyncContext.canSwitchProfile is the single
  // source of truth for locking the profile switcher (false during a sync + its
  // tail, and during a spellcheck scan; a commit does not lock it).
  const syncActive = !canSwitchProfile;

  return { profiles, setProfiles, activeId, setActiveId, defaultProfileId, setDefaultProfileId, theme, showCoverage, setShowCoverage, navRail, setNavRail, loadingProfiles, activeProfile, chooseTheme, setDefaultFor, reloadProfiles, prompt, confirm, notice, isOpen, openModal, closeModal, syncStatus, pulling, progress, syncError, canSync, canCommit, canSwitchProfile, beginSync, failSync, endSync, beginCommit, endCommit, view, setView, groupBy, setGroupBy, selectedFolder, setSelectedFolder, selectedContainer, setSelectedContainer, selectedComponent, setSelectedComponent, showNewTest, setShowNewTest, newTestFolder, setNewTestFolder, openContainer, selectedKey, setSelectedKey, selectedSet, setSelectedSet, startTour, health, setHealth, tourSeenVersion, setTourSeenVersion, editingProfile, setEditingProfile, detailVersion, setDetailVersion, lastCommitResult, setLastCommitResult, sidebarWidth, setSidebarWidth, prevProfileRef, queryClient, pendingQuery, pendingChanges, syncState, folders, groupContainers, components, pendingByTestKey, reloadPending, refreshProfileData, isDemo, demoVar, caps, shownViews, syncActive, startSidebarResize };
}

export type AppState = ReturnType<typeof useAppState>;
