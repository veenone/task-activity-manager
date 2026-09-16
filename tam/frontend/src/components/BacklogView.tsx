import { IssueListView } from "./IssueListView";

// BacklogView is the Backlog tab: the project's whole issue list, with create
// and import enabled. The reusable grid, filter bar and pager live in
// IssueListView; Assigned to me (a later tab) reuses it with a narrower
// baseQuery and its own copy.
export function BacklogView() {
  return (
    <IssueListView
      viewId="backlog"
      label="Backlog"
      showCreate
      showImport
      emptyNote="No issues cached yet. Use Sync in the topbar to pull this project's issues."
    />
  );
}
