import { Suspense, lazy, useMemo, useState } from "react";
import { MultiSelect, useProfile, useViewState } from "@agile-suite/core";
import { ASSIGNEE_NONE } from "../api";
import type { Profile, Settings } from "../api";
import { useDependencyGraph } from "../queries/dependencies";
import { useFilterOptions, useSprints } from "../queries/issues";
import { useOpenSprints } from "../queries/boards";
import { useLinkTypes } from "../queries/pending";
import { useDenial } from "../queries/permissions";
import { PERM_LINK_ISSUES } from "../api/permissions";
import { DEP_LAYOUT, canvasData, chain, chainExport, defaultTypes, edgeId, visibleGraph, wholeExport } from "../lib/dependencyGraph";
import { useGuardedSelect } from "../lib/unsavedEdit";
import { IssueDetailPanel } from "./IssueDetailPanel";
import { LinkPanel } from "./LinkPanel";
import { NewLinkDialog } from "./NewLinkDialog";
import { useDependencyEdits } from "./useDependencyEdits";
import { useDependencyExport } from "./useDependencyExport";

// G6 is loaded with the view, not with the app, the way PIE loads it.
const G6Canvas = lazy(() => import("@agile-suite/core/src/g6/G6Canvas"));

const count = (n: number, word: string) => `${n} ${word}${n === 1 ? "" : "s"}`;

