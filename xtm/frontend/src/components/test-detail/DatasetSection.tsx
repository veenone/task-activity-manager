import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { GetContainerDataset, GetTestDataset, errMsg, type TestDataset } from "../../api";
import { useTestDataset } from "../../queries/datasets";
import { keys } from "../../queries/keys";
import { DatasetEditor } from "./DatasetEditor";
import "./dataset.css";

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
// nothing at all on a backend with no dataset concept. Where the backend
// takes dataset writes (canEdit), a test with no dataset instead offers to
// give it one, and a test with one can be edited (#277 W1).
export function DatasetSection({
  profileId,
  testKey,
  supported,
  canEdit = false,
  containerKey = "",
  onEdited,
}: {
  profileId: string;
  testKey: string;
  supported: boolean;
  canEdit?: boolean;
  // The Test Execution the test is being looked at inside, if any. An
  // execution can define its own dataset for a test, and that is what runs
  // there (#277 W3).
  containerKey?: string;
  onEdited?: () => void;
}) {
  const [override, setOverride] = useState<TestDataset | null>(null);
  const [overrideError, setOverrideError] = useState("");
  const [overrideLoading, setOverrideLoading] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [editing, setEditing] = useState(false);
  const queryClient = useQueryClient();
  const datasetQuery = useTestDataset(profileId, testKey, supported);
  const dataset = datasetQuery.data ?? null;

  useEffect(() => {
    if (!supported || !testKey || !containerKey) {
      setOverride(null);
      setOverrideError("");
      return;
    }
    let cancelled = false;
    setOverrideLoading(true);
    setOverrideError("");
    GetContainerDataset(profileId, containerKey, testKey, false)
      .then((next) => {
        if (!cancelled) setOverride(next);
      })
      .catch((caught) => {
        if (!cancelled) setOverrideError(errMsg(caught));
      })
      .finally(() => {
        if (!cancelled) setOverrideLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [profileId, testKey, supported, containerKey]);

  async function refresh() {
    setRefreshing(true);
    setOverrideError("");
    try {
      const next = await GetTestDataset(profileId, testKey, true);
      queryClient.setQueryData(keys.testDataset(profileId, testKey), next);
      setOverride(
        containerKey
          ? await GetContainerDataset(profileId, containerKey, testKey, true)
          : null,
      );
    } catch (caught) {
      setOverrideError(errMsg(caught));
    } finally {
      setRefreshing(false);
    }
  }

  if (!supported) return null;
  const error = overrideError || (datasetQuery.error ? errMsg(datasetQuery.error) : "");
  if (error) {
    return (
      <div className="dataset-section">
        <h4 className="dataset-head">Dataset</h4>
        <p className="dataset-error">{error}</p>
      </div>
    );
  }
  // Nearest level wins, which is what Xray does when the test runs.
  const overridden = (override?.parameters.length ?? 0) > 0;
  const shown = overridden ? override : dataset;
  const level = !containerKey ? "" : overridden ? `defined in ${containerKey}` : "from the test";
  const loading = datasetQuery.isFetching || overrideLoading || refreshing;

  if (editing) {
    return (
      <>
        <DatasetEditor
          profileId={profileId}
          testKey={testKey}
          initial={dataset ?? { parameters: [], rows: [] }}
          onSaved={() => {
            setEditing(false);
            onEdited?.();
          }}
          onCancel={() => setEditing(false)}
        />
        <div className="dataset-section">
          <h4 className="dataset-head">Dataset</h4>
        </div>
      </>
    );
  }
  // A test with no dataset shows nothing at all, so an ordinary test's panel
  // is unchanged, unless one can be created here.
  if (!shown || shown.parameters.length === 0) {
    if (!canEdit) return null;
    return (
      <div className="dataset-section">
        <h4 className="dataset-head">
          Dataset
          <span className="dataset-count">none</span>
          <button className="link-btn dataset-refresh" onClick={() => setEditing(true)}>
            Create dataset
          </button>
        </h4>
      </div>
    );
  }

  const total = shown.rows.length;
  const drawn = shown.rows.slice(0, DATASET_ROW_CAP);
  const combinatorial = shown.parameters.some((p) => p.combinatorial);

  return (
    <div className="dataset-section">
      <h4 className="dataset-head">
        Dataset
        <span className="dataset-count">{iterationLabel(total, combinatorial)}</span>
        {level && <span className="dataset-level">{level}</span>}
        <button
          className="link-btn dataset-refresh"
          onClick={() => void refresh()}
          disabled={loading}
          title="Re-fetch the dataset from Jira"
        >
          {loading ? "Loading…" : "Refresh"}
        </button>
        {canEdit && !overridden && (
          <button
            className="link-btn dataset-edit"
            onClick={() => setEditing(true)}
            title="Edit the parameters and rows"
          >
            Edit
          </button>
        )}
      </h4>
      <div className="dataset-scroll">
        <table className="dataset-table" aria-label="Dataset parameters and rows">
          <thead>
            <tr>
              {shown.parameters.map((p) => (
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
            {drawn.map((row, i) => (
              <tr key={i}>
                {shown.parameters.map((p, c) => (
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
