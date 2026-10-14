import { ListMatchingKeys, ExportTests, CreateSavedView, DeleteSavedView, errMsg } from "../../api";
import type { TestQuery } from "../../api";
import type { SortCol } from "./columns";
import type { TestTableProps, TestTableState } from "./useTestTableState";

// useTestTableActions is what the grid does beyond its own state: save, apply
// and delete a saved view, export the filtered tests, and select every test
// matching the filter.
export function useTestTableActions(s: TestTableState, p: TestTableProps) {
  const { profileId, onSelectAllMatching, prompt, notice, search, setSearch, debouncedSearch, status, setStatus, execType, setExecType, review, setReview, sortBy, setSortBy, desc, setDesc, setError, selectingAll, setSelectingAll, setSelectAllError, savedViews, setSavedViews, activeView, setActiveView } = s;
  const { folderId, containerKey, component } = p;

  async function saveView() {
    const name = await prompt({
      title: "Save current filter as",
      placeholder: "View name",
      submitLabel: "Save",
    });
    if (!name || !name.trim()) return;
    const query = JSON.stringify({ search, status, execType, review, sortBy, desc });
    setError("");
    try {
      const v = await CreateSavedView(profileId, name.trim(), query);
      setSavedViews((prev) => [v, ...prev]);
      setActiveView(v.id);
    } catch (e) {
      setError(errMsg(e));
    }
  }

  function applyView(id: string) {
    setActiveView(id);
    if (!id) return;
    const v = savedViews.find((x) => x.id === id);
    if (!v) return;
    try {
      const q = JSON.parse(v.query) as Partial<{
        search: string;
        status: string;
        execType: string;
        review: string;
        sortBy: SortCol;
        desc: boolean;
      }>;
      setSearch(q.search ?? "");
      setStatus(q.status ?? "");
      setExecType(q.execType ?? "");
      setReview(q.review ?? "");
      setSortBy(q.sortBy ?? "key");
      setDesc(q.desc ?? false);
    } catch {
      // Ignore a malformed saved query rather than break the grid.
    }
  }

  async function deleteActiveView() {
    if (!activeView) return;
    setError("");
    try {
      await DeleteSavedView(profileId, activeView);
      setSavedViews((prev) => prev.filter((v) => v.id !== activeView));
      setActiveView("");
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function exportTests() {
    try {
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
        limit: 0,
        offset: 0,
      };
      const path = await ExportTests(profileId, q);
      if (path) await notice({ title: "Tests exported", message: path });
    } catch (e) {
      await notice({ title: "Export failed", message: errMsg(e), tone: "error" });
    }
  }

  async function selectAllMatching() {
    if (selectingAll) return;
    setSelectingAll(true);
    setSelectAllError("");
    try {
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
        limit: 0,
        offset: 0,
      };
      const keys = await ListMatchingKeys(profileId, q);
      onSelectAllMatching(keys);
    } catch (e) {
      setSelectAllError(errMsg(e));
    } finally {
      setSelectingAll(false);
    }
  }

  return { saveView, applyView, deleteActiveView, exportTests, selectAllMatching };
}

export type TestTableActions = ReturnType<typeof useTestTableActions>;
