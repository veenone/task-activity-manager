import { useId, useRef, useState, type CSSProperties } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { ExportDatasetCSV, ImportDatasetCSV, RenameDatasetPlaceholder, SaveTestDataset, type DatasetParameter, type TestDataset } from "../../api";
import { Menu } from "../Menu";
import { Modal } from "../Modal";
import { useConfirm } from "../useConfirm";
import { DatasetParameterDialog } from "./DatasetParameterDialog";
import {
  DATASET_ACTIONS_COLUMN_WIDTH,
  DATASET_PARAMETER_COLUMN_WIDTH,
  DATASET_ROW_COLUMN_WIDTH,
  datasetEditorMinWidth,
} from "./datasetLayout";
import { keys } from "../../queries/keys";

interface Rename {
  from: string;
  to: string;
}

interface ParameterEdit {
  index: number | null;
  parameter: DatasetParameter;
}

function renamesIn(before: TestDataset, after: DatasetParameter[]): Rename[] {
  const out: Rename[] = [];
  for (const parameter of after) {
    if (!parameter.id) continue;
    const previous = before.parameters.find((candidate) => candidate.id === parameter.id);
    if (previous && previous.name !== parameter.name) {
      out.push({ from: previous.name, to: parameter.name });
    }
  }
  return out;
}

function swap<T>(items: T[], left: number, right: number): T[] {
  const result = [...items];
  [result[left], result[right]] = [result[right], result[left]];
  return result;
}

function fit(row: string[], width: number): string[] {
  return Array.from({ length: width }, (_, index) => row[index] ?? "");
}

