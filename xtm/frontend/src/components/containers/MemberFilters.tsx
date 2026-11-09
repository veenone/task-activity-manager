import { useState } from "react";
import { ContainerTestsMatching, errMsg } from "../../api";

type Labelled = { testKey: string; labels?: string[] };

// memberLabels lists the labels a container's tests carry, sorted.
export function memberLabels(rows: Labelled[]): string[] {
  return [...new Set(rows.flatMap((r) => r.labels ?? []))].sort();
}

// keepMembers applies the label filter (from the local cache) and the JQL
// match (the keys Jira answered with, or null for no JQL filter).
export function keepMembers<R extends Labelled>(rows: R[], label: string, jqlKeys: Set<string> | null): R[] {
  return rows.filter((r) => (!label || (r.labels ?? []).includes(label)) && (!jqlKeys || jqlKeys.has(r.testKey)));
}

// MemberFilters narrows an open Test Execution's tests the way Jira's own Test
// Execution page does: by label, which works offline, and by JQL, which asks
// Jira which of the execution's tests match (#446).
export function MemberFilters({
  profileId,
  containerKey,
  labels,
  label,
  onLabel,
  jqlOn,
  onJqlKeys,
}: {
  profileId: string;
  containerKey: string;
  labels: string[];
  label: string;
  onLabel: (label: string) => void;
  jqlOn: boolean;
  onJqlKeys: (keys: Set<string> | null) => void;
}) {
  const [jql, setJql] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const apply = async () => {
    if (!jql.trim()) return;
    setBusy(true);
    setError("");
    try {
      onJqlKeys(new Set(await ContainerTestsMatching(profileId, containerKey, jql.trim())));
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="member-filters">
      {labels.length > 0 && (
        <label className="member-filter">
          Label
          <select className="container-status-filter app-select" value={label} onChange={(e) => onLabel(e.target.value)}>
            <option value="">All labels</option>
            {labels.map((l) => (
              <option key={l} value={l}>
                {l}
              </option>
            ))}
          </select>
        </label>
      )}
      <form
        className="member-filter"
        onSubmit={(e) => {
          e.preventDefault();
          void apply();
        }}
      >
        <label className="member-filter">
          JQL
          <input
            className="detail-input member-jql"
            value={jql}
            placeholder="e.g. priority = High AND labels = smoke"
            onChange={(e) => setJql(e.target.value)}
          />
        </label>
        <button type="submit" className="btn" disabled={busy || !jql.trim()}>
          Apply JQL
        </button>
        {jqlOn && (
          <button
            type="button"
            className="btn btn-ghost"
            onClick={() => {
              setJql("");
              setError("");
              onJqlKeys(null);
            }}
          >
            Clear JQL
          </button>
        )}
      </form>
      {error && (
        <span role="alert" className="error-text">
          {error}
        </span>
      )}
    </div>
  );
}
