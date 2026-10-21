import { useEffect, useId, useRef, useState } from "react";
import type { FieldDef } from "../api";
import {
  buildCreateFieldsPayload,
  createFieldsValid,
  initCreateFieldDefaults,
} from "./createFields";
import type { RawFieldValue } from "./createFields";
import { MultiSelect } from "@agile-suite/core";

// useCreateScreenFields loads the required fields a create screen adds to a
// form, and holds what the user enters for them. load runs again whenever
// reloadKey changes. A failed lookup leaves the form as it was: the create can
// still be tried, and Jira will say what it lacks.
export function useCreateScreenFields(
  load: () => Promise<FieldDef[] | null>,
  reloadKey: string,
) {
  const loadRef = useRef(load);
  loadRef.current = load;
  const [fields, setFields] = useState<FieldDef[]>([]);
  const [values, setValues] = useState<Record<string, RawFieldValue>>({});

  useEffect(() => {
    let cancelled = false;
    loadRef
      .current()
      .then((defs) => {
        if (cancelled) return;
        setFields(defs ?? []);
        setValues(initCreateFieldDefaults(defs ?? []));
      })
      .catch(() => {
        if (!cancelled) setFields([]);
      });
    return () => {
      cancelled = true;
    };
  }, [reloadKey]);

  return {
    fields,
    values,
    setValue: (id: string, v: RawFieldValue) =>
      setValues((prev) => ({ ...prev, [id]: v })),
    valid: createFieldsValid(fields, values),
    payload: () => buildCreateFieldsPayload(fields, values),
  };
}

interface Props {
  fields: FieldDef[];
  values: Record<string, RawFieldValue>;
  onChange: (id: string, value: RawFieldValue) => void;
}

// CreateFieldInputs renders one labelled input per create-screen field, in the
// detail panel's field style: a select for options and versions, a checkbox
// dropdown for multi-value fields, and text, number or date otherwise.
export function CreateFieldInputs({ fields, values, onChange }: Props) {
  const idBase = useId();
  return (
    <>
      {fields.map((f) => {
        const id = `${idBase}-${f.id}`;
        const raw = values[f.id] ?? "";
        const text = typeof raw === "string" ? raw : "";
        const label = (
          <label className="field-label" htmlFor={id}>
            {`${f.name}${f.required ? " *" : ""}`}
          </label>
        );
        if (f.type === "option" || f.type === "version") {
          return (
            <div key={f.id}>
              {label}
              <select
                id={id}
                className="detail-input"
                value={text}
                onChange={(e) => onChange(f.id, e.target.value)}
              >
                {!text && <option value="">(select)</option>}
                {f.allowedValues.map((av) => (
                  <option key={av.id} value={av.id}>
                    {av.value}
                  </option>
                ))}
              </select>
            </div>
          );
        }
        if (f.type === "versions" || f.type === "array") {
          return (
            <div key={f.id}>
              <div className="field-label">{`${f.name}${f.required ? " *" : ""}`}</div>
              <MultiSelect
                options={f.allowedValues.map((av) => ({ value: av.id, label: av.value }))}
                selected={Array.isArray(raw) ? raw : []}
                onChange={(next) => onChange(f.id, next)}
                allLabel={`Select ${f.name}…`}
              />
            </div>
          );
        }
        if (f.type === "number" || f.type === "date") {
          return (
            <div key={f.id}>
              {label}
              <input
                id={id}
                className="detail-input"
                type={f.type}
                value={text}
                onChange={(e) => onChange(f.id, e.target.value)}
              />
            </div>
          );
        }
        return (
          <div key={f.id}>
            {label}
            <input
              id={id}
              className="detail-input"
              value={text}
              onChange={(e) => onChange(f.id, e.target.value)}
            />
          </div>
        );
      })}
    </>
  );
}
