import type { query } from "../wailsjs/go/models";

// RunNotes is what PIE says about the last run above its views: what changed
// since the previous run of a saved query, an error, how far the link walk
// got, and which cases are for issues the run did not match.
export function RunNotes({
  change,
  error,
  result,
  caseKeys,
}: {
  change: { added: string[]; dropped: string[] } | null;
  error: string;
  result: query.Result | null;
  caseKeys: Set<string>;
}) {
  const matched = new Set(result?.issues.filter((i) => i.matched).map((i) => i.key));
  // A case outlives the run it was opened from, so the analyst is told when
  // the query stops matching its issue.
  const stray = result ? [...caseKeys].filter((k) => !matched.has(k)) : [];
  return (
    <>
      {change && (
        <details className="pie-change">
          <summary>
            Since the previous run: {change.added.length} added, {change.dropped.length} dropped
          </summary>
          {change.added.length > 0 && <p>Added: {change.added.join(", ")}</p>}
          {change.dropped.length > 0 && <p>Dropped: {change.dropped.join(", ")}</p>}
        </details>
      )}

      {error && (
        <p role="alert" className="error-text">
          {error}
        </p>
      )}

      {result?.linkError && (
        <p role="alert" className="error-text">
          Linked issues could not all be fetched: {result.linkError}
        </p>
      )}
      {result?.linksTruncated && (
        <p className="muted">Links were followed only until 5000 issues were fetched, so some linked sets may be incomplete.</p>
      )}
      {stray.length > 0 && (
        <p className="muted">
          {stray.length === 1
            ? `1 case is for an issue this run did not match: ${stray[0]}`
            : `${stray.length} cases are for issues this run did not match: ${stray.join(", ")}`}
        </p>
      )}
    </>
  );
}
