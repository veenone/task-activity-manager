import { useState } from "react";

import {
  RenameDatasetPlaceholder,
  SaveTestDataset,
  type DatasetParameter,
  type TestDataset,
} from "../../api";
import { useConfirm } from "../useConfirm";

// DatasetEditor edits a test's parameter table in place (#277 W1).
//
// Saving is local, like every other edit in XTM: the table goes to the store
// and the journal, and reaches Jira on the next Commit. The limits Xray
// enforces are checked on the Go side, so what comes back from a refused save
// is the reason, shown here with the table still on screen to fix.
// Rename is one parameter that kept its id and changed its name.
interface Rename {
  from: string;
  to: string;
}

// renamesIn pairs parameters by the id Xray gave them, so a rename is told
// from a removal and an addition. A parameter with no id is new here and
// cannot have been renamed.
function renamesIn(before: TestDataset, after: DatasetParameter[]): Rename[] {
  const out: Rename[] = [];
  for (const p of after) {
    if (!p.id) continue;
    const was = before.parameters.find((q) => q.id === p.id);
    if (was && was.name !== p.name) out.push({ from: was.name, to: p.name });
  }
  return out;
}

// swap returns the list with two entries exchanged.
function swap<T>(xs: T[], i: number, j: number): T[] {
  const out = [...xs];
  [out[i], out[j]] = [out[j], out[i]];
  return out;
}

// fit pads a row to the parameter count, so moving a column does not leave a
// hole in a row that was short.
function fit(row: string[], width: number): string[] {
  return Array.from({ length: width }, (_, i) => row[i] ?? "");
}

// optionsOf is a list parameter's allowed values, empty for a text one.
function optionsOf(p: DatasetParameter): string[] {
  return p.options ?? [];
}

// optionListID names the datalist a column's cells point at.
function optionListID(column: number): string {
  return `dataset-options-${column}`;
}

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
  const { confirm } = useConfirm();

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

  // Moving a parameter moves its column of values with it. Moving a row
  // changes the order the iterations run in, which is the point of it.
  const moveParameter = (i: number, by: number) => {
    const j = i + by;
    if (j < 0 || j >= parameters.length) return;
    setParameters((ps) => swap(ps, i, j));
    setRows((rs) => rs.map((row) => swap(fit(row, parameters.length), i, j)));
  };
  const moveRow = (r: number, by: number) => {
    const j = r + by;
    if (j < 0 || j >= rows.length) return;
    setRows((rs) => swap(rs, r, j));
  };

  // A ${placeholder} refers to a parameter by name, so a rename leaves every
  // reference to the old name resolving to nothing and the test runs with the
  // literal text. The rewrite is offered rather than done, because only the
  // author knows whether a reference was deliberate.
  const offerRewrites = async (renames: Rename[]) => {
    for (const r of renames) {
      const ok = await confirm({
        title: "Rewrite the references too?",
        message: `This test's steps and definition may refer to \${${r.from}}. Rewrite those to \${${r.to}}?`,
        confirmLabel: "Rewrite",
        cancelLabel: "Leave them",
      });
      if (ok) {
        await RenameDatasetPlaceholder(profileId, testKey, r.from, r.to);
      }
    }
  };

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
      await offerRewrites(renamesIn(initial, parameters));
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
                  <span className="dataset-move">
                    <button
                      className="link-btn"
                      aria-label={`Move parameter ${i + 1} left`}
                      onClick={() => moveParameter(i, -1)}
                      disabled={i === 0}
                      title="Move this parameter one column left"
                    >
                      {"<"}
                    </button>
                    <button
                      className="link-btn"
                      aria-label={`Move parameter ${i + 1} right`}
                      onClick={() => moveParameter(i, 1)}
                      disabled={i === parameters.length - 1}
                      title="Move this parameter one column right"
                    >
                      {">"}
                    </button>
                  </span>
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
                      list={optionsOf(p).length > 0 ? optionListID(c) : undefined}
                      onChange={(e) => setCell(r, c, e.target.value)}
                    />
                  </td>
                ))}
                <td>
                  <button
                    className="link-btn"
                    aria-label={`Move row ${r + 1} up`}
                    onClick={() => moveRow(r, -1)}
                    disabled={r === 0}
                    title="Run this iteration earlier"
                  >
                    {"^"}
                  </button>
                  <button
                    className="link-btn"
                    aria-label={`Move row ${r + 1} down`}
                    onClick={() => moveRow(r, 1)}
                    disabled={r === rows.length - 1}
                    title="Run this iteration later"
                  >
                    {"v"}
                  </button>
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

      {/* The values a list parameter allows, offered to every cell under it.
          Xray owns the list; this only saves the user remembering it. */}
      {parameters.map((p, i) =>
        optionsOf(p).length > 0 ? (
          <datalist key={i} id={optionListID(i)}>
            {optionsOf(p).map((o) => (
              <option key={o} value={o} />
            ))}
          </datalist>
        ) : null,
      )}

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
