import { useEffect, useMemo, useState } from "react";
import { useProfile } from "../../contexts/ProfileContext";
import { useSync } from "../../contexts/SyncContext";
import { usePreconditions, usePreconditionTests } from "../../queries/preconditions";
import { useViewState } from "../../lib/viewState";
import { BrowserOpenURL, isDemoUrl, errMsg } from "../../api";
import type { PreconditionUsage } from "../../api";
import { useConfirm } from "../useConfirm";
import { keyCompare, cmpStr, applyDir } from "../../sort";

export interface PreconditionsViewProps {
  onChanged: () => void;
  // The active profile's Jira base URL, so the detail header's key can open the
  // real issue. Absent on a profile that has none, which hides the link.
  jiraUrl?: string;
}

// Xray Server/DC precondition types. The type drives how Xray interprets the
// precondition definition; Manual is the default for hand-written steps.
export const PRECOND_TYPES = ["Manual", "Generic", "Cucumber"];

function cmpPre(
  a: PreconditionUsage,
  b: PreconditionUsage,
  field: string,
): number {
  switch (field) {
    case "type":
      return cmpStr(a.type, b.type) || keyCompare(a.key, b.key);
    case "status":
      return cmpStr(a.status, b.status) || keyCompare(a.key, b.key);
    case "usage":
      return (
        (a.testCount ?? 0) - (b.testCount ?? 0) || keyCompare(a.key, b.key)
      );
    default:
      return keyCompare(a.key, b.key);
  }
}

