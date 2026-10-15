import { useCallback, useEffect, useState } from "react";

import { GetTestDataset, type TestDataset } from "../../api";

// DATASET_ROW_CAP is how many rows the panel draws.
//
// Xray allows 1,000 rows and 20 parameters, which is 20,000 cells in a narrow
// side panel: slow to draw and unreadable once drawn. The panel is for seeing
// what the parameters are and what a row looks like, not for auditing a
// thousand of them, so it draws the first few and says how many there are.
export const DATASET_ROW_CAP = 25;

// DatasetSection shows a parameterized test's dataset: the parameters its
// ${placeholder} references resolve against, and one row per iteration (#267).
//
// It renders nothing for a test with no dataset, which is most of them, and
// nothing at all on a backend with no dataset concept.
export function DatasetSection({
  profileId,
  testKey,
  supported,
}: {
  profileId: string;
  testKey: string;
  supported: boolean;
}) {
  const [dataset, setDataset] = useState<TestDataset | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const load = useCallback(
    async (forceRefresh: boolean) => {
      if (!supported || !testKey) return;
      setLoading(true);
      setError("");
      try {
        setDataset(await GetTestDataset(profileId, testKey, forceRefresh));
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
      } finally {
        setLoading(false);
      }
    },
    [profileId, testKey, supported],
  );

  useEffect(() => {
    void load(false);
  }, [load]);

  if (!supported) return null;
  if (error) {
    return (
      <div className="dataset-section">
        <h4 className="dataset-head">Dataset</h4>
        <p className="dataset-error">{error}</p>
      </div>
    );
  }
  // A test with no dataset shows nothing at all, so an ordinary test's panel
  // is unchanged.
  if (!dataset || dataset.parameters.length === 0) return null;

  const total = dataset.rows.length;
  const shown = dataset.rows.slice(0, DATASET_ROW_CAP);
  const combinatorial = dataset.parameters.some((p) => p.combinatorial);

  return (
    <div className="dataset-section">
      <h4 className="dataset-head">
        Dataset
        <span className="dataset-count">{iterationLabel(total, combinatorial)}</span>
        <button
          className="link-btn dataset-refresh"
          onClick={() => void load(true)}
          disabled={loading}
          title="Re-fetch the dataset from Jira"
        >
          {loading ? "Loading…" : "Refresh"}
        </button>
      </h4>
      <div className="dataset-scroll">
        <table className="dataset-table" aria-label="Dataset parameters and rows">
          <thead>
            <tr>
              {dataset.parameters.map((p) => (
                <th key={p.name} scope="col">
                  {p.name}
                  {p.combinatorial && (
                    <span
                      className="dataset-combinatorial"
                      title="Combinatorial parameter: Xray expands it into extra iterations"
                    >
                      *
                    </span>
                  )}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {shown.map((row, i) => (
              <tr key={i}>
                {dataset.parameters.map((p, c) => (
                  <td key={p.name}>{row[c] ?? ""}</td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {total > DATASET_ROW_CAP && (
        <p className="dataset-capped">
          Showing {DATASET_ROW_CAP} of {total} rows. Open the test in Jira to see them all.
        </p>
      )}
    </div>
  );
}

// iterationLabel says how much work the dataset represents.
//
// With no combinatorial parameter a row is an iteration, so the count is
// arithmetic. With one, Xray generates every combination and runs more
// iterations than there are rows, by a rule its public documentation does not
// state. Printing a guessed number next to a real table would read as fact,
// so the row count is reported on its own instead.
function iterationLabel(rows: number, combinatorial: boolean): string {
  if (combinatorial) {
    return `${rows} ${rows === 1 ? "row" : "rows"}, expanded by Xray`;
  }
  return `${rows} ${rows === 1 ? "iteration" : "iterations"}`;
}
