import { useState } from "react";
import { ContainerTestsMatching, errMsg } from "../../api";
import { usePrompt } from "../usePrompt";

type Labelled = { testKey: string; labels?: string[] };

// memberLabels lists the labels a container's tests carry, sorted.
export function memberLabels(rows: Labelled[]): string[] {
  return [...new Set(rows.flatMap((r) => r.labels ?? []))].sort();
}

// keepMembers applies the label filter (from the local cache) and the JQL
// match (the keys Jira answered with, or null for no JQL filter).
export function keepMembers<R extends Labelled>(
  rows: R[],
  label: string,
  jqlKeys: Set<string> | null,
): R[] {
  return rows.filter(
    (r) =>
      (!label || (r.labels ?? []).includes(label)) &&
      (!jqlKeys || jqlKeys.has(r.testKey)),
  );
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
  jqlAvailable = true,
  onJqlKeys,
}: {
  profileId: string;
  containerKey: string;
  labels: string[];
  label: string;
  onLabel: (label: string) => void;
  jqlOn: boolean;
  // jqlAvailable is false for a backend with no JQL and for the demo profile,
  // which has no Jira; the form is left out rather than shown to fail.
  jqlAvailable?: boolean;
  onJqlKeys: (keys: Set<string> | null) => void;
}) {
  const { prompt } = usePrompt();
  const [jql, setJql] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  // JQL is the filter used least, so it asks in a dialog rather than
  // keeping a wide box on the filter row (#471). The dialog opens on the
  // last query, so a refused one can be fixed.
  const ask = async () => {
    const q = (
      await prompt({
        title: "Filter the tests by JQL",
        defaultValue: jql,
        placeholder: "e.g. priority = High AND labels = smoke",
        submitLabel: "Apply JQL",
      })
    )?.trim();
    if (!q) return;
    setJql(q);
    setBusy(true);
    setError("");
    try {
      onJqlKeys(new Set(await ContainerTestsMatching(profileId, containerKey, q)));
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="member-filters">
      {labels.length > 0 && (
        <select
          aria-label="Label"
          className="container-status-filter app-select"
          value={label}
          onChange={(e) => onLabel(e.target.value)}
        >
          <option value="">All labels</option>
          {labels.map((l) => (
            <option key={l} value={l}>
              {l}
            </option>
          ))}
        </select>
      )}
      {jqlAvailable && (
        <span className="member-filter">
          <button type="button" className="btn" disabled={busy} onClick={() => void ask()}>
            JQL…
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
        </span>
      )}
      {error && (
        <span role="alert" className="error-text">
          {error}
        </span>
      )}
    </div>
  );
}