// usePreconditionsState holds what the Preconditions view knows: the list and
// the selected precondition's tests, filter, usage pills, sort and both pages,
// the edit drafts, which panels are open, and the docked test detail. Effects
// keep a valid selection and reset the drafts when it changes.
export function usePreconditionsState(p: PreconditionsViewProps) {
  const { jiraUrl } = p;
  const { activeId: profileId } = useProfile();
  // The precondition list and the selected precondition's linked tests both
  // come from the query cache with stable keys (Phase 4c); a mutation refreshes
  // them via invalidateProfileData.
  const preconditionsQuery = usePreconditions(profileId);
  const list = preconditionsQuery.data ?? [];
  const loading = preconditionsQuery.isFetching;
  const listError = preconditionsQuery.error
    ? errMsg(preconditionsQuery.error)
    : "";
  const [selected, setSelected] = useViewState(
    profileId,
    "preconditions",
    "selected",
    "",
  );
  const testsQuery = usePreconditionTests(profileId, selected);
  const tests = testsQuery.data ?? [];
  const [filter, setFilter] = useViewState(
    profileId,
    "preconditions",
    "filter",
    "",
  );
  const [usageFilter, setUsageFilter] = useViewState<
    "all" | "with" | "without"
  >(profileId, "preconditions", "usageFilter", "all");
  const [sortField, setSortField] = useViewState(
    profileId,
    "preconditions",
    "sortField",
    "key",
  );
  const [sortDesc, setSortDesc] = useViewState(
    profileId,
    "preconditions",
    "sortDesc",
    true,
  );
  const [error, setError] = useState("");
  // Said after a preconditions-only sync. Errors go to the detail pane's error
  // line with every other failure; a success has nowhere else to be reported,
  // and a Sync button that leaves the list looking identical needs to say it
  // did something.
  const [notice, setNotice] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [showImport, setShowImport] = useState(false);
  const [showAdd, setShowAdd] = useState(false);
  // A test opened from the "Used by" list, docked as an inline detail beside the
  // precondition detail (mirrors the requirement / browse views, #4).
  const [detailKey, setDetailKey] = useViewState(
    profileId,
    "preconditions",
    "detailKey",
    "",
  );
  const [detailVersion, setDetailVersion] = useState(0);
  const { confirm } = useConfirm();
  // A view-initiated sync has to register with the sync machine, not just call
  // the binding: the reducer drops every sync:progress frame while the machine
  // is idle, so the status bar showed nothing for the whole run. Registering
  // also makes the two syncs exclude each other, since both write the same
  // profile's rows.
  const { beginSync, endSync, canSync } = useSync();

  // editing toggles the detail pane between read-only display and an explicit
  // edit session. Resets to false whenever the selected precondition changes so
  // navigating away never leaves a stale edit session open.
  const [editing, setEditing] = useState(false);

  const selectedPre = list.find((p) => p.key === selected) ?? null;
  const isLocal = selected.startsWith("new-precond-");

  // Refreshing only the preconditions, the way Requirements and Containers do.
  // The precondition stage is the slowest one in a full sync (an association
  // read per precondition), so a user who has just edited a condition in Jira
  // should not have to pay for the test, folder and container passes to see it.
  // syncing is this view's own run, which is what the button label reports.
  // canSync is the machine's global guard, which is what disables it: a sync
  // started anywhere blocks one started here.
  const [syncing, setSyncing] = useState(false);

  // The key opens the real Jira issue in the system browser, the way the test
  // key does in TestDetail. Hidden for demo profiles and for a precondition
  // that only exists locally, neither of which has a Jira page to open.
  const jiraBase = (jiraUrl ?? "").trim().replace(/\/+$/, "");
  const canLinkToJira = !!jiraBase && !isDemoUrl(jiraUrl ?? "") && !isLocal;
  function openInJira() {
    if (canLinkToJira) BrowserOpenURL(`${jiraBase}/browse/${selected}`);
  }

  // Draft buffers for the editable text fields. These are only committed to
  // Jira when the user explicitly clicks Save (not on blur). They resync from
  // the remote whenever the selected precondition reloads, and editing resets
  // to false so navigating to a new precondition always starts in read mode.
  const [summaryDraft, setSummaryDraft] = useState("");
  const [descDraft, setDescDraft] = useState("");
  const [condDraft, setCondDraft] = useState("");
  const [typeDraft, setTypeDraft] = useState("Manual");
  // Collapsible read-only fields -- collapsed by default, reset on selection change.
  const [condOpen, setCondOpen] = useState(false);
  const [descOpen, setDescOpen] = useState(false);
  // Collapse the read-only detail fields (summary, type, condition, description)
  // to a header line, like the requirement detail. Expanded by default.
  const [detailsOpen, setDetailsOpen] = useState(true);
  useEffect(() => {
    setEditing(false);
    setSummaryDraft(selectedPre?.summary ?? "");
    setDescDraft(selectedPre?.description ?? "");
    setCondDraft(selectedPre?.condition ?? "");
    setTypeDraft(selectedPre?.type || "Manual");
    setCondOpen(false);
    setDescOpen(false);
    setDetailsOpen(true);
  }, [selectedPre]);

  // Per-bucket counts for the usage pill filter, computed from the text-filtered
  // list so counts respond to the search input.
  const usageCounts = useMemo(() => {
    const q = filter.trim().toLowerCase();
    const base = !q
      ? list
      : list.filter(
          (p) =>
            p.key.toLowerCase().includes(q) ||
            p.summary.toLowerCase().includes(q) ||
            p.type.toLowerCase().includes(q) ||
            p.status.toLowerCase().includes(q),
        );
    const withTests = base.filter((p) => (p.testCount ?? 0) > 0).length;
    return {
      all: base.length,
      with: withTests,
      without: base.length - withTests,
    };
  }, [list, filter]);

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    const base = !q
      ? list
      : list.filter(
          (p) =>
            p.key.toLowerCase().includes(q) ||
            p.summary.toLowerCase().includes(q) ||
            p.type.toLowerCase().includes(q) ||
            p.status.toLowerCase().includes(q),
        );
    const usageFiltered =
      usageFilter === "all"
        ? base
        : base.filter((p) =>
            usageFilter === "with"
              ? (p.testCount ?? 0) > 0
              : (p.testCount ?? 0) === 0,
          );
    return [...usageFiltered].sort((a, b) =>
      applyDir(cmpPre(a, b, sortField), sortDesc),
    );
  }, [list, filter, usageFilter, sortField, sortDesc]);

  // Pagination of the precondition master list.
  const [listPage, setListPage] = useViewState(
    profileId,
    "preconditions",
    "listPage",
    0,
  );
  const [listPageSize, setListPageSize] = useViewState(
    profileId,
    "preconditions",
    "listPageSize",
    15,
  );
  useEffect(() => {
    setListPage(0);
  }, [filter, usageFilter, sortField, sortDesc]);
  const listTotalPages = Math.max(1, Math.ceil(filtered.length / listPageSize));
  const listSafePage = Math.min(listPage, listTotalPages - 1);
  const pageList = filtered.slice(
    listSafePage * listPageSize,
    (listSafePage + 1) * listPageSize,
  );

  // Load the master list whenever the profile changes or data refreshes,
  // keeping the current selection if it still exists.
  // Auto-select the first precondition once the list loads or changes, keeping
  // the current selection if it still exists. (The list fetch now lives in the
  // usePreconditions query above.)
  useEffect(() => {
    const rows = preconditionsQuery.data;
    if (!rows) return;
    setSelected((cur) =>
      cur && rows.some((p) => p.key === cur)
        ? cur
        : rows.length > 0
          ? rows[0].key
          : "",
    );
  }, [preconditionsQuery.data]);

  // Pagination of the "Used by" tests list.
  const [testsPage, setTestsPage] = useViewState(
    profileId,
    "preconditions",
    "testsPage",
    0,
  );
  const [testsPageSize, setTestsPageSize] = useViewState(
    profileId,
    "preconditions",
    "testsPageSize",
    15,
  );
  useEffect(() => {
    setTestsPage(0);
  }, [selected]);
  const testsTotalPages = Math.max(1, Math.ceil(tests.length / testsPageSize));
  const testsSafePage = Math.min(testsPage, testsTotalPages - 1);
  const pageTests = tests.slice(
    testsSafePage * testsPageSize,
    (testsSafePage + 1) * testsPageSize,
  );

  // (The selected Precondition's linked tests now load via usePreconditionTests
  // above, refreshed by the same invalidation as the list.)

  return { profileId, confirm, beginSync, endSync, canSync, selected, setSelected, filter, setFilter, usageFilter, setUsageFilter, sortField, setSortField, sortDesc, setSortDesc, error, setError, notice, setNotice, showCreate, setShowCreate, showImport, setShowImport, showAdd, setShowAdd, detailKey, setDetailKey, detailVersion, setDetailVersion, editing, setEditing, syncing, setSyncing, summaryDraft, setSummaryDraft, descDraft, setDescDraft, condDraft, setCondDraft, typeDraft, setTypeDraft, condOpen, setCondOpen, descOpen, setDescOpen, detailsOpen, setDetailsOpen, listPage, setListPage, listPageSize, setListPageSize, testsPage, setTestsPage, testsPageSize, setTestsPageSize, preconditionsQuery, list, loading, listError, testsQuery, tests, selectedPre, isLocal, jiraBase, canLinkToJira, usageCounts, filtered, listTotalPages, listSafePage, pageList, testsTotalPages, testsSafePage, pageTests, openInJira };
}

export type PreconditionsState = ReturnType<typeof usePreconditionsState>;
