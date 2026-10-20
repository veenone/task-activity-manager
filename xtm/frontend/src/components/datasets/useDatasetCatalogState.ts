import { useEffect, useMemo, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { SyncDatasetCatalog, errMsg } from "../../api";
import { useProfile } from "../../contexts/ProfileContext";
import { useSync } from "../../contexts/SyncContext";
import { useViewState } from "../../lib/viewState";
import { useDatasetCatalog } from "../../queries/datasets";
import { keys } from "../../queries/keys";

export function useDatasetCatalogState() {
  const { activeId: profileId } = useProfile();
  const queryClient = useQueryClient();
  const { beginSync, endSync, failSync, canSync } = useSync();
  const [search, setSearchState] = useViewState(
    profileId,
    "datasets",
    "search",
    "",
  );
  const [scope, setScopeState] = useViewState(
    profileId,
    "datasets",
    "scope",
    "",
  );
  const [ownership, setOwnershipState] = useViewState(
    profileId,
    "datasets",
    "ownership",
    "",
  );
  const [page, setPage] = useViewState(profileId, "datasets", "page", 0);
  const [pageSize, setPageSizeState] = useViewState(
    profileId,
    "datasets",
    "pageSize",
    25,
  );
  const [expandedKey, setExpandedKey] = useViewState(
    profileId,
    "datasets",
    "expandedKey",
    "",
  );
  const [syncing, setSyncing] = useState(false);
  const [syncError, setSyncError] = useState("");

  const query = useMemo(
    () => ({
      search,
      scope,
      ownership,
      limit: pageSize,
      offset: page * pageSize,
    }),
    [search, scope, ownership, page, pageSize],
  );
  const catalog = useDatasetCatalog(profileId, query);
  const total = catalog.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const safePage = Math.min(page, totalPages - 1);

  useEffect(() => {
    if (safePage !== page) setPage(safePage);
  }, [page, safePage, setPage]);

  function resetPageAnd(run: () => void) {
    run();
    setPage(0);
  }

  async function sync() {
    if (!profileId || !canSync || syncing) return;
    setSyncError("");
    setSyncing(true);
    beginSync({
      initialProgress: {
        phase: "",
        stage: "Finding tests",
        fetched: 0,
        total: 0,
        done: false,
      },
      clearError: true,
    });
    try {
      await SyncDatasetCatalog(profileId);
      await queryClient.invalidateQueries({ queryKey: keys.datasets(profileId) });
    } catch (error) {
      const message = errMsg(error);
      setSyncError(message);
      failSync(message);
    } finally {
      setSyncing(false);
      endSync();
    }
  }

  return {
    profileId,
    catalog,
    search,
    setSearch: (value: string) => resetPageAnd(() => setSearchState(value)),
    scope,
    setScope: (value: string) => resetPageAnd(() => setScopeState(value)),
    ownership,
    setOwnership: (value: string) =>
      resetPageAnd(() => setOwnershipState(value)),
    page: safePage,
    setPage,
    pageSize,
    setPageSize: (value: number) =>
      resetPageAnd(() => setPageSizeState(value)),
    expandedKey,
    toggleExpanded: (key: string) =>
      setExpandedKey((current) => (current === key ? "" : key)),
    syncing,
    syncError,
    canSync,
    sync,
  };
}

export type DatasetCatalogState = ReturnType<typeof useDatasetCatalogState>;
