import { useEffect, useMemo, useState } from "react";
import { useFocusWhen } from "@agile-suite/core";
import { useProfile } from "../../contexts/ProfileContext";
import { useViewState } from "../../lib/viewState";
import { errMsg } from "../../api";
import type { ExecMemberRun, RollupMember } from "../../api";
import { keyCompare, cmpStr, applyDir } from "../../sort";
import { useContainers, useContainerBoard, useContainerBugs, useContainerMembers, useContainerRollup } from "../../queries/containers";
import { useCapabilities } from "../../features";
import { KINDS } from "./constants";

// useContainerState holds what the Containers view knows: the chosen kind and
// container with their filters, the queries behind them, the member table's
// sort, filters and page, and the open panels. Effects reset the dependent
// state when the kind or the container changes.
export function useContainerState() {
  const { activeId: profileId } = useProfile();
  // Gates the Test Execution environment controls to what the active
  // profile's backend actually supports (P6.2a).
  const caps = useCapabilities(profileId);
  const [kind, setKind] = useViewState(profileId, "containers", "kind", "testplan");
  const [cStatus, setCStatus] = useViewState(profileId, "containers", "cStatus", "");
  // Execution-type filter (Test Execution kind only): "" = all, "standalone", "subtask".
  const [cExecType, setCExecType] = useViewState(profileId, "containers", "cExecType", "");
  // Environment filter (Test Execution kind only): "" = any; otherwise keep only
  // executions whose environments array contains the value (mirrors
  // ContainerQuery.Environment server-side, applied client-side here since the
  // environments are already loaded on each container).
  const [cEnv, setCEnv] = useViewState(profileId, "containers", "cEnv", "");
  // Label filter (all kinds): "" = any; otherwise keep only containers whose
  // labels array contains the value.
  const [cLabel, setCLabel] = useViewState(profileId, "containers", "cLabel", "");
  // Inline environment editor (selected execution): a draft of a new env name.
  const [envDraft, setEnvDraft] = useState("");
  // Batch environment editor (all currently-filtered executions): chosen
  // operation, the env name to apply, and an in-flight guard.
  const [batchEnvOp, setBatchEnvOp] = useState<"add_env" | "remove_env" | "set_env">("add_env");
  const [batchEnvName, setBatchEnvName] = useState("");
  const [batchEnvBusy, setBatchEnvBusy] = useState(false);
  // The batch-environment controls live in a popover so the filter bar stays a
  // single compact row instead of a dedicated tools strip (#310 follow-up).
  const [envToolsOpen, setEnvToolsOpen] = useState(false);
  const [cSortField, setCSortField] = useViewState(profileId, "containers", "cSortField", "key");
  const [cSortDesc, setCSortDesc] = useViewState(profileId, "containers", "cSortDesc", false);
  const [rowSortField, setRowSortField] = useState("key");
  const [rowSortDesc, setRowSortDesc] = useState(false);
  const containersQuery = useContainers(profileId, kind);
  const containers = containersQuery.data ?? [];
  const listError = containersQuery.error ? errMsg(containersQuery.error) : "";
  const [selected, setSelected] = useViewState(profileId, "containers", "selected", "");
  // The selected container's detail reads come from the query cache with stable
  // keys (Phase 4c), refreshed by the same invalidation as the container list.
  const boardQuery = useContainerBoard(profileId, selected);
  const board = boardQuery.data ?? null;
  const relatedBugs = useContainerBugs(profileId, selected).data ?? [];
  const membersQuery = useContainerMembers(profileId, selected, kind);
  const memberRuns = useMemo(() => {
    const m = new Map<string, ExecMemberRun>();
    for (const r of membersQuery.data ?? []) m.set(r.testKey, r);
    return m;
  }, [membersQuery.data]);
  const rollup = useContainerRollup(profileId, selected, kind).data ?? null;
  const [bugFor, setBugFor] = useState<{ testKey: string; summary: string } | null>(null);
  // Test key whose Defects cell has an open LinkBugPicker (Test Execution
  // member table only).
  const [linkBugFor, setLinkBugFor] = useState<string | null>(null);
  const [mode, setMode] = useViewState<"containers" | "bugs">(profileId, "containers", "mode", "containers");
  // Whether the related-bugs collapsible section in the container card is open.
  // Collapsed by default so a large bug list never hides the member table below.
  const [bugsExpanded, setBugsExpanded] = useState(false);
  const loading = containersQuery.isFetching;
  const [error, setError] = useState("");
  const [seeding, setSeeding] = useState(false);
  const [syncing, setSyncing] = useState(false);
  // Bulk execution-result selection: the set of Test keys checked in the board
  // (Test Execution only).
  const [selectedRuns, setSelectedRuns] = useState<Set<string>>(new Set());
  const [bulkStatus, setBulkStatus] = useState("PASS");
  const [editingName, setEditingName] = useState(false);
  const focusRef = useFocusWhen<HTMLInputElement>(editingName);
  const [nameDraft, setNameDraft] = useState("");
  const [showAdd, setShowAdd] = useState(false);
  const [showJUnitImport, setShowJUnitImport] = useState(false);
  const [showJUnitNewExec, setShowJUnitNewExec] = useState(false);
  const [boardPage, setBoardPage] = useState(0);
  // Active fix-version filter for the member table (Test Execution only). Empty
  // string means "All". Clicking an execution fix-version chip toggles it; a
  // second click clears the filter (single-select toggle).
  const [memberFvFilter, setMemberFvFilter] = useState("");
  // Run-status filter for the member table, driven by clicking a segment of the
  // run colorbar or one of its count badges. Empty string means "all"; the
  // "(not run)" bucket matches members with a blank run status.
  const [memberRunFilter, setMemberRunFilter] = useState("");
  // Clickable roll-up breakdown: which bucket's modal is open, and the member
  // detail (lazily fetched on first badge click, cached per selected container).
  const [breakdownStatus, setBreakdownStatus] = useState<string | null>(null);
  const [breakdown, setBreakdown] = useState<RollupMember[] | null>(null);
  const [breakdownFor, setBreakdownFor] = useState("");
  const [breakdownLoading, setBreakdownLoading] = useState(false);
  // Collapse the selected container's detail (parent, environments, fix
  // versions, name, run bar) to a single header line to give the member table
  // more room. Persisted per profile so the choice sticks across selections.
  const [cardCollapsed, setCardCollapsed] = useViewState(
    profileId,
    "containers",
    "cardCollapsed",
    false,
  );

  // In-view read-only test detail sidebar for Test Execution member rows:
  // detailKey is session-persisted so the panel restores on returning to this
  // view; detailVersion is ephemeral and bumped on each open to force a
  // re-fetch even when reopening the same test.
  const [detailKey, setDetailKey] = useViewState<string | null>(profileId, "containers", "detailKey", null);
  const [detailVersion, setDetailVersion] = useState(0);

  // Member tables can be long (an Execution may hold hundreds of tests), so the
  // board is paged client-side. The page size is user-selectable; default 15.
  const [pageSize, setPageSize] = useState(15);

  const kindLabel = KINDS.find((k) => k.value === kind)?.label ?? "container";
  const selectedContainer = containers.find((c) => c.key === selected) ?? null;

  const statusOptions = useMemo(() => {
    const s = new Set<string>();
    for (const c of containers) if (c.status) s.add(c.status);
    return [...s].sort();
  }, [containers]);

  // Per-status counts for the pill filter (denominator = all containers for the
  // current kind, before any status/type/env filtering).
  const statusCounts = useMemo(() => {
    const m = new Map<string, number>();
    m.set("", containers.length);
    for (const c of containers) {
      if (c.status) m.set(c.status, (m.get(c.status) ?? 0) + 1);
    }
    return m;
  }, [containers]);

  const viewContainers = useMemo(() => {
    const base = containers.filter(
      (c) =>
        (!cStatus || c.status === cStatus) &&
        (!cLabel || (c.labels ?? []).includes(cLabel)) &&
        (kind !== "testexec" ||
          !cExecType ||
          (cExecType === "subtask" ? !!c.parentKey : !c.parentKey)) &&
        (kind !== "testexec" ||
          !cEnv ||
          (c.environments ?? []).includes(cEnv)),
    );
    return [...base].sort((a, b) => {
      let cmp: number;
      switch (cSortField) {
        case "summary":
          cmp = cmpStr(a.summary, b.summary) || keyCompare(a.key, b.key);
          break;
        case "status":
          cmp = cmpStr(a.status, b.status) || keyCompare(a.key, b.key);
          break;
        default:
          cmp = keyCompare(a.key, b.key);
      }
      return applyDir(cmp, cSortDesc);
    });
  }, [containers, cStatus, cLabel, cExecType, cEnv, kind, cSortField, cSortDesc]);

  // Distinct environments across the loaded executions, for the filter dropdown.
  const envOptions = useMemo(() => {
    const s = new Set<string>();
    for (const c of containers)
      for (const e of c.environments ?? []) if (e) s.add(e);
    return [...s].sort();
  }, [containers]);

  // Distinct labels across all loaded containers, for the label filter.
  const labelOptions = useMemo(() => {
    const s = new Set<string>();
    for (const c of containers)
      for (const l of c.labels ?? []) if (l) s.add(l);
    return [...s].sort();
  }, [containers]);

  useEffect(() => {
    if (viewContainers.length === 0) return;
    setSelected((cur) => {
      if (viewContainers.some((c) => c.key === cur)) return cur;
      return viewContainers[0].key;
    });
  }, [viewContainers]);

  useEffect(() => {
    setBugFor(null);
    setLinkBugFor(null);
  }, [mode, selected, kind]);

  useEffect(() => {
    setCStatus("");
    setCExecType("");
    setEnvToolsOpen(false);
  }, [kind]);

  // Close the batch-environment popover on Escape, matching the Menu component.
  useEffect(() => {
    if (!envToolsOpen) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") setEnvToolsOpen(false);
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [envToolsOpen]);

  // Collapse the bugs section whenever the user picks a different container so
  // a previously-expanded list from another container doesn't carry over.
  useEffect(() => {
    setBugsExpanded(false);
  }, [selected]);

  useEffect(() => {
    const rows = containersQuery.data;
    if (!rows) return;
    setSelected((cur) =>
      cur && rows.some((c) => c.key === cur) ? cur : rows.length > 0 ? rows[0].key : "",
    );
  }, [containersQuery.data]);

  useEffect(() => {
    setBoardPage(0);
  }, [rowSortField, rowSortDesc]);

  // The board / related bugs / member runs / roll-up now load via the
  // useContainer* queries above. Reset the board's view state when the selected
  // container changes (the old board load effect did this before fetching).
  useEffect(() => {
    setError("");
    setBoardPage(0);
    setSelectedRuns(new Set());
    setMemberFvFilter("");
    setMemberRunFilter("");
  }, [selected]);

  // Reset the cached breakdown when the selected container changes, so a stale
  // one is never shown for a different plan/set.
  useEffect(() => {
    setBreakdown(null);
    setBreakdownFor("");
    setBreakdownStatus(null);
  }, [selected]);

  // Client-side paging of the member table.
  const allRows = useMemo(() => {
    let rows = board?.rows ?? [];
    // Apply the fix-version filter for Test Executions: keep only members whose
    // own fixVersions array (from the memberRuns enrichment) contains the chosen
    // value. When no filter is active every row is shown.
    if (kind === "testexec" && memberFvFilter) {
      rows = rows.filter((r) =>
        (memberRuns.get(r.testKey)?.fixVersions ?? []).includes(memberFvFilter),
      );
    }
    // Run-status filter from clicking the colorbar / a count badge. "(not run)"
    // matches members with no run result (blank), mirroring how the backend
    // buckets runCounts (blankAs(runStatus, "(not run)")).
    if (memberRunFilter) {
      rows = rows.filter((r) =>
        memberRunFilter === "(not run)"
          ? !r.runStatus
          : r.runStatus === memberRunFilter,
      );
    }
    return [...rows].sort((a, b) => {
      let cmp: number;
      switch (rowSortField) {
        case "summary":
          cmp = cmpStr(a.summary, b.summary) || keyCompare(a.testKey, b.testKey);
          break;
        case "status":
          cmp = cmpStr(a.status, b.status) || keyCompare(a.testKey, b.testKey);
          break;
        case "result":
          cmp =
            cmpStr(a.runStatus, b.runStatus) ||
            keyCompare(a.testKey, b.testKey);
          break;
        default:
          cmp = keyCompare(a.testKey, b.testKey);
      }
      return applyDir(cmp, rowSortDesc);
    });
  }, [board, kind, memberRuns, memberFvFilter, memberRunFilter, rowSortField, rowSortDesc]);

  // Toggle the run-status filter from a colorbar segment / badge and reset to
  // the first page so the (possibly shorter) filtered list starts at the top.
  const pickRunFilter = (label: string) => {
    setMemberRunFilter((cur) => (cur === label ? "" : label));
    setBoardPage(0);
  };
  const boardTotalPages = Math.max(
    1,
    Math.ceil(allRows.length / pageSize),
  );
  const safePage = Math.min(boardPage, boardTotalPages - 1);
  const pageRows = allRows.slice(
    safePage * pageSize,
    (safePage + 1) * pageSize,
  );

  return { kind, setKind, cStatus, setCStatus, cExecType, setCExecType, cEnv, setCEnv, cLabel, setCLabel, envDraft, setEnvDraft, batchEnvOp, setBatchEnvOp, batchEnvName, setBatchEnvName, batchEnvBusy, setBatchEnvBusy, envToolsOpen, setEnvToolsOpen, cSortField, setCSortField, cSortDesc, setCSortDesc, rowSortField, setRowSortField, rowSortDesc, setRowSortDesc, selected, setSelected, bugFor, setBugFor, linkBugFor, setLinkBugFor, mode, setMode, bugsExpanded, setBugsExpanded, error, setError, seeding, setSeeding, syncing, setSyncing, selectedRuns, setSelectedRuns, bulkStatus, setBulkStatus, editingName, setEditingName, nameDraft, setNameDraft, showAdd, setShowAdd, showJUnitImport, setShowJUnitImport, showJUnitNewExec, setShowJUnitNewExec, boardPage, setBoardPage, memberFvFilter, setMemberFvFilter, memberRunFilter, setMemberRunFilter, breakdownStatus, setBreakdownStatus, breakdown, setBreakdown, breakdownFor, setBreakdownFor, breakdownLoading, setBreakdownLoading, cardCollapsed, setCardCollapsed, detailKey, setDetailKey, detailVersion, setDetailVersion, pageSize, setPageSize, profileId, caps, containersQuery, containers, listError, boardQuery, board, relatedBugs, membersQuery, memberRuns, rollup, loading, focusRef, kindLabel, selectedContainer, statusOptions, statusCounts, viewContainers, envOptions, labelOptions, allRows, pickRunFilter, boardTotalPages, safePage, pageRows };
}

export type ContainerState = ReturnType<typeof useContainerState>;
