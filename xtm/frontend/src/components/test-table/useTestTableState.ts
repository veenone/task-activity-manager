import { useEffect, useRef, useState } from "react";
import { useSelection } from "../../contexts/SelectionContext";
import { useProfile } from "../../contexts/ProfileContext";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useViewState } from "../../lib/viewState";
import { ListStatuses, ListSavedViews, errMsg } from "../../api";
import { usePrompt } from "../usePrompt";
import { useNotice } from "../useNotice";
import { useCapabilities } from "../../features";
import type { TestQuery, PendingChange, SavedView } from "../../api";
import { useTests } from "../../queries/tests";
import { useTestCallerKeys } from "../../queries/testCalls";
import { DEFAULT_PAGE_SIZE, EMPTY_CALLER_KEYS, SELECT_W, ROW_HEIGHT, COL_WIDTH, COLUMNS_STORAGE_KEY, loadColumns } from "./columns";
import type { SortCol, ColKey, ColState } from "./columns";

export interface TestTableProps {
  folderId: string;
  containerKey: string;
  component: string;
  pendingByTestKey: Map<string, PendingChange[]>;
  onSync?: () => void;
  syncing?: boolean;
}

// useTestTableState holds what the Browse grid knows: the filters, sort and
// page (kept per profile), the query behind the page, the row virtualizer and
// roving focus, the column config, saved views and status options, and the
// page-selection state. Effects debounce the search and reset the page.
export function useTestTableState(p: TestTableProps) {
  const { folderId, containerKey, component } = p;
  const { activeId: profileId } = useProfile();
  // Browse selection comes from SelectionContext; the aliases keep the existing
  // onSelect / onToggleSelect / … call sites in this component unchanged.
  const {
    selectedKey,
    selectedSet,
    setSelectedKey: onSelect,
    toggle: onToggleSelect,
    togglePage: onToggleSelectPage,
    selectAllMatching: onSelectAllMatching,
  } = useSelection();
  // Gates the exec-type filter to what the active profile's backend actually
  // supports (P6.2a).
  const caps = useCapabilities(profileId);
  const [search, setSearch] = useViewState(profileId, "browse", "search", "");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [status, setStatus] = useViewState(profileId, "browse", "status", "");
  const [execType, setExecType] = useViewState(profileId, "browse", "execType", "");
  const [review, setReview] = useState("");
  const [sortBy, setSortBy] = useViewState<SortCol>(profileId, "browse", "sortBy", "key");
  // Default to newest-first by issue number (RND_P_4TFINT_05-202). The key sort
  // is numeric on the trailing number, so this lists the highest issue keys
  // first; saved views restore their own direction.
  const [desc, setDesc] = useViewState(profileId, "browse", "desc", true);
  const [offset, setOffset] = useViewState(profileId, "browse", "offset", 0);
  const [pageSize, setPageSize] = useViewState(profileId, "browse", "pageSize", DEFAULT_PAGE_SIZE);
  const [pageInput, setPageInput] = useState("");

  // The browse grid's page comes from the query cache (audit A3, Phase 2),
  // refreshed by invalidateProfileData on a mutation.
  const q: TestQuery = {
    search: debouncedSearch,
    status: status.trim(),
    folderId,
    containerKey,
    component,
    execType,
    review,
    sortBy,
    desc,
    limit: pageSize,
    offset,
  };
  const testsQuery = useTests(profileId, q);
  const page = testsQuery.data ?? { tests: [], total: 0 };
  const loading = testsQuery.isFetching;
  const listError = testsQuery.error ? errMsg(testsQuery.error) : "";
  // Keys of tests that call another test in their steps — drives the grid cue.
  const callerKeys = useTestCallerKeys(profileId).data ?? EMPTY_CALLER_KEYS;
  // Roving tabindex over the grid rows, tracked by index because rows are
  // virtualized (off-screen rows aren't in the DOM). Exactly one row is in the
  // tab order; Arrow/Home/End move focus by scrolling the target row into view
  // and focusing it (X6).
  const [focusedIndex, setFocusedIndex] = useState(-1);
  const scrollRef = useRef<HTMLDivElement>(null);
  const pendingFocusRef = useRef<number | null>(null);
  const [error, setError] = useState("");
  const [selectingAll, setSelectingAll] = useState(false);
  const [selectAllError, setSelectAllError] = useState("");

  const [savedViews, setSavedViews] = useState<SavedView[]>([]);
  const [activeView, setActiveView] = useState("");
  const [statusOptions, setStatusOptions] = useState<string[]>([]);
  const { prompt } = usePrompt();
  const { notice } = useNotice();

  const [columns, setColumns] = useState<ColState[]>(loadColumns);
  const [showColumns, setShowColumns] = useState(false);
  useEffect(() => {
    localStorage.setItem(COLUMNS_STORAGE_KEY, JSON.stringify(columns));
  }, [columns]);

  // (Caller badges now come from the useTestCallerKeys query above.)
  const visibleColumns = columns.filter((c) => c.visible);

  // Virtualize the row body so a large page (up to 500 rows) only mounts the
  // ~visible slice instead of every <tr> (P2). Rows are a fixed height, so a
  // constant estimate is exact and no per-row measurement is needed.
  const rowVirtualizer = useVirtualizer({
    count: page.tests.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 12,
  });

  // The single keyboard-tabbable row index: the last-focused row if still on
  // this page, else the open row, else the first row — always a valid index so
  // the grid stays reachable by Tab after paging/filtering.
  const selectedIndex = page.tests.findIndex((t) => t.key === selectedKey);
  const tabbableIndex =
    focusedIndex >= 0 && focusedIndex < page.tests.length
      ? focusedIndex
      : selectedIndex >= 0
        ? selectedIndex
        : 0;

  // Total fixed width of the visible columns; the grid never shrinks below this,
  // so columns keep their widths and the scroll container pans horizontally.
  const gridMinWidth =
    SELECT_W + visibleColumns.reduce((sum, c) => sum + COL_WIDTH[c.key], 0);

  // Focus a row by index. The row may not be mounted when navigation asks for
  // it, so moveFocus records the target and a post-render effect retries.
  function focusRowByIndex(index: number): boolean {
    const el = scrollRef.current?.querySelector<HTMLElement>(
      `tr[data-rowindex="${index}"]`,
    );
    if (el) {
      el.focus();
      return true;
    }
    return false;
  }
  function moveFocus(target: number) {
    if (page.tests.length === 0) return;
    const clamped = Math.max(0, Math.min(page.tests.length - 1, target));
    setFocusedIndex(clamped);
    rowVirtualizer.scrollToIndex(clamped, { align: "auto" });
    if (!focusRowByIndex(clamped)) pendingFocusRef.current = clamped;
  }
  useEffect(() => {
    if (
      pendingFocusRef.current != null &&
      focusRowByIndex(pendingFocusRef.current)
    ) {
      pendingFocusRef.current = null;
    }
  });

  function toggleColumn(key: ColKey) {
    setColumns((prev) =>
      prev.map((c) => (c.key === key ? { ...c, visible: !c.visible } : c)),
    );
  }
  function moveColumn(key: ColKey, dir: -1 | 1) {
    setColumns((prev) => {
      const i = prev.findIndex((c) => c.key === key);
      const j = i + dir;
      if (i < 0 || j < 0 || j >= prev.length) return prev;
      const next = [...prev];
      [next[i], next[j]] = [next[j], next[i]];
      return next;
    });
  }

  useEffect(() => {
    let cancelled = false;
    ListSavedViews(profileId)
      .then((vs) => {
        if (!cancelled) setSavedViews(vs ?? []);
      })
      .catch((e) => console.error("list views:", errMsg(e)));
    // Workflow statuses for the filter dropdown (FR-4): authoritative list from
    // Jira's Test workflow, unioned with statuses present in the synced data.
    ListStatuses(profileId)
      .then((s) => {
        if (!cancelled) setStatusOptions(s ?? []);
      })
      .catch((e) => console.error("list statuses:", errMsg(e)));
    return () => {
      cancelled = true;
    };
  }, [profileId]);

  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(search), 250);
    return () => clearTimeout(t);
  }, [search]);

  useEffect(() => {
    setOffset(0);
  }, [
    debouncedSearch,
    status,
    execType,
    review,
    folderId,
    containerKey,
    component,
    sortBy,
    desc,
    pageSize,
    profileId,
  ]);

  // (The browse page fetch now lives in the useTests query above.)

  function toggleSort(col: SortCol) {
    if (sortBy === col) {
      setDesc((d) => !d);
    } else {
      setSortBy(col);
      setDesc(false);
    }
  }

  const from = page.total === 0 ? 0 : offset + 1;
  const to = Math.min(offset + pageSize, page.total);
  const totalPages = Math.max(1, Math.ceil(page.total / pageSize));
  const currentPage = Math.floor(offset / pageSize) + 1;

  // goToPage clamps to the valid range and moves the offset to that page's
  // first row.
  function goToPage(n: number) {
    const clamped = Math.min(Math.max(1, n), totalPages);
    setOffset((clamped - 1) * pageSize);
  }

  // commitPageInput parses the jump-to-page box and navigates. Invalid input is
  // ignored; the box is then cleared so it shows the live page again.
  function commitPageInput() {
    const n = parseInt(pageInput, 10);
    if (!Number.isNaN(n)) goToPage(n);
    setPageInput("");
  }

  const pageKeys = page.tests.map((t) => t.key);
  const allOnPageSelected =
    pageKeys.length > 0 && pageKeys.every((k) => selectedSet.has(k));
  const someOnPageSelected =
    !allOnPageSelected && pageKeys.some((k) => selectedSet.has(k));

  // The Gmail-style "select all matching" banner shows when the current
  // page is fully selected AND the filter has more results we haven't yet
  // included. selectedSet.size meeting page.total means the user already
  // extended to the full result set — hide the banner in that case.
  const canSelectAllMatching =
    allOnPageSelected &&
    page.total > pageKeys.length &&
    selectedSet.size < page.total;

  return { profileId, selectedKey, selectedSet, onSelect, onToggleSelect, onToggleSelectPage, onSelectAllMatching, prompt, notice, search, setSearch, debouncedSearch, setDebouncedSearch, status, setStatus, execType, setExecType, review, setReview, sortBy, setSortBy, desc, setDesc, offset, setOffset, pageSize, setPageSize, pageInput, setPageInput, focusedIndex, setFocusedIndex, error, setError, selectingAll, setSelectingAll, selectAllError, setSelectAllError, savedViews, setSavedViews, activeView, setActiveView, statusOptions, setStatusOptions, columns, setColumns, showColumns, setShowColumns, caps, q, testsQuery, page, loading, listError, callerKeys, scrollRef, pendingFocusRef, visibleColumns, rowVirtualizer, selectedIndex, tabbableIndex, gridMinWidth, from, to, totalPages, currentPage, pageKeys, allOnPageSelected, someOnPageSelected, canSelectAllMatching, focusRowByIndex, moveFocus, toggleColumn, moveColumn, toggleSort, goToPage, commitPageInput };
}

export type TestTableState = ReturnType<typeof useTestTableState>;
