import { errMsg } from "../api";
import { RollupBreakdownModal } from "./RollupBreakdownModal";
import { AddTestsModal } from "./AddTestsModal";
import { BugsPanel } from "./BugsPanel";
import { JUnitImportModal } from "./JUnitImportModal";
import { JUnitNewExecModal } from "./JUnitNewExecModal";
import { CreateBugModal } from "./CreateBugModal";
import { LinkBugPicker } from "./LinkBugPicker";
import { TestDetail } from "./TestDetail";
import { useNotice } from "./useNotice";
import { useContainerState } from "./containers/useContainerState";
import { useContainerActions } from "./containers/useContainerActions";
import type { ContainerViewProps } from "./containers/useContainerActions";
import { ContainerPickerBar } from "./containers/ContainerPickerBar";
import { ContainerFilterBar } from "./containers/ContainerFilterBar";
import { ContainerCard } from "./containers/ContainerCard";
import { MemberTable } from "./containers/MemberTable";

// ContainersView manages Test Sets / Plans / Executions (FR-13.7 + container
// CRUD): pick a kind and a container, see its member Tests with run status, and
// create / rename / delete. Computed from the local store; recomputes when the
// profile changes or a sync/commit invalidates the query cache.
//
// The view composes containers/: useContainerState and useContainerActions
// hold what it knows and does, and the picker bar, filter bar, card and member
// table render it.
export function ContainersView(p: ContainerViewProps) {
  const s = useContainerState();
  const a = useContainerActions(s, p);
  const { notice } = useNotice();
  const {
    kind, selected, bugFor, setBugFor, linkBugFor, setLinkBugFor, mode, setMode, error, showAdd,
    setShowAdd, showJUnitImport, setShowJUnitImport, showJUnitNewExec, setShowJUnitNewExec,
    breakdownStatus, setBreakdownStatus, breakdown, breakdownLoading, detailKey, setDetailKey,
    detailVersion, containers, listError, boardQuery, board, memberRuns, loading, kindLabel,
    selectedContainer,
  } = s;
  const { onChanged, jiraUrl, onOpenTest } = p;

  return (
    <div
      data-tour="plans-body"
      className={`board${mode === "bugs" ? " board--bugs" : ""}${detailKey && mode !== "bugs" ? " board--with-exec-detail" : ""}`}
    >
      <div className="board-exec-body">
      <div className="containers-mode" data-tour="plans-tools">
        <button
          className={`seg-btn${mode === "containers" ? " seg-btn-active" : ""}`}
          onClick={() => setMode("containers")}
        >
          Containers
        </button>
        <button
          className={`seg-btn${mode === "bugs" ? " seg-btn-active" : ""}`}
          onClick={() => setMode("bugs")}
        >
          Bugs
        </button>
      </div>

      {mode === "bugs" ? (
        <BugsPanel
          jiraUrl={jiraUrl ?? ""}
          onOpenTest={onOpenTest ?? (() => {})}
        />
      ) : (
        <>
      <ContainerPickerBar s={s} a={a} p={p} />

      <ContainerFilterBar s={s} a={a} p={p} />

      {(listError || error || boardQuery.error) && (
        <div className="error-text">
          {listError || error || errMsg(boardQuery.error)}
        </div>
      )}

      {!loading && containers.length === 0 && (
        <p className="muted">
          You don't have any {kindLabel}s yet. Create one, run a sync, or
          generate sample data.
        </p>
      )}

      <ContainerCard s={s} a={a} p={p} />

      <MemberTable s={s} a={a} p={p} />
        </>
      )}

      {showAdd && selectedContainer && (
        <AddTestsModal
          containerKey={selected}
          existingKeys={(board?.rows ?? []).map((r) => r.testKey)}
          onCancel={() => setShowAdd(false)}
          onDone={() => {
            setShowAdd(false);
            onChanged();
          }}
        />
      )}

      {bugFor && (
        <CreateBugModal
          testKey={bugFor.testKey}
          testSummary={bugFor.summary}
          execKey={selected}
          onClose={() => setBugFor(null)}
          onCreated={() => {
            setBugFor(null);
            onChanged();
          }}
        />
      )}

      {linkBugFor && selected && (
        <LinkBugPicker
          execKey={selected}
          testKey={linkBugFor}
          existingKeys={memberRuns.get(linkBugFor)?.defects ?? []}
          onClose={() => setLinkBugFor(null)}
          onLinked={onChanged}
        />
      )}

      {showJUnitImport && selected && kind === "testexec" && (
        <JUnitImportModal
          execKey={selected}
          onCancel={() => setShowJUnitImport(false)}
          onApplied={(succeeded, failed) => {
            setShowJUnitImport(false);
            onChanged();
            notice({
              title: "JUnit import applied",
              message:
                `${succeeded} result${succeeded !== 1 ? "s" : ""} queued. Commit them from the Pending list.` +
                (failed > 0 ? ` (${failed} failed)` : ""),
            });
          }}
        />
      )}

      {showJUnitNewExec && kind === "testexec" && (
        <JUnitNewExecModal
          onCancel={() => setShowJUnitNewExec(false)}
          onApplied={(result) => {
            setShowJUnitNewExec(false);
            onChanged();
            notice({
              title: "Execution created",
              message:
                `Created ${result.execKey}: ${result.created} test${result.created !== 1 ? "s" : ""} created, ` +
                `${result.allocated} allocated, ${result.resultsSet} result${result.resultsSet !== 1 ? "s" : ""} set` +
                (result.failed && result.failed.length > 0
                  ? ` (${result.failed.length} failed)`
                  : "") +
                ". Queued, commit it from the Pending list.",
            });
          }}
        />
      )}

      {breakdownStatus !== null && (
        <RollupBreakdownModal
          kindLabel={kindLabel}
          containerKey={selected}
          status={breakdownStatus}
          members={breakdown ?? []}
          loading={breakdownLoading}
          onClose={() => setBreakdownStatus(null)}
        />
      )}

      </div>

      {detailKey && mode !== "bugs" && (
        <TestDetail
          testKey={detailKey}
          version={detailVersion}
          pendingForTest={[]}
          folders={[]}
          jiraUrl={jiraUrl ?? ""}
          readOnly
          // Only an execution can hold a dataset of its own; a set or a plan
          // has no run to hang one off.
          containerKey={kind === "testexec" ? selected : ""}
          onClose={() => setDetailKey(null)}
          onEdited={() => {}}
        />
      )}
    </div>
  );
}
