import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { errMsg, useConfirm, useProfile } from "@agile-suite/core";
import {
  CreateDashboard,
  DeleteDashboard,
  ListDashboards,
  ListJiraFilters,
  RefreshDashboard,
} from "../api";
import type { Dashboard, DashboardBucket, Profile, Settings } from "../api";
import { formatWhen, points, workDuration } from "../lib/format";

// The Dashboards view: the saved Jira filters this profile has pinned, and
// the figures each one last answered with.
//
// Everything on screen comes out of the local store. A dashboard holds a
// snapshot rather than a live query because it has to say what it counts
// with Jira unreachable, and say the same thing when it is reopened a week
// later; Refresh is what goes back to Jira, and it is always the reader's
// own decision.
//
// Nothing here writes to Jira. Reading a filter and counting what it
// matches is the whole of what a dashboard does.

// TOP_ROWS is how many rows of a ranking the panel draws. A filter can
// match thirty statuses, and a panel listing all of them stops being a
// ranking.
const TOP_ROWS = 6;

export function DashboardsView() {
  const { activeId } = useProfile<Profile, Settings>();
  const queries = useQueryClient();
  const { confirm } = useConfirm();
  const [filterId, setFilterId] = useState("");
  const [error, setError] = useState("");

  const dashboards = useQuery({
    queryKey: [activeId, "dashboards"],
    queryFn: () => ListDashboards(activeId),
    enabled: activeId !== "",
  });
  // The saved filters are Jira's, so this read can fail while the
  // dashboards below still draw. Its failure is reported where the picker
  // is, not over the figures.
  const filters = useQuery({
    queryKey: [activeId, "jiraFilters"],
    queryFn: () => ListJiraFilters(activeId),
    enabled: activeId !== "",
    retry: false,
  });

  const done = () => {
    void queries.invalidateQueries({ queryKey: [activeId, "dashboards"] });
  };

  const add = useMutation({
    mutationFn: () => {
      const chosen = (filters.data ?? []).find((f) => f.id === filterId);
      if (!chosen) throw new Error("Choose a saved filter first.");
      return CreateDashboard(activeId, chosen.name, chosen.id, chosen.jql);
    },
    onSuccess: () => {
      setError("");
      done();
    },
    onError: (e) => setError(errMsg(e)),
  });

  const refresh = useMutation({
    mutationFn: (id: string) => RefreshDashboard(activeId, id),
    onSuccess: () => {
      setError("");
      done();
    },
    // The figures on screen are the last ones read, so a failed refresh
    // leaves them exactly where they are and says why.
    onError: (e) => setError(errMsg(e)),
  });

  const remove = useMutation({
    mutationFn: (id: string) => DeleteDashboard(activeId, id),
    onSuccess: done,
    onError: (e) => setError(errMsg(e)),
  });

  async function askRemove(d: Dashboard) {
    if (await confirm({
      title: "Remove this dashboard?",
      message: `${d.name} stops being counted here. The filter itself stays in Jira.`,
      confirmLabel: "Remove",
      danger: true,
    })) {
      remove.mutate(d.id);
    }
  }

  const busy = add.isPending || refresh.isPending;
  const list = dashboards.data ?? [];

  return (
    <section className="dashboards-view" aria-label="Dashboards">
      <div className="dashboards-toolbar">
        <label className="muted small" htmlFor="dashboard-filter">Saved filter</label>
        <select
          id="dashboard-filter"
          className="detail-input"
          value={filterId}
          onChange={(e) => setFilterId(e.target.value)}
          disabled={busy || (filters.data ?? []).length === 0}
        >
          <option value="">Choose a filter</option>
          {(filters.data ?? []).map((f) => (
            <option key={f.id} value={f.id}>{f.name}</option>
          ))}
        </select>
        <button type="button" className="btn" onClick={() => add.mutate()} disabled={busy || filterId === ""}>
          Add dashboard
        </button>
        {filters.isError && (
          <span className="error-text">Saved filters could not be read: {errMsg(filters.error)}</span>
        )}
      </div>

      {error && <p className="error-text">{error}</p>}

      {dashboards.isPending ? (
        <p className="muted">Reading the dashboards</p>
      ) : list.length === 0 ? (
        <p className="muted">
          No dashboards yet. Pick one of your saved Jira filters above and TAM will count what it matches.
        </p>
      ) : (
        <div className="dashboard-grid">
          {list.map((d) => (
            <DashboardPanel
              key={d.id}
              dashboard={d}
              busy={busy}
              onRefresh={() => refresh.mutate(d.id)}
              onRemove={() => void askRemove(d)}
            />
          ))}
        </div>
      )}
    </section>
  );
}

function DashboardPanel({ dashboard, busy, onRefresh, onRemove }: {
  dashboard: Dashboard;
  busy: boolean;
  onRefresh: () => void;
  onRemove: () => void;
}) {
  const s = dashboard.snapshot;
  return (
    <section className="dashboard-panel" aria-label={dashboard.name}>
      <header className="dashboard-head">
        <h3 className="dashboard-title">{dashboard.name}</h3>
        <div className="dashboard-actions">
          <button type="button" className="btn btn-ghost" onClick={onRefresh} disabled={busy}>Refresh</button>
          <button type="button" className="btn btn-ghost" onClick={onRemove} disabled={busy}>Remove</button>
        </div>
      </header>
      <p className="muted small dashboard-when">
        {dashboard.refreshedAt ? `Counted ${formatWhen(dashboard.refreshedAt)}` : "Not counted yet"}
        {s.capped ? ". More issues match than one count reads, so these figures are a part of the filter." : ""}
      </p>
      <dl className="dashboard-figures">
        <div><dt>Issues</dt><dd>{s.total}</dd></div>
        <div><dt>Points</dt><dd>{points(s.points)}</dd></div>
        <div><dt>Estimated</dt><dd>{s.estimateSeconds > 0 ? workDuration(s.estimateSeconds) : "-"}</dd></div>
        <div><dt>Logged</dt><dd>{s.spentSeconds > 0 ? workDuration(s.spentSeconds) : "-"}</dd></div>
      </dl>
      <div className="dashboard-rankings">
        <Ranking title="By status" rows={s.byStatus} />
        <Ranking title="By type" rows={s.byType} />
        <Ranking title="By assignee" rows={s.byAssignee} />
      </div>
    </section>
  );
}

// Ranking is one count panel as a table, because that is what it is: two
// columns a reader compares down, and the one shape a screen reader can
// read a ranking out of.
function Ranking({ title, rows }: { title: string; rows: DashboardBucket[] }) {
  if (rows.length === 0) return <p className="muted small">{title}: nothing counted.</p>;
  return (
    <table className="dashboard-ranking" aria-label={title}>
      <caption className="muted small">{title}</caption>
      <thead>
        <tr><th scope="col">Name</th><th scope="col">Issues</th></tr>
      </thead>
      <tbody>
        {rows.slice(0, TOP_ROWS).map((r) => (
          <tr key={r.name}><td>{r.name}</td><td>{r.count}</td></tr>
        ))}
      </tbody>
    </table>
  );
}
