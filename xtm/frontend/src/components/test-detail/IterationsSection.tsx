import { useCallback, useEffect, useState } from "react";

import { GetRunIterations, type RunIteration } from "../../api";
import { RunStatusBadge } from "./rows";

// IterationsSection shows how each row of a test's dataset went in one
// execution (#267 Phase 4).
//
// A parameterized test runs once per dataset row, and Xray keeps a result per
// row. The run's single status is derived from them, so on its own it says
// that something failed without saying which values failed.
//
// It renders nothing without an execution selected, and nothing for a run
// that is not parameterized, which is most of them.
export function IterationsSection({
  profileId,
  testKey,
  containerKey,
}: {
  profileId: string;
  testKey: string;
  containerKey: string;
}) {
  const [iterations, setIterations] = useState<RunIteration[]>([]);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    if (!containerKey || !testKey) return;
    setError("");
    try {
      setIterations((await GetRunIterations(profileId, containerKey, testKey)) ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, [profileId, testKey, containerKey]);

  useEffect(() => {
    void load();
  }, [load]);

  if (!containerKey) return null;
  if (error) {
    return (
      <div className="dataset-section">
        <h4 className="dataset-head">Iterations</h4>
        <p className="dataset-error">{error}</p>
      </div>
    );
  }
  if (iterations.length === 0) return null;

  // Every iteration of a run carries the same parameters, so the first one
  // names the columns.
  const names = iterations[0].parameters?.map((p) => p.name) ?? [];
  const passed = iterations.filter((i) => i.status === "PASS").length;

  return (
    <div className="dataset-section">
      <h4 className="dataset-head">
        Iterations
        <span className="dataset-count">
          {passed} of {iterations.length} iterations passed
        </span>
        <span className="dataset-level">in {containerKey}</span>
      </h4>
      <div className="dataset-scroll">
        <table className="dataset-table" aria-label="Iterations and their results">
          <thead>
            <tr>
              {names.map((name) => (
                <th key={name} scope="col">
                  {name}
                </th>
              ))}
              <th scope="col">Result</th>
            </tr>
          </thead>
          <tbody>
            {iterations.map((iter, i) => (
              <tr key={iter.id || i}>
                {names.map((name, c) => (
                  <td key={name}>{iter.parameters?.[c]?.value ?? ""}</td>
                ))}
                <td>
                  <RunStatusBadge status={iter.status} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
