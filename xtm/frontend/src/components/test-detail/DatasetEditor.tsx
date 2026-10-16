import { useState } from "react";

import { SaveTestDataset, type DatasetParameter, type TestDataset } from "../../api";

// DatasetEditor edits a test's parameter table in place (#277 W1).
//
// Saving is local, like every other edit in XTM: the table goes to the store
// and the journal, and reaches Jira on the next Commit. The limits Xray
// enforces are checked on the Go side, so what comes back from a refused save
// is the reason, shown here with the table still on screen to fix.
export function DatasetEditor({
  profileId,
  testKey,
  initial,
  onSaved,
  onCancel,
}: {
  profileId: string;
  testKey: string;
  initial: TestDataset;
  onSaved: (saved: TestDataset) => void;
  onCancel: () => void;
}) {
  const [parameters, setParameters] = useState<DatasetParameter[]>(
    initial.parameters.length > 0 ? initial.parameters : [{ name: "", combinatorial: false }],
  );
  const [rows, setRows] = useState<string[][]>(initial.rows.length > 0 ? initial.rows : [[""]]);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  const setParameter = (i: number, over: Partial<DatasetParameter>) =>
    setParameters((ps) => ps.map((p, j) => (i === j ? { ...p, ...over } : p)));

  const addParameter = () => {
    setParameters((ps) => [...ps, { name: "", combinatorial: false }]);
    setRows((rs) => rs.map((r) => [...r, ""]));
  };

  const removeParameter = (i: number) => {
    setParameters((ps) => ps.filter((_, j) => j !== i));
    setRows((rs) => rs.map((r) => r.filter((_, j) => j !== i)));
  };

  const setCell = (r: number, c: number, value: string) =>
    setRows((rs) => rs.map((row, i) => (i === r ? row.map((v, j) => (j === c ? value : v)) : row)));

  const addRow = () => setRows((rs) => [...rs, parameters.map(() => "")]);
  const removeRow = (r: number) => setRows((rs) => rs.filter((_, i) => i !== r));

  const save = async () => {
    setSaving(true);
    setError("");
    // A row shorter than the parameter list is padded here rather than in the
    // store, so what is saved is what the grid shows.
    const table: TestDataset = {
      parameters,
      rows: rows.map((r) => parameters.map((_, c) => r[c] ?? "")),
    };
    try {
      await SaveTestDataset(profileId, testKey, table);
      onSaved(table);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="dataset-editor">
      <div className="dataset-scroll">
        <table className="dataset-table" aria-label="Dataset editor">
          <thead>
            <tr>
              {parameters.map((p, i) => (
                <th key={i} scope="col">
                  <input
                    className="dataset-input"
                    value={p.name}
                    aria-label={`Parameter ${i + 1} name`}
                    onChange={(e) => setParameter(i, { name: e.target.value })}
                  />
                  <label className="dataset-combi-label">
                    <input
                      type="checkbox"
                      checked={p.combinatorial}
                      aria-label={`Parameter ${i + 1} is combinatorial`}
                      onChange={(e) => setParameter(i, { combinatorial: e.target.checked })}
                    />
                    combinatorial
                  </label>
                  <button
                    className="link-btn dataset-drop"
                    aria-label={`Remove parameter ${i + 1}`}
                    onClick={() => removeParameter(i)}
                    disabled={parameters.length === 1}
                    title="Remove this parameter and its values"
                  >
                    Remove
                  </button>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((row, r) => (
              <tr key={r}>
                {parameters.map((p, c) => (
                  <td key={c}>
                    <input
                      className="dataset-input"
                      value={row[c] ?? ""}
                      aria-label={`Row ${r + 1}, parameter ${c + 1}`}
                      onChange={(e) => setCell(r, c, e.target.value)}
                    />
                  </td>
                ))}
                <td>
                  <button
                    className="link-btn dataset-drop"
                    aria-label={`Remove row ${r + 1}`}
                    onClick={() => removeRow(r)}
                    disabled={rows.length === 1}
                    title="Remove this row"
                  >
                    Remove
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {error && <p className="dataset-error">{error}</p>}

      <div className="dataset-actions">
        <button className="btn" onClick={addParameter}>
          Add parameter
        </button>
        <button className="btn" onClick={addRow}>
          Add row
        </button>
        <button className="btn btn-primary" onClick={() => void save()} disabled={saving}>
          {saving ? "Saving…" : "Save"}
        </button>
        <button className="btn" onClick={onCancel} disabled={saving}>
          Cancel
        </button>
      </div>
    </div>
  );
}
