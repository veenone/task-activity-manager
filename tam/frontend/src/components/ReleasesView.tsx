import { useMemo, useState } from "react";
import { Menu, announce, errMsg, useNotice, useProfile, useViewState } from "@agile-suite/core";
import type { MenuItem } from "@agile-suite/core";
import type { Profile, Settings, SortColumn, VersionSummary } from "../api";
import { PERM_ADMINISTER_PROJECTS } from "../api/permissions";
import { useSync } from "../contexts/SyncContext";
import { RELEASE_FILTERS, filterReleases, isOpen, releaseWaiting, sortIssues, unfinishedIssues } from "../lib/releases";
import type { ReleaseFilter } from "../lib/releases";
import { useGuardedSelect } from "../lib/unsavedEdit";
import { usePendingChanges } from "../queries/pending";
import { useDenial } from "../queries/permissions";
import { useArchiveRelease, useVersionIssues, useVersionSummaries } from "../queries/releases";
import { DeleteReleaseModal } from "./DeleteReleaseModal";
import { IssueDetailPanel } from "./IssueDetailPanel";
import { IssueTable } from "./IssueTable";
import { ReleaseDialog } from "./ReleaseDialog";
import { ReleaseFormModal } from "./ReleaseFormModal";
import { ReleaseList } from "./ReleaseList";
import "./releases.css";

// A draft release is not in Jira, so it cannot be released or archived yet.
const DRAFT_HINT = "Commit this release first";

