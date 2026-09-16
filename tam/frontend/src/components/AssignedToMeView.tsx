import { useCallback, useState } from "react";
import { useProfile } from "@agile-suite/core";
import type { Issue, Profile, Settings } from "../api";
import { useJiraDisplayName, useJiraUsername } from "../queries/issues";
import { useSync } from "../contexts/SyncContext";
import { useModal } from "../modals";
import { IssueListView } from "./IssueListView";

// AssignedToMeView is the Assigned to me tab: the Backlog grid narrowed to
// the connected Jira user's issues. IssueListView carries the filter bar,
// grid, pager and detail panel; this view only knows who "me" is, says so
// above the grid, and reads IssueListView's own query result (through
// onPage) for the header count and the stale-rows note, rather than firing
// a second query for either.
export function AssignedToMeView() {
  const { activeId } = useProfile<Profile, Settings>();
  const username = useJiraUsername(activeId);
  const displayName = useJiraDisplayName(activeId);
  const { canSync, runSync } = useSync();
  const { openModal } = useModal();
  const [total, setTotal] = useState(0);
  const [staleRows, setStaleRows] = useState(false);

  const onPage = useCallback((pageTotal: number, rows: Issue[]) => {
    setTotal(pageTotal);
    setStaleRows(rows.some((r) => !r.assigneeName));
  }, []);

  // Neither setting resolves until GetProfileSetting answers, and a query
  // that went out with assigneeName "" would list the whole project rather
  // than nothing, so the grid stays unmounted until both are known.
  if (username.isPending || displayName.isPending) return null;

  const me = username.data ?? "";
  const myDisplayName = displayName.data ?? "";

  if (!me) {
    return (
      <section className="backlog" aria-label="Assigned to me">
        <p className="muted backlog-empty">
          TAM does not know your Jira user yet. Sync once or test the connection.
        </p>
        <div className="assigned-empty-actions">
          <button type="button" className="btn" disabled={!canSync} onClick={() => void runSync(false)}>
            Sync
          </button>
          <button type="button" className="btn" onClick={() => openModal("profiles")}>
            Test connection
          </button>
        </div>
      </section>
    );
  }

  return (
    <div className="assigned-view">
      <p className="assigned-header">
        {`Assigned to ${me}${myDisplayName ? ` (${myDisplayName})` : ""} · ${total} ${total === 1 ? "issue" : "issues"}`}
      </p>
      {staleRows && (
        <p className="muted assigned-note">Showing matches by display name until the next sync</p>
      )}
      <IssueListView
        viewId="assigned"
        label="Assigned to me"
        baseQuery={{ assigneeName: me, assigneeDisplayName: myDisplayName }}
        emptyNote="No issues assigned to you in this project yet."
        onPage={onPage}
      />
    </div>
  );
}
