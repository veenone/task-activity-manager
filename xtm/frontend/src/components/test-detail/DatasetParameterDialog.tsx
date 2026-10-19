import { useState } from "react";

import type { DatasetParameter } from "../../api";
import { Modal } from "../Modal";
import { useConfirm } from "../useConfirm";

const TITLE_ID = "dataset-parameter-title";
const AD_HOC_SCOPE = "PARAMETER_AD_HOC";

function uniqueOptions(value: string): string[] {
  const seen = new Set<string>();
  const options: string[] = [];
  for (const line of value.split(/\r?\n/)) {
    const option = line.trim();
    if (option && !seen.has(option)) {
      seen.add(option);
      options.push(option);
    }
  }
  return options;
}

function affectedRows(usedValues: string[], options: string[]): Map<string, number[]> {
  const allowed = new Set(options);
  const affected = new Map<string, number[]>();
  usedValues.forEach((value, index) => {
    if (!value || allowed.has(value)) return;
    const rows = affected.get(value) ?? [];
    rows.push(index + 1);
    affected.set(value, rows);
  });
  return affected;
}

export function DatasetParameterDialog({
  parameter,
  usedValues,
  onSave,
  onCancel,
}: {
  parameter: DatasetParameter;
  usedValues: string[];
  onSave: (parameter: DatasetParameter) => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState(parameter.name);
  const [type, setType] = useState(parameter.type === "LIST" ? "LIST" : "TEXT");
  const [combinatorial, setCombinatorial] = useState(parameter.combinatorial);
  const [optionsText, setOptionsText] = useState((parameter.options ?? []).join("\n"));
  const [converted, setConverted] = useState(false);
  const [error, setError] = useState("");
  const { confirm } = useConfirm();

  const predefined =
    type === "LIST" &&
    !converted &&
    Boolean(parameter.listScope && parameter.listScope !== AD_HOC_SCOPE);

  async function convertList() {
    const ok = await confirm({
      title: "Convert this list?",
      message:
        "This parameter will stop using its predefined Jira list and keep a private copy of the current options.",
      confirmLabel: "Convert",
      cancelLabel: "Keep predefined list",
    });
    if (ok) setConverted(true);
  }

  function save() {
    const trimmedName = name.trim();
    if (!trimmedName) {
      setError("Enter a parameter name.");
      return;
    }
    if (type === "TEXT") {
      const { options: _options, listScope: _scope, listId: _listId, ...rest } = parameter;
      onSave({ ...rest, name: trimmedName, combinatorial, type: "TEXT" });
      return;
    }

    const options = uniqueOptions(optionsText);
    if (options.length === 0) {
      setError("Add at least one list option.");
      return;
    }
    const affected = affectedRows(usedValues, options);
    if (affected.size > 0) {
      setError(
        Array.from(affected, ([value, rows]) =>
          `${value} is still used by ${rows.length === 1 ? "row" : "rows"} ${rows.join(" and ")}.`,
        ).join(" "),
      );
      return;
    }

    if (predefined) {
      onSave({ ...parameter, name: trimmedName, combinatorial, type: "LIST" });
      return;
    }
    const { listId: _listId, ...withoutListId } = parameter;
    const rest = converted ? withoutListId : parameter;
    onSave({
      ...rest,
      name: trimmedName,
      combinatorial,
      type: "LIST",
      options,
      listScope: AD_HOC_SCOPE,
    });
  }

  const title = parameter.name ? "Edit parameter" : "Add parameter";
  const bindingLabel = `${parameter.listScope === "PROJECT" ? "Project" : "Predefined"} list ${parameter.listId ?? ""}`.trim();

  return (
    <Modal onClose={onCancel} className="modal dataset-parameter-dialog" labelledBy={TITLE_ID}>
      <div className="dataset-dialog-head">
        <div>
          <h2 id={TITLE_ID}>{title}</h2>
          <p>Configure the column used by test step dataset macros.</p>
        </div>
        <button className="btn btn-ghost" onClick={onCancel} aria-label="Close">×</button>
      </div>

      <div className="dataset-dialog-body">
        <label className="dataset-field">
          <span>Parameter name</span>
          <input value={name} onChange={(event) => setName(event.target.value)} />
        </label>
        <label className="dataset-field">
          <span>Type</span>
          <select
            aria-label="Parameter type"
            value={type}
            disabled={predefined}
            onChange={(event) => {
              setType(event.target.value);
              setError("");
            }}
          >
            <option value="TEXT">Text</option>
            <option value="LIST">List</option>
          </select>
        </label>
        <label className="dataset-check-field">
          <input
            type="checkbox"
            checked={combinatorial}
            onChange={(event) => setCombinatorial(event.target.checked)}
          />
          Generate combinations from this parameter
        </label>

        {type === "LIST" && (
          <div className="dataset-list-config">
            {predefined && (
              <div className="dataset-binding">
                <span>
                  <strong>{bindingLabel}</strong>
                  <small>Managed by Jira and preserved when this dataset is saved.</small>
                </span>
                <button className="btn" onClick={() => void convertList()}>
                  Convert to ad-hoc list
                </button>
              </div>
            )}
            <label className="dataset-field">
              <span>List options</span>
              <textarea
                aria-label="List options"
                rows={6}
                value={optionsText}
                disabled={predefined}
                onChange={(event) => {
                  setOptionsText(event.target.value);
                  setError("");
                }}
              />
              <small>Enter one option per line. Order is preserved.</small>
            </label>
          </div>
        )}

        {error && <p className="dataset-error" role="alert">{error}</p>}
      </div>

      <div className="dataset-dialog-actions">
        <button className="btn" onClick={onCancel}>Cancel</button>
        <button className="btn btn-primary" onClick={save}>Save parameter</button>
      </div>
    </Modal>
  );
}
