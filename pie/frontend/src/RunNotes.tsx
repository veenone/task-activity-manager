import type { query } from "../wailsjs/go/models";

// RunNotes is what PIE says about the last run above its views: what changed
// since the previous run of a saved query, an error, and how far the link walk
// got.
export function RunNotes({
  change,
  error,
  result,
}: {
  change: { added: string[]; dropped: string[] } | null;
  error: string;
  result: query.Result | null;
}) {
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
    </>
  );
}
