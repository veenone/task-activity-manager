import { fieldLabel } from "../api";
import type { AuditEntry } from "../api";
import { useActivity } from "../queries/pending";
import { formatWhen } from "../lib/format";

interface Props {
  profileId: string;
  issueKey: string;
}

// describe turns one audit entry into a sentence: who did what to which
// field. A draft's create row always carries Field "create" (never empty),
// so its commit and discard entries are told apart by entityType, checked
// first, rather than by whether a field is set.
export function describe(a: AuditEntry): string {
  if (a.entityType === "link") {
    const [type = "", direction = "", key = ""] = a.field.split("|");
    const what = `${type} (${direction}) to ${key}`;
    switch (a.action) {
      case "link":
        return `${a.actor} added a link: ${what}`;
      case "commit":
        return `${a.actor} pushed the link ${what}`;
      case "discard":
        return `${a.actor} discarded the link ${what}`;
    }
  }
  if (a.entityType === "issue_create") {
    switch (a.action) {
      case "commit":
        return `${a.actor} pushed the draft to Jira`;
      case "discard":
        return `${a.actor} discarded the draft`;
      case "create":
        return `${a.actor} drafted this issue: ${a.afterVal}`;
    }
  }
  const field = a.field ? fieldLabel(a.field) : "";
  const change = a.field ? `${field}: ${a.beforeVal || "(none)"} to ${a.afterVal || "(none)"}` : "";
  switch (a.action) {
    case "edit":
      return `${a.actor} edited ${change}`;
    case "create":
      return `${a.actor} drafted this issue: ${a.afterVal}`;
    case "created":
      return `${a.actor} created it in Jira as ${a.afterVal} (was ${a.beforeVal})`;
    case "commit":
      return `${a.actor} pushed ${change}`;
    case "discard":
      return `${a.actor} discarded ${field}: back to ${a.afterVal || "(none)"}`;
    case "override":
      return `${a.actor} chose to override Jira's version ${a.afterVal}`;
    default:
      return `${a.actor} ${a.action}${change ? " " + change : ""}`;
  }
}

export function ActivityTab({ profileId, issueKey }: Props) {
  const activity = useActivity(profileId, issueKey);
  return (
    <div role="tabpanel" id="panel-activity" aria-labelledby="tab-activity" className="tab-panel">
      <div className="detail-section-head">
        <h3>Local activity</h3>
        <button type="button" className="btn btn-ghost" onClick={() => void activity.refetch()} disabled={activity.isFetching}>
          {activity.isFetching ? "Refreshing" : "Refresh"}
        </button>
      </div>
      {activity.isPending ? (
        <p className="muted">Loading activity</p>
      ) : activity.isError ? (
        <p className="error-text" data-testid="activity-error">
          Could not load the activity: {activity.error.message}{" "}
          <button type="button" className="btn btn-ghost" onClick={() => void activity.refetch()}>Retry</button>
        </p>
      ) : activity.data.length === 0 ? (
        <p className="muted">No local activity yet. Edits, commits, and discards land here.</p>
      ) : (
        <ul className="activity-list">
          {activity.data.map((a) => (
            <li key={a.id} className="activity-row">
              <span className="muted small">{formatWhen(a.occurredAt)}</span>
              <span>{describe(a)}</span>
              {a.note && <span className="muted small">{a.note}</span>}
            </li>
          ))}
        </ul>
      )}
      <p className="muted small detail-note">This is TAM's own trail, not Jira's history.</p>
    </div>
  );
}
