import type { PermissionView } from "../api";

// projectOf is the project key an issue key starts with: "PLAT" for
// "PLAT-431". A key with no dash has no project.
export function projectOf(issueKey: string): string {
  const i = issueKey.lastIndexOf("-");
  return i > 0 ? issueKey.slice(0, i) : "";
}

// denial is why an action is not offered, or "" when it is. An action is
// held back only when Jira has said the permission is missing: no answer
// yet, a project no sync asked about, and a permission Jira did not report
// all leave it open, because Jira still refuses at Commit and a gap in what
// TAM knows must not lock a user out of something they hold.
//
// issueKey picks the project the action is in. Without one, the action is
// in the profile's own project, which is where sprints and new issues live.
export function denial(view: PermissionView | undefined, perm: string, issueKey?: string): string {
  if (!view) return "";
  const project = issueKey ? projectOf(issueKey) : view.project;
  const have = view.byProject?.[project]?.[perm];
  if (have !== false) return "";
  return `Your Jira account does not have the "${view.names?.[perm] ?? perm}" permission in ${project}.`;
}