// ReleasesView is the project's releases (Jira fix versions): each one's
// dates, status and progress, and the issues in the one picked. Creating,
// editing, releasing, archiving and deleting are pending changes that Commit
// sends, the way sprint changes are (#605).
export function ReleasesView() {
  const { activeId, activeProfile } = useProfile<Profile, Settings>();
  const { runQuietLock } = useSync();
  const { notice } = useNotice();
  const denied = useDenial(activeId)(PERM_ADMINISTER_PROJECTS);
  const [filter, setFilter] = useViewState<ReleaseFilter>(activeId, "releases", "filter", "unreleased");
  const [selectedId, setSelectedId] = useViewState(activeId, "releases", "selectedId", 0);
  const [selectedKey, setSelectedKey] = useViewState(activeId, "releases", "selectedKey", "");
  const [sort, setSort] = useViewState<SortColumn | "">(activeId, "releases", "sort", "");
  const [desc, setDesc] = useViewState(activeId, "releases", "desc", false);
  const [line, setLine] = useState("");
  const [form, setForm] = useState<{ release?: VersionSummary } | null>(null);
  const [releasing, setReleasing] = useState<VersionSummary | null>(null);
  const [deleting, setDeleting] = useState<VersionSummary | null>(null);

  // Everything above belongs to the profile it was picked for, so a switch
  // clears it in the render that first sees the new id, as SprintsView does.
  const [madeFor, setMadeFor] = useState(activeId);
  if (madeFor !== activeId) {
    setMadeFor(activeId);
    setLine("");
    setForm(null);
    setReleasing(null);
    setDeleting(null);
  }

  const summaries = useVersionSummaries(activeId);
  const all = useMemo(() => summaries.data ?? [], [summaries.data]);
  const shown = filterReleases(all, filter);
  const selected = shown.find((v) => v.id === selectedId) ?? shown[0];
  const issuesQuery = useVersionIssues(activeId, selected?.id ?? 0);
  const issues = useMemo(() => sortIssues(issuesQuery.data ?? [], sort, desc), [issuesQuery.data, sort, desc]);
  const pending = usePendingChanges(activeId);
  const waiting = useMemo(() => releaseWaiting(pending.data ?? []), [pending.data]);
  const archive = useArchiveRelease(activeId, runQuietLock);
  const selectIssue = useGuardedSelect(selectedKey, setSelectedKey);
  const openIssue = issues.find((i) => i.key === selectedKey);
  const scoped = !!activeProfile?.scopeJql?.trim();

  function pick(id: number) {
    setSelectedId(id);
    setSelectedKey("");
  }

  function onSort(column: SortColumn) {
    if (sort === column) setDesc(!desc);
    else {
      setSort(column);
      setDesc(false);
    }
  }

  function toggleArchive(v: VersionSummary) {
    archive.mutate({ id: v.id, archived: !v.archived }, {
      onSuccess: () => {
        const sentence = `${v.name} will be ${v.archived ? "unarchived" : "archived"} on Commit.`;
        announce(sentence);
        setLine(sentence);
      },
      onError: (e) => void notice({ title: "The release was not changed", message: errMsg(e), tone: "error" }),
    });
  }

  function section(v: VersionSummary) {
    const queued = waiting.get(v.id) ?? "";
    const busyRow = queued !== "" && queued !== "Edited, not committed";
    // Release and archive need the release in Jira, the permission, and
    // nothing else queued for it; edit and delete of a draft are local.
    const releaseHold = v.draft ? DRAFT_HINT : denied || (busyRow ? `This release is ${queued.toLowerCase()}` : "");
    const editHold = v.draft ? "" : denied;
    const more: MenuItem[] = [
      {
        key: "archive",
        label: v.archived ? "Unarchive" : "Archive",
        disabled: !!releaseHold || archive.isPending,
        title: releaseHold || undefined,
        onClick: () => toggleArchive(v),
      },
      { key: "delete", label: "Delete…", danger: true, disabled: !!editHold, title: editHold || undefined, onClick: () => setDeleting(v) },
    ];
    return (
      <div className="release-section">
        <div className="release-section-head">
          <h3 className="release-section-name">{v.name}</h3>
          <span className="release-section-actions">
            <button type="button" className="btn" disabled={!!editHold} title={editHold || undefined} onClick={() => setForm({ release: v })}>Edit</button>
            <button
              type="button"
              className="btn"
              disabled={!!releaseHold || !isOpen(v)}
              title={releaseHold || (isOpen(v) ? undefined : "Already released")}
              onClick={() => setReleasing(v)}
            >
              Release…
            </button>
            <Menu label="More" items={more} align="right" triggerLabel={`More actions on ${v.name}`} triggerClassName="btn" />
          </span>
        </div>
        {issuesQuery.isError ? (
          <p className="error-text">{`Could not load this release's issues: ${issuesQuery.error.message}`}</p>
        ) : issuesQuery.isLoading ? (
          <p className="muted">Loading the issues</p>
        ) : issues.length === 0 ? (
          <p className="muted">No issue TAM holds is in this release.</p>
        ) : (
          <IssueTable issues={issues} selectedKey={selectedKey} onSelect={selectIssue} sort={sort} desc={desc} onSort={onSort} />
        )}
      </div>
    );
  }

  function body() {
    if (summaries.isError) {
      return (
        <p className="error-text">
          Could not load the releases: {summaries.error.message}{" "}
          <button type="button" className="btn" onClick={() => void summaries.refetch()}>Retry</button>
        </p>
      );
    }
    if (summaries.isLoading) return <p className="muted">Loading the releases</p>;
    if (all.length === 0) return <p className="muted">This project has no releases yet. New release drafts the first one.</p>;
    if (shown.length === 0) return <p className="muted">No release matches this filter.</p>;
    return (
      <>
        <ReleaseList releases={shown} selectedId={selected?.id ?? 0} onSelect={pick} waiting={waiting} />
        {selected && section(selected)}
      </>
    );
  }

  return (
    <section className="backlog" aria-label="Releases" data-tour="releases-body">
      <div className="board-head">
        <h2 className="board-head-name">Releases</h2>
        <label className="board-picker">
          <span>Show</span>
          <select aria-label="Show" value={filter} onChange={(e) => { setFilter(e.target.value as ReleaseFilter); setSelectedKey(""); }}>
            {RELEASE_FILTERS.map((f) => <option key={f.id} value={f.id}>{f.label}</option>)}
          </select>
        </label>
        <div className="board-head-actions">
          <button type="button" className="btn" disabled={!!denied} title={denied || undefined} onClick={() => setForm({})}>New release</button>
        </div>
      </div>

      {line && (
        <div className="pending-banner" role="status" aria-label="Release outcome">
          <p>{line}</p>
        </div>
      )}
      {scoped && <p className="muted small">Counts are of the issues TAM has synced. The profile's scope JQL may leave some of a release's issues out.</p>}

      <div className="epics-body">
        <div className="epics-tree-pane release-pane">{body()}</div>
        {openIssue && (
          <IssueDetailPanel key={openIssue.key} profileId={activeId} issue={openIssue} jiraUrl={activeProfile?.jiraUrl} onClose={() => setSelectedKey("")} />
        )}
      </div>

      {form && (
        <ReleaseFormModal
          profileId={activeId}
          release={form.release}
          onClose={() => setForm(null)}
          onSaved={(id, sentence) => { pick(id); setLine(sentence); }}
        />
      )}
      {releasing && (
        <ReleaseDialog
          profileId={activeId}
          release={releasing}
          releases={all}
          unfinished={unfinishedIssues(issuesQuery.data ?? [])}
          scoped={scoped}
          onClose={() => setReleasing(null)}
          onReleased={setLine}
        />
      )}
      {deleting && (
        <DeleteReleaseModal profileId={activeId} release={deleting} releases={all} onClose={() => setDeleting(null)} onDeleted={setLine} />
      )}
    </section>
  );
}