// DependenciesView is the project's linked issues as a graph (#612): Blocks
// links by default, filtered by sprint, status, assignee and link type.
// Picking an issue lights what it waits on and what waits on it and opens
// the issue panel; dragging from one issue to another links them; picking a
// link removes it or changes its type. Every edit is journaled for Commit.
export function DependenciesView() {
  const { activeId, activeProfile } = useProfile<Profile, Settings>();
  const graph = useDependencyGraph(activeId);
  const linkTypes = useLinkTypes(activeId);
  const sprints = useSprints(activeId);
  const options = useFilterOptions(activeId);
  const openSprints = useOpenSprints(activeId);
  const [types, setTypes] = useViewState<string[] | null>(activeId, "dependencies", "types", null);
  const [sprintId, setSprintId] = useViewState(activeId, "dependencies", "sprintId", "");
  const [status, setStatus] = useViewState(activeId, "dependencies", "status", "");
  const [assignee, setAssignee] = useViewState(activeId, "dependencies", "assignee", "");
  const [selectedKey, setSelectedKey] = useViewState(activeId, "dependencies", "selectedKey", "");
  const [selectedLink, setSelectedLink] = useState("");
  const selectIssue = useGuardedSelect(selectedKey, setSelectedKey);
  const edits = useDependencyEdits(activeId, graph.data);
  const denial = useDenial(activeId);
  const exporter = useDependencyExport(activeId);

  const [filtersFor, setFiltersFor] = useState(activeId);
  if (filtersFor !== activeId) {
    setFiltersFor(activeId);
    setSelectedLink("");
  }

  const typeNames = useMemo(() => {
    const names = (linkTypes.data ?? []).map((t) => t.name);
    for (const l of graph.data?.links ?? []) if (!names.includes(l.type)) names.push(l.type);
    return names;
  }, [linkTypes.data, graph.data]);
  const chosenTypes = types ?? defaultTypes(typeNames);
  const visible = useMemo(
    () => (graph.data ? visibleGraph(graph.data, { types: chosenTypes, sprintId, status, assignee }) : null),
    [graph.data, chosenTypes, sprintId, status, assignee],
  );
  const focus = visible?.keys.has(selectedKey) ? selectedKey : null;
  const drawn = useMemo(
    () => (graph.data && visible ? canvasData(graph.data, visible, focus, selectedLink) : { nodes: [], edges: [] }),
    [graph.data, visible, focus, selectedLink],
  );
  const link = visible?.links.find((l) => edgeId(l) === selectedLink);
  const selected = graph.data?.issues.find((i) => i.key === selectedKey);
  const around = focus && visible ? chain(visible.links, focus) : null;

  const pickIssue = (key: string) => {
    setSelectedLink("");
    selectIssue(key);
  };
  const pickLink = (id: string) => {
    setSelectedKey("");
    setSelectedLink(id);
  };

  return (
    <section className="backlog" aria-label="Dependencies" data-tour="dependencies-body">
      <div className="filter-bar">
        <select className="detail-input filter-status" aria-label="Status" value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="">All statuses</option>
          {(options.data?.statuses ?? []).map((s) => (
            <option key={s} value={s}>{s}</option>
          ))}
        </select>
        <select className="detail-input filter-assignee" aria-label="Assignee" value={assignee} onChange={(e) => setAssignee(e.target.value)}>
          <option value="">All assignees</option>
          <option value={ASSIGNEE_NONE}>Unassigned</option>
          {(options.data?.assignees ?? []).filter(Boolean).map((a) => (
            <option key={a} value={a}>{a}</option>
          ))}
        </select>
        <select className="detail-input filter-sprint" aria-label="Sprint" value={sprintId} onChange={(e) => setSprintId(e.target.value)}>
          <option value="">All sprints</option>
          {(sprints.data ?? []).map((s) => (
            <option key={s.id} value={s.id}>{s.name}</option>
          ))}
        </select>
        <MultiSelect
          title="Link types"
          allLabel="No link types"
          options={typeNames.map((t) => ({ value: t, label: t }))}
          selected={chosenTypes}
          onChange={setTypes}
        />
        <button
          type="button"
          className="btn filter-export"
          title="Export the selected issue and its dependencies"
          disabled={!focus || !graph.data || !visible || exporter.busy}
          onClick={() => graph.data && visible && focus && void exporter.run(chainExport(graph.data, visible, focus, chosenTypes))}
        >
          Export chain
        </button>
        <button
          type="button"
          className="btn filter-export"
          title="Export every issue in the profile and how they are linked, whatever the filters"
          disabled={!graph.data || exporter.busy}
          onClick={() => graph.data && void exporter.run(wholeExport(graph.data))}
        >
          Export all
        </button>
      </div>
      {exporter.saved && <p className="muted small filter-note">{`Saved to ${exporter.saved}${exporter.note ? `, ${exporter.note}` : ""}`}</p>}
      {exporter.error && <p className="error-text filter-note">The dependencies could not be exported: {exporter.error}</p>}

      <p className="muted epics-summary">
        {visible ? `${count(visible.keys.size, "issue")}, ${count(visible.links.length, "link")}` : ""}
        {around && ` · ${focus} waits on ${around.up.size}, ${around.down.size} wait on it`}
      </p>

      <div className="epics-body">
        <div className="epics-tree-pane deps-graph-pane">
          {graph.isError ? (
            <p className="error-text">Could not load the dependencies: {graph.error.message}</p>
          ) : graph.isLoading ? (
            <p className="muted">Loading dependencies</p>
          ) : !visible || visible.links.length === 0 ? (
            <p className="muted">
              {graph.data?.links.length ? "No links match this filter." : "No linked issues cached yet. Use Sync in the topbar."}
            </p>
          ) : (
            <Suspense fallback={<p className="muted">Loading the graph</p>}>
              <G6Canvas
                nodes={drawn.nodes}
                edges={drawn.edges}
                focus={focus}
                layout={DEP_LAYOUT}
                onSelect={pickIssue}
                onSelectEdge={pickLink}
                onConnect={edits.connect}
                label="Issue dependencies. Drag from one issue to another to link them."
              />
            </Suspense>
          )}
        </div>
        {selected ? (
          <IssueDetailPanel
            key={selected.key}
            profileId={activeId}
            issue={selected}
            jiraUrl={activeProfile?.jiraUrl}
            sprints={openSprints.data}
            emptyNote="No sprints yet, sync a board first"
            onClose={() => setSelectedKey("")}
          />
        ) : link ? (
          <LinkPanel
            key={selectedLink}
            link={link}
            types={linkTypes.data ?? []}
            busy={edits.busy}
            denied={denial(PERM_LINK_ISSUES, link.owner)}
            onRemove={() => edits.removeLink(link, () => {})}
            onRetype={(t) => void edits.retypeLink(link, t, () => setSelectedLink(""))}
            onDiscard={() => edits.discardLink(link, () => setSelectedLink(""))}
            onClose={() => setSelectedLink("")}
          />
        ) : null}
      </div>

      {edits.connecting && (
        <NewLinkDialog
          source={edits.connecting.source}
          target={edits.connecting.target}
          types={linkTypes.data ?? []}
          busy={edits.busy}
          onAdd={edits.addLink}
          onClose={edits.cancelConnect}
        />
      )}
    </section>
  );
}
