import { useMemo, useState } from "react";
import { Pager, assigneeLabel, useProfile, useViewState } from "@agile-suite/core";
import type { ComponentSummary, Profile, Settings, SortColumn } from "../api";
import { PERM_ADMINISTER_PROJECTS } from "../api/permissions";
import { COMPONENT_SORTS, componentWaiting, filterComponents } from "../lib/components";
import type { ComponentSort } from "../lib/components";
import { plural } from "../lib/format";
import { sortIssues } from "../lib/releases";
import { useGuardedSelect } from "../lib/unsavedEdit";
import { useComponentIssues, useComponentSummaries } from "../queries/components";
import { usePendingChanges } from "../queries/pending";
import { useDenial } from "../queries/permissions";
import { ComponentFormModal } from "./ComponentFormModal";
import { DeleteComponentModal } from "./DeleteComponentModal";
import { IssueDetailPanel } from "./IssueDetailPanel";
import { IssueTable } from "./IssueTable";
import "./components.css";

// XTM's page sizes: a project has tens of components, not thousands.
const PAGE_SIZES = [10, 20, 30, 40, 50];

// ComponentsView is the project's components, laid out as XTM's Components
// view is: a paged list beside the picked component's details and the issues
// filed under it, which open in the issue panel. Creating, editing and
// deleting are pending changes that Commit sends (#605).
export function ComponentsView() {
  const { activeId, activeProfile } = useProfile<Profile, Settings>();
  const denied = useDenial(activeId)(PERM_ADMINISTER_PROJECTS);
  const [filter, setFilter] = useViewState(activeId, "components", "filter", "");
  const [sortBy, setSortBy] = useViewState<ComponentSort>(activeId, "components", "sortBy", "name");
  const [selectedId, setSelectedId] = useViewState(activeId, "components", "selectedId", 0);
  const [selectedKey, setSelectedKey] = useViewState(activeId, "components", "selectedKey", "");
  const [sort, setSort] = useViewState<SortColumn | "">(activeId, "components", "sort", "");
  const [desc, setDesc] = useViewState(activeId, "components", "desc", false);
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState(PAGE_SIZES[0]);
  const [line, setLine] = useState("");
  const [form, setForm] = useState<{ component?: ComponentSummary } | null>(null);
  const [deleting, setDeleting] = useState<ComponentSummary | null>(null);

  // A profile switch clears what belonged to the old one, as ReleasesView does.
  const [madeFor, setMadeFor] = useState(activeId);
  if (madeFor !== activeId) {
    setMadeFor(activeId);
    setLine("");
    setForm(null);
    setDeleting(null);
    setPage(0);
  }

  const summaries = useComponentSummaries(activeId);
  const all = useMemo(() => summaries.data ?? [], [summaries.data]);
  const shown = useMemo(() => filterComponents(all, filter, sortBy), [all, filter, sortBy]);
  const pages = Math.max(1, Math.ceil(shown.length / pageSize));
  const safePage = Math.min(page, pages - 1);
  const pageItems = shown.slice(safePage * pageSize, (safePage + 1) * pageSize);
  // Keep a selection only while it still exists; else the first shown row.
  const selected = all.find((c) => c.id === selectedId) ?? shown[0];
  const issuesQuery = useComponentIssues(activeId, selected?.id ?? 0);
  const issues = useMemo(() => sortIssues(issuesQuery.data ?? [], sort, desc), [issuesQuery.data, sort, desc]);
  const pending = usePendingChanges(activeId);
  const waiting = useMemo(() => componentWaiting(pending.data ?? []), [pending.data]);
  const selectIssue = useGuardedSelect(selectedKey, setSelectedKey);
  const openIssue = issues.find((i) => i.key === selectedKey);
  const scoped = !!activeProfile?.scopeJql?.trim();
  const others = (c: ComponentSummary) => all.filter((o) => o.id !== c.id);

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

  function list() {
    if (summaries.isError) return <p className="error-text">{`Could not load the components: ${summaries.error.message}`}</p>;
    if (summaries.isLoading) return <p className="muted">Loading components…</p>;
    if (all.length === 0) return <p className="muted">This project has no components yet.</p>;
    if (shown.length === 0) return <p className="muted">No component matches that filter.</p>;
    return (
      <>
        <ul className="components-items">
          {pageItems.map((c) => {
            const queued = c.draft ? "Draft" : waiting.get(c.id);
            return (
              <li key={c.id}>
                <button
                  type="button"
                  className={`components-item${c.id === selected?.id ? " components-item-active" : ""}`}
                  aria-current={c.id === selected?.id ? "true" : undefined}
                  onClick={() => pick(c.id)}
                >
                  <div className="components-item-top">
                    <span className="components-item-name">
                      {c.name}
                      {queued && <span className="chip chip-draft components-item-chip">{queued}</span>}
                    </span>
                    <span className="components-item-count">{plural(c.issues, "issue", "issues")}</span>
                  </div>
                  <div className="components-item-meta muted">{c.description || "No description"}</div>
                </button>
              </li>
            );
          })}
        </ul>
        <Pager compact page={safePage} pageSize={pageSize} total={shown.length} pageSizeOptions={PAGE_SIZES} onPage={setPage}
          onPageSize={(n) => { setPageSize(n); setPage(0); }} />
      </>
    );
  }

  function detail(c: ComponentSummary) {
    const queued = waiting.get(c.id) ?? "";
    // A draft is local, so editing or dropping it needs no permission.
    const hold = c.draft ? "" : denied || (queued === "Deleting on Commit" ? "This component is deleting on Commit" : "");
    return (
      <div className="components-detail">
        <div className="components-detail-head">
          <h3 className="components-detail-name">{c.name}</h3>
          <div className="components-detail-actions">
            <button type="button" className="btn btn-ghost" aria-label={`Edit ${c.name}`} disabled={!!hold} title={hold || undefined} onClick={() => setForm({ component: c })}>Edit</button>
            <button type="button" className="btn btn-danger" aria-label={`Delete ${c.name}`} disabled={!!hold} title={hold || undefined} onClick={() => setDeleting(c)}>Delete</button>
          </div>
        </div>
        <dl className="components-detail-fields">
          <div><dt>Description</dt><dd>{c.description || <span className="muted">None</span>}</dd></div>
          <div><dt>Lead</dt><dd>{c.leadDisplayName || c.leadName || <span className="muted">Unassigned</span>}</dd></div>
          <div><dt>Default assignee</dt><dd>{assigneeLabel(c.assigneeType || "PROJECT_DEFAULT")}</dd></div>
        </dl>
        <h4 className="components-tests-head">Issues carrying this component</h4>
        {issuesQuery.isError ? (
          <p className="error-text">{`Could not load this component's issues: ${issuesQuery.error.message}`}</p>
        ) : issuesQuery.isLoading ? (
          <p className="muted">Loading issues…</p>
        ) : issues.length === 0 ? (
          <p className="muted">No synced issue carries this component.</p>
        ) : (
          <div className="components-issues">
            <IssueTable issues={issues} selectedKey={selectedKey} onSelect={selectIssue} sort={sort} desc={desc} onSort={onSort} />
          </div>
        )}
      </div>
    );
  }

  return (
    <section className="backlog components-page" aria-label="Components" data-tour="components-body">
      {line && (
        <div className="pending-banner" role="status" aria-label="Component outcome">
          <p>{line}</p>
        </div>
      )}
      {scoped && <p className="muted small">Counts are of the issues TAM has synced. The profile's scope JQL may leave some of a component's issues out.</p>}
      <div className="epics-body">
        <div className={`components-view${selected ? " components-with-detail" : ""}`}>
          <div className="components-list">
            <div className="components-list-head">
              <input className="components-search" value={filter} placeholder="Filter components…" aria-label="Filter components"
                onChange={(e) => { setFilter(e.target.value); setPage(0); }} />
              <select className="components-sort" aria-label="Sort by" value={sortBy} onChange={(e) => { setSortBy(e.target.value as ComponentSort); setPage(0); }}>
                {COMPONENT_SORTS.map((s) => <option key={s.value} value={s.value}>{s.label}</option>)}
              </select>
              <button type="button" className="btn btn-primary components-new" disabled={!!denied} title={denied || undefined} onClick={() => setForm({})}>New component</button>
            </div>
            {list()}
          </div>
          {selected && detail(selected)}
        </div>
        {openIssue && (
          <IssueDetailPanel key={openIssue.key} profileId={activeId} issue={openIssue} jiraUrl={activeProfile?.jiraUrl} onClose={() => setSelectedKey("")} />
        )}
      </div>

      {form && (
        <ComponentFormModal
          profileId={activeId}
          component={form.component}
          takenNames={(form.component ? others(form.component) : all).map((c) => c.name)}
          onClose={() => setForm(null)}
          onSaved={(id, sentence) => { pick(id); setLine(sentence); }}
        />
      )}
      {deleting && (
        <DeleteComponentModal profileId={activeId} component={deleting} others={others(deleting)} onClose={() => setDeleting(null)} onDeleted={setLine} />
      )}
    </section>
  );
}