function isList(parameter: DatasetParameter): boolean {
  return parameter.type === "LIST";
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
  const [parameters, setParameters] = useState<DatasetParameter[]>(initial.parameters);
  const [rows, setRows] = useState<string[][]>(initial.rows.length > 0 ? initial.rows : [[]]);
  const [parameterEdit, setParameterEdit] = useState<ParameterEdit | null>(null);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const titleId = useId();
  const addParameterRef = useRef<HTMLButtonElement>(null);
  const { confirm } = useConfirm();
  const queryClient = useQueryClient();

  const removeParameter = (index: number) => {
    setParameters((current) => current.filter((_, itemIndex) => itemIndex !== index));
    setRows((current) => current.map((row) => row.filter((_, itemIndex) => itemIndex !== index)));
  };

  const setCell = (rowIndex: number, columnIndex: number, value: string) =>
    setRows((current) => current.map((row, index) => (index === rowIndex ? row.map((cell, cellIndex) => (cellIndex === columnIndex ? value : cell)) : row)));

  const addRow = () => setRows((current) => [...current, parameters.map(() => "")]);
  const removeRow = (rowIndex: number) => setRows((current) => current.filter((_, index) => index !== rowIndex));

  const moveParameter = (index: number, direction: number) => {
    const target = index + direction;
    if (target < 0 || target >= parameters.length) return;
    setParameters((current) => swap(current, index, target));
    setRows((current) => current.map((row) => swap(fit(row, parameters.length), index, target)));
  };

  const moveRow = (index: number, direction: number) => {
    const target = index + direction;
    if (target < 0 || target >= rows.length) return;
    setRows((current) => swap(current, index, target));
  };

  const applyParameter = (parameter: DatasetParameter) => {
    if (parameterEdit?.index === null) {
      setParameters((current) => [...current, parameter]);
      setRows((current) => current.map((row) => [...row, ""]));
    } else if (parameterEdit) {
      setParameters((current) => current.map((item, index) => (index === parameterEdit.index ? parameter : item)));
    }
    setParameterEdit(null);
  };

  const offerRewrites = async (renames: Rename[]) => {
    for (const rename of renames) {
      const accepted = await confirm({
        title: "Rewrite the references too?",
        message: `This test's steps and definition may refer to \${${rename.from}}. Rewrite those to \${${rename.to}}?`,
        confirmLabel: "Rewrite",
        cancelLabel: "Leave them",
      });
      if (accepted) {
        await RenameDatasetPlaceholder(profileId, testKey, rename.from, rename.to);
      }
    }
  };

  const importFile = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    setError("");
    try {
      const table = await ImportDatasetCSV(await file.text(), {
        parameters,
        rows,
      });
      setParameters(table.parameters);
      setRows(table.rows);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : String(caught));
    }
  };

  const exportFile = async () => {
    setError("");
    try {
      await ExportDatasetCSV(testKey, { parameters, rows });
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : String(caught));
    }
  };

  const save = async () => {
    setSaving(true);
    setError("");
    const table: TestDataset = {
      parameters,
      rows: rows.map((row) => parameters.map((_, column) => row[column] ?? "")),
    };
    try {
      await SaveTestDataset(profileId, testKey, table);
      queryClient.setQueryData(keys.testDataset(profileId, testKey), table);
      await offerRewrites(renamesIn(initial, parameters));
      onSaved(table);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : String(caught));
    } finally {
      setSaving(false);
    }
  };

  const creating = initial.parameters.length === 0;

  return (
    <Modal initialFocus={addParameterRef} onClose={onCancel} className="modal dataset-editor-modal" labelledBy={titleId}>
      <div className="pending-head dataset-editor-head">
        <div>
          <h2 id={titleId}>{creating ? "Create dataset" : "Edit dataset"}</h2>
          <p>Parameters become reusable macros in the test steps.</p>
        </div>
        <button className="btn btn-ghost" onClick={onCancel} aria-label="Close">
          ×
        </button>
      </div>

      <div className="bulk-body dataset-editor-body">
        {parameters.length === 0 ? (
          <div className="dataset-editor-empty">
            <div>
              <strong>Start with a parameter</strong>
              <p>Add a text or list parameter, then add one row per iteration.</p>
            </div>
            <button
              ref={addParameterRef}
              className="btn btn-primary"
              onClick={() =>
                setParameterEdit({
                  index: null,
                  parameter: { name: "", combinatorial: false, type: "TEXT" },
                })
              }
            >
              Add parameter
            </button>
            <label className="btn dataset-import">
              Import CSV
              <input type="file" accept=".csv,text/csv" aria-label="Import CSV" onChange={(event) => void importFile(event)} />
            </label>
          </div>
        ) : (
          <>
            <div className="dataset-editor-toolbar" role="group" aria-label="Dataset toolbar">
              <button
                ref={addParameterRef}
                className="btn"
                onClick={() =>
                  setParameterEdit({
                    index: null,
                    parameter: { name: "", combinatorial: false, type: "TEXT" },
                  })
                }
              >
                Add parameter
              </button>
              <label className="btn dataset-import">
                Import CSV
                <input type="file" accept=".csv,text/csv" aria-label="Import CSV" onChange={(event) => void importFile(event)} />
              </label>
              <button className="btn" onClick={() => void exportFile()}>
                Export CSV
              </button>
            </div>

            <div className="dataset-scroll dataset-editor-scroll">
              <table
                className="dataset-table dataset-editor-table"
                aria-label="Dataset editor"
                style={{
                  "--dataset-row-column-width": `${DATASET_ROW_COLUMN_WIDTH}px`,
                  "--dataset-parameter-column-width": `${DATASET_PARAMETER_COLUMN_WIDTH}px`,
                  "--dataset-actions-column-width": `${DATASET_ACTIONS_COLUMN_WIDTH}px`,
                  "--dataset-editor-min-width": `${datasetEditorMinWidth(parameters.length)}px`,
                } as CSSProperties}
              >
                <colgroup>
                  <col className="dataset-row-column" />
                  {parameters.map((parameter, index) => (
                    <col className="dataset-parameter-column" key={parameter.id ?? `${parameter.name}:${index}`} />
                  ))}
                  <col className="dataset-actions-column" />
                </colgroup>
                <thead>
                  <tr>
                    <th scope="col" className="dataset-row-heading">
                      Row
                    </th>
                    {parameters.map((parameter, index) => (
                      <th key={parameter.id ?? `${parameter.name}:${index}`} scope="col">
                        <div className="dataset-parameter-heading">
                          <span>
                            <strong>{parameter.name}</strong>
                            <small>{isList(parameter) ? "List" : "Text"}</small>
                          </span>
                          {parameter.combinatorial && (
                            <span className="dataset-combinatorial" title="Combinatorial parameter">
                              *
                            </span>
                          )}
                          <Menu
                            label="Actions"
                            triggerClassName="btn btn-ghost dataset-parameter-menu"
                            triggerLabel={`Actions for parameter ${index + 1}`}
                            align="right"
                            items={[
                              {
                                key: "edit",
                                label: "Edit parameter",
                                onClick: () => setParameterEdit({ index, parameter }),
                              },
                              {
                                key: "left",
                                label: "Move left",
                                onClick: () => moveParameter(index, -1),
                                disabled: index === 0,
                              },
                              {
                                key: "right",
                                label: "Move right",
                                onClick: () => moveParameter(index, 1),
                                disabled: index === parameters.length - 1,
                              },
                              { key: "divider", divider: true },
                              {
                                key: "remove",
                                label: "Remove parameter",
                                onClick: () => removeParameter(index),
                                disabled: parameters.length === 1,
                                danger: true,
                              },
                            ]}
                          />
                        </div>
                      </th>
                    ))}
                    <th scope="col" className="dataset-row-actions-heading">
                      Row actions
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {rows.map((row, rowIndex) => (
                    <tr key={rowIndex}>
                      <th scope="row" className="dataset-row-number">
                        {rowIndex + 1}
                      </th>
                      {parameters.map((parameter, columnIndex) => (
                        <td key={parameter.id ?? `${parameter.name}:${columnIndex}`}>
                          {isList(parameter) ? (
                            <select
                              className="dataset-cell-control"
                              value={row[columnIndex] ?? ""}
                              aria-label={`Row ${rowIndex + 1}, ${parameter.name}`}
                              onChange={(event) => setCell(rowIndex, columnIndex, event.target.value)}
                            >
                              <option value="">Select an option</option>
                              {(parameter.options ?? []).map((option) => (
                                <option key={option} value={option}>
                                  {option}
                                </option>
                              ))}
                            </select>
                          ) : (
                            <input
                              className="dataset-cell-control"
                              value={row[columnIndex] ?? ""}
                              aria-label={`Row ${rowIndex + 1}, ${parameter.name}`}
                              onChange={(event) => setCell(rowIndex, columnIndex, event.target.value)}
                            />
                          )}
                        </td>
                      ))}
                      <td>
                        <div className="dataset-row-actions">
                          <button className="btn btn-ghost" aria-label={`Move row ${rowIndex + 1} up`} onClick={() => moveRow(rowIndex, -1)} disabled={rowIndex === 0}>
                            ↑
                          </button>
                          <button className="btn btn-ghost" aria-label={`Move row ${rowIndex + 1} down`} onClick={() => moveRow(rowIndex, 1)} disabled={rowIndex === rows.length - 1}>
                            ↓
                          </button>
                          <button className="btn btn-ghost" aria-label={`Remove row ${rowIndex + 1}`} onClick={() => removeRow(rowIndex)} disabled={rows.length === 1}>
                            Remove
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        )}

        {error && (
          <p className="dataset-error" role="alert">
            {error}
          </p>
        )}
      </div>

      <div className="pending-actions dataset-editor-actions" role="group" aria-label="Dataset actions">
        <button className="btn" onClick={addRow} disabled={parameters.length === 0 || saving}>
          Add row
        </button>
        <div className="dataset-primary-actions">
          <button className="btn" onClick={onCancel} disabled={saving}>
            Cancel
          </button>
          <button className="btn btn-primary" onClick={() => void save()} disabled={saving || parameters.length === 0}>
            {saving ? "Saving…" : "Save"}
          </button>
        </div>
      </div>

      {parameterEdit && (
        <DatasetParameterDialog
          parameter={parameterEdit.parameter}
          usedValues={parameterEdit.index === null ? [] : rows.map((row) => row[parameterEdit.index ?? 0] ?? "")}
          onSave={applyParameter}
          onCancel={() => setParameterEdit(null)}
        />
      )}
    </Modal>
  );
}
