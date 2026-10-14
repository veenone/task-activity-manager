import { useEffect, useMemo, useState } from "react";
import { useProfile } from "../../contexts/ProfileContext";
import { useViewState } from "../../lib/viewState";
import { BrowserOpenURL, GetTestRunHistory, GetBugDetail, isDemoUrl } from "../../api";
import type { BugWithTests, TestRunEntry, Container, BugDetail } from "../../api";
import { usePrompt } from "../usePrompt";
import { useBugs, useBugTests } from "../../queries/bugs";
import { keyCompare, cmpStr, applyDir } from "../../sort";

export interface BugsPanelProps {
  jiraUrl: string;
  onOpenTest: (testKey: string) => void;
}

function cmpBug(a: BugWithTests, b: BugWithTests, field: string): number {
  switch (field) {
    case "status":
      return cmpStr(a.status, b.status) || keyCompare(a.key, b.key);
    case "project":
      return cmpStr(a.projectKey, b.projectKey) || keyCompare(a.key, b.key);
    case "priority":
      return cmpStr(a.priority, b.priority) || keyCompare(a.key, b.key);
    default:
      return keyCompare(a.key, b.key);
  }
}

// useBugsState holds what the Bugs panel knows: the bug list and the selected
// bug's tests, the filter, sort, page and checks, the detail pane's lazy bug
// detail and collapse state, the right sidebar, and each test row's run
// history. Effects keep the selection valid and reset state when it changes.
export function useBugsState(p: BugsPanelProps) {
  const { jiraUrl } = p;
  const { activeId: profileId } = useProfile();
  // The bug list + the selected bug's linked tests come from the query cache
  // with stable keys (Phase 4c); a mutation refreshes them via
  // invalidateProfileData, and a bugs-only sync refetches the list directly.
  const bugsQuery = useBugs(profileId);
  const bugs = bugsQuery.data ?? [];
  const [filter, setFilter] = useViewState(profileId, "bugs", "filter", "");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [selected, setSelected] = useViewState(profileId, "bugs", "selected", "");
  const bugTestsQuery = useBugTests(profileId, selected);
  const tests = bugTestsQuery.data ?? [];
  // Checked bugs for the bulk "Create Test Execution" action. This is kept
  // independent of `selected` (the detail-pane row): ticking a checkbox must
  // not change which bug is shown in the detail pane, and vice versa.
  const [checked, setChecked] = useState<Set<string>>(new Set());
  const [creating, setCreating] = useState(false);
  // State for "Add tests to existing execution" modal.
  const [addToExecOpen, setAddToExecOpen] = useState(false);
  const [addToExecLoading, setAddToExecLoading] = useState(false);
  const [addToExecExecs, setAddToExecExecs] = useState<Container[]>([]);
  const [addToExecTarget, setAddToExecTarget] = useState("");
  const [addToExecFilter, setAddToExecFilter] = useState("");
  const [addToExecWorking, setAddToExecWorking] = useState(false);
  const [addToExecError, setAddToExecError] = useState("");
  const [page, setPage] = useViewState(profileId, "bugs", "page", 0); // 0-based
  const [pageSize, setPageSize] = useViewState(profileId, "bugs", "pageSize", 5);
  const [sortField, setSortField] = useViewState(profileId, "bugs", "sortField", "key");
  const [sortDesc, setSortDesc] = useViewState(profileId, "bugs", "sortDesc", true);
  const [testFilter, setTestFilter] = useViewState<"all" | "with" | "without">(profileId, "bugs", "testFilter", "all");
  const [syncing, setSyncing] = useState(false);
  const [exporting, setExporting] = useState(false);
  const { prompt } = usePrompt();

  // Extended bug detail (description + defect custom fields) fetched lazily
  // when a bug is selected, bypassed for NEW- keys.
  const [bugDetail, setBugDetail] = useState<BugDetail | null>(null);
  const [bugDetailLoading, setBugDetailLoading] = useState(false);
  // Collapsible description in the detail card — collapsed by default.
  const [descOpen, setDescOpen] = useState(false);
  // Collapsible defect analysis in the detail card -- collapsed by default.
  const [analysisOpen, setAnalysisOpen] = useState(false);
  // Collapse the bug metadata + extra fields to a header line, like the
  // requirement detail. Expanded by default.
  const [detailsOpen, setDetailsOpen] = useState(true);

  // In-view right sidebar: persists the open test key across sessions (test detail
  // only; plan/exec panels are ephemeral and reset to null on navigation).
  const [detailKey, setDetailKey] = useViewState<string | null>(profileId, "bugs", "detailKey", null);
  // sidebarDetail drives which panel the right sidebar shows. kind=test uses
  // detailKey + detailVersion; kind=plan/exec is ephemeral (not session-persisted).
  type SidebarDetail = { kind: "test"; key: string } | { kind: "plan" | "exec"; key: string } | null;
  const [sidebarDetail, setSidebarDetail] = useState<SidebarDetail>(
    detailKey ? { kind: "test", key: detailKey } : null
  );
  const [detailVersion, setDetailVersion] = useState(0);

  // Keep sidebarDetail in sync when the profile switches and a different
  // persisted detailKey is loaded. useState ignores subsequent changes to its
  // initializer, so an explicit sync effect is required on profileId changes.
  useEffect(() => {
    setSidebarDetail(detailKey ? { kind: "test", key: detailKey } : null);
  }, [profileId]); // eslint-disable-line react-hooks/exhaustive-deps

  // Ephemeral expand state for the affected-tests table. Not session-persisted
  // because it's a transient drill-down, not a view preference.
  const [expandedTests, setExpandedTests] = useState<Set<string>>(new Set());
  // Cache for per-test run history fetched on first expand (keyed by test key).
  const [runHistoryCache, setRunHistoryCache] = useState<Map<string, TestRunEntry[]>>(new Map());
  // Set of test keys whose run history is currently loading.
  const [runHistoryLoading, setRunHistoryLoading] = useState<Set<string>>(new Set());
  // Per-test sort state for the run-history breakdown. Keyed by test key.
  // Default: finishedAt descending (newest run first).
  type RunSort = { field: string; desc: boolean };
  const [runSortMap, setRunSortMap] = useState<Map<string, RunSort>>(new Map());

  function getRunSort(testKey: string): RunSort {
    return runSortMap.get(testKey) ?? { field: "finishedAt", desc: true };
  }

  function toggleRunSort(testKey: string, field: string) {
    setRunSortMap((prev) => {
      const current = prev.get(testKey) ?? { field: "finishedAt", desc: true };
      const next: RunSort =
        current.field === field
          ? { field, desc: !current.desc }
          : { field, desc: true };
      return new Map(prev).set(testKey, next);
    });
  }

  function sortedRuns(runs: TestRunEntry[], sort: RunSort): TestRunEntry[] {
    return [...runs].sort((a, b) => {
      let cmp = 0;
      switch (sort.field) {
        case "runStatus":
          cmp = (a.runStatus ?? "").localeCompare(b.runStatus ?? "");
          break;
        case "finishedAt":
          cmp = (a.finishedAt || a.startedAt || "").localeCompare(b.finishedAt || b.startedAt || "");
          break;
        case "execCreated":
          cmp = (a.execCreated || "").localeCompare(b.execCreated || "");
          break;
        case "execUpdated":
          cmp = (a.execUpdated || "").localeCompare(b.execUpdated || "");
          break;
        case "execResolved":
          cmp = (a.execResolved || "").localeCompare(b.execResolved || "");
          break;
        case "execKey":
          cmp = (a.execKey || "").localeCompare(b.execKey || "");
          break;
        case "fixVersions":
          cmp = ((a.fixVersions || []).join(", ")).localeCompare((b.fixVersions || []).join(", "));
          break;
        case "environment":
          cmp = (a.environment || "").localeCompare(b.environment || "");
          break;
        case "executedBy":
          cmp = (a.executedBy || "").localeCompare(b.executedBy || "");
          break;
        case "defects":
          cmp = ((a.defects || []).join(", ")).localeCompare((b.defects || []).join(", "));
          break;
        default:
          cmp = 0;
      }
      return sort.desc ? -cmp : cmp;
    });
  }

  // Toggle the run-history expand for a test row. Fetches history on first
  // expand if not already cached.
  function toggleTestExpand(testKey: string) {
    setExpandedTests((prev) => {
      const next = new Set(prev);
      if (next.has(testKey)) {
        next.delete(testKey);
      } else {
        next.add(testKey);
        // Fetch history only if not already cached.
        if (!runHistoryCache.has(testKey)) {
          setRunHistoryLoading((l) => { const nl = new Set(l); nl.add(testKey); return nl; });
          GetTestRunHistory(profileId, testKey)
            .then((entries) => {
              setRunHistoryCache((m) => new Map(m).set(testKey, entries ?? []));
            })
            .catch(() => {
              setRunHistoryCache((m) => new Map(m).set(testKey, []));
            })
            .finally(() => {
              setRunHistoryLoading((l) => { const nl = new Set(l); nl.delete(testKey); return nl; });
            });
        }
      }
      return next;
    });
  }

  // Toggle a bug's checkbox without disturbing the detail-pane selection.
  function toggleChecked(key: string) {
    setChecked((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  // De-duplicated union of the linked test keys across all checked bugs. A bug
  // with no linked tests contributes nothing, so the union can be empty even
  // when bugs are checked - in that case the action is disabled.
  const unionTestKeys = useMemo(() => {
    const set = new Set<string>();
    for (const b of bugs) {
      if (checked.has(b.key)) {
        for (const k of b.testKeys ?? []) set.add(k);
      }
    }
    return [...set];
  }, [bugs, checked]);

  const isDemo = isDemoUrl(jiraUrl);
  const canLink = !!jiraUrl && !isDemo;
  function openBug(key: string) {
    const base = (jiraUrl ?? "").trim().replace(/\/+$/, "");
    if (base && canLink && !key.startsWith("NEW-"))
      BrowserOpenURL(`${base}/browse/${key}`);
  }

  // Per-bucket counts for the test-linkage pill filter, computed from the
  // text-filtered list so the counts respond to the search input.
  const testLinkCounts = useMemo(() => {
    const f = filter.trim().toLowerCase();
    const textFiltered = !f
      ? bugs
      : bugs.filter(
          (b) =>
            b.key.toLowerCase().includes(f) ||
            b.summary.toLowerCase().includes(f) ||
            b.projectKey.toLowerCase().includes(f) ||
            b.status.toLowerCase().includes(f),
        );
    const withTests = textFiltered.filter((b) => b.testKeys.length > 0).length;
    return {
      all: textFiltered.length,
      with: withTests,
      without: textFiltered.length - withTests,
    };
  }, [bugs, filter]);

  const shown = useMemo(() => {
    const f = filter.trim().toLowerCase();
    const textFiltered = !f
      ? bugs
      : bugs.filter(
          (b) =>
            b.key.toLowerCase().includes(f) ||
            b.summary.toLowerCase().includes(f) ||
            b.projectKey.toLowerCase().includes(f) ||
            b.status.toLowerCase().includes(f),
        );
    const linkFiltered =
      testFilter === "with"
        ? textFiltered.filter((b) => b.testKeys.length > 0)
        : testFilter === "without"
          ? textFiltered.filter((b) => b.testKeys.length === 0)
          : textFiltered;
    return [...linkFiltered].sort((a, b) => applyDir(cmpBug(a, b, sortField), sortDesc));
  }, [bugs, filter, testFilter, sortField, sortDesc]);

  // Reset to the first page whenever the data source, filter, or test-linkage filter changes.
  useEffect(() => {
    setPage(0);
  }, [profileId, filter, testFilter, sortField, sortDesc]);

  // Keep a valid selection: default to the first shown bug, and re-point when
  // the current one is filtered out.
  useEffect(() => {
    if (shown.length === 0) {
      setSelected("");
    } else if (!shown.some((b) => b.key === selected)) {
      setSelected(shown[0].key);
    }
  }, [shown, selected]);

  // Clear the per-test expand state whenever the user switches to a different
  // bug, so stale expansions from the previous selection don't carry over.
  // Also collapse the description so it doesn't stay open across navigations.
  useEffect(() => {
    setExpandedTests(new Set());
    setDescOpen(false);
    setAnalysisOpen(false);
    setDetailsOpen(true);
  }, [selected]);

  // (The affected tests for the selected bug now load via useBugTests above.)

  // Load extended bug detail (description + defect fields) for the selected bug.
  // Skipped for NEW- placeholder keys that have no real Jira issue yet.
  useEffect(() => {
    if (!profileId || !selected || selected.startsWith("NEW-")) {
      setBugDetail(null);
      return;
    }
    let cancelled = false;
    setBugDetailLoading(true);
    setBugDetail(null);
    GetBugDetail(profileId, selected)
      .then((d) => {
        if (!cancelled) setBugDetail(d ?? null);
      })
      .catch(() => {
        if (!cancelled) setBugDetail(null);
      })
      .finally(() => {
        if (!cancelled) setBugDetailLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [profileId, selected]);

  const totalPages = Math.max(1, Math.ceil(shown.length / pageSize));
  const safePage = Math.min(Math.max(0, page), totalPages - 1);
  const paged = shown.slice(safePage * pageSize, safePage * pageSize + pageSize);
  const sel = bugs.find((b) => b.key === selected) ?? null;

  // Select-all state for the current page.
  const allPageChecked = paged.length > 0 && paged.every((b) => checked.has(b.key));
  const somePageChecked = paged.some((b) => checked.has(b.key)) && !allPageChecked;

  // Select-all state across every bug matching the current filter (all pages).
  const allShownChecked = shown.length > 0 && shown.every((b) => checked.has(b.key));
  const someShownChecked = shown.some((b) => checked.has(b.key)) && !allShownChecked;

  function toggleSelectAll() {
    setChecked((prev) => {
      const next = new Set(prev);
      if (allPageChecked) {
        // Deselect all on this page.
        for (const b of paged) next.delete(b.key);
      } else {
        // Select all on this page.
        for (const b of paged) next.add(b.key);
      }
      return next;
    });
  }

  // Select (or clear) every bug matching the current filter, across all pages —
  // not just the visible page.
  function toggleSelectAllShown() {
    setChecked((prev) => {
      const next = new Set(prev);
      if (allShownChecked) {
        for (const b of shown) next.delete(b.key);
      } else {
        for (const b of shown) next.add(b.key);
      }
      return next;
    });
  }

  return { profileId, prompt, filter, setFilter, error, setError, notice, setNotice, selected, setSelected, checked, setChecked, creating, setCreating, addToExecOpen, setAddToExecOpen, addToExecLoading, setAddToExecLoading, addToExecExecs, setAddToExecExecs, addToExecTarget, setAddToExecTarget, addToExecFilter, setAddToExecFilter, addToExecWorking, setAddToExecWorking, addToExecError, setAddToExecError, page, setPage, pageSize, setPageSize, sortField, setSortField, sortDesc, setSortDesc, testFilter, setTestFilter, syncing, setSyncing, exporting, setExporting, bugDetail, setBugDetail, bugDetailLoading, setBugDetailLoading, descOpen, setDescOpen, analysisOpen, setAnalysisOpen, detailsOpen, setDetailsOpen, detailKey, setDetailKey, sidebarDetail, setSidebarDetail, detailVersion, setDetailVersion, expandedTests, setExpandedTests, runHistoryCache, setRunHistoryCache, runHistoryLoading, setRunHistoryLoading, runSortMap, setRunSortMap, bugsQuery, bugs, bugTestsQuery, tests, unionTestKeys, isDemo, canLink, testLinkCounts, shown, totalPages, safePage, paged, sel, allPageChecked, somePageChecked, allShownChecked, someShownChecked, getRunSort, toggleRunSort, sortedRuns, toggleTestExpand, toggleChecked, openBug, toggleSelectAll, toggleSelectAllShown };
}

export type BugsState = ReturnType<typeof useBugsState>;
