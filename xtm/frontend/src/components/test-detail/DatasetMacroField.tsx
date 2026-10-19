import { useMemo, useRef, useState } from "react";
import type { ComponentProps, ReactNode } from "react";

import type { DatasetParameter } from "../../api";
import { MarkdownField, type MarkdownFieldHandle } from "../MarkdownField";
import "./dataset-macro.css";

interface TriggerRange {
  start: number;
  end: number;
  prefix: string;
}

type Props = Omit<ComponentProps<typeof MarkdownField>, "ref"> & {
  parameters: DatasetParameter[];
};

function activeTrigger(value: string): TriggerRange | null {
  const start = value.lastIndexOf("${");
  if (start < 0 || value.indexOf("}", start) >= 0) return null;
  return { start, end: value.length, prefix: value.slice(start + 2) };
}

function undefinedReferences(value: string, parameterNames: Set<string>): string[] {
  const missing = new Set<string>();
  for (const match of value.matchAll(/\$\{([^}]+)\}/g)) {
    if (!parameterNames.has(match[1])) missing.add(match[1]);
  }
  return Array.from(missing);
}

export function renderDatasetReferences(
  value: string,
  parameterNames: Iterable<string>,
): ReactNode {
  const missing = undefinedReferences(value, new Set(parameterNames));
  if (missing.length === 0) return null;
  return (
    <span className="dataset-macro-warning" role="status">
      {missing.map((name) => `Undefined dataset parameter: ${name}`).join(". ")}
    </span>
  );
}

export function DatasetMacroField({
  parameters,
  value,
  onChange,
  onCommit,
  className,
  ...fieldProps
}: Props) {
  const fieldRef = useRef<MarkdownFieldHandle>(null);
  const [open, setOpen] = useState(false);
  const [trigger, setTrigger] = useState<TriggerRange | null>(null);
  const [activeIndex, setActiveIndex] = useState(0);
  const names = useMemo(() => parameters.map((parameter) => parameter.name), [parameters]);
  const filtered = useMemo(() => {
    const prefix = trigger?.prefix.toLocaleLowerCase() ?? "";
    return names.filter((name) => name.toLocaleLowerCase().startsWith(prefix));
  }, [names, trigger]);

  function change(next: string) {
    onChange(next);
    const nextTrigger = activeTrigger(next);
    setTrigger(nextTrigger);
    setActiveIndex(0);
    setOpen(Boolean(nextTrigger));
  }

  function insert(name: string) {
    const macro = `\${${name}}`;
    if (trigger) {
      fieldRef.current?.insertText(macro, { start: trigger.start, end: trigger.end });
    } else {
      fieldRef.current?.insertText(macro);
    }
    setOpen(false);
    setTrigger(null);
  }

  function onKeyDown(event: React.KeyboardEvent<HTMLDivElement>) {
    if (!open) return;
    if (event.key === "Escape") {
      event.preventDefault();
      setOpen(false);
      return;
    }
    if (filtered.length === 0) return;
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActiveIndex((current) => (current + 1) % filtered.length);
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setActiveIndex((current) => (current - 1 + filtered.length) % filtered.length);
    } else if (event.key === "Enter") {
      event.preventDefault();
      insert(filtered[activeIndex] ?? filtered[0]);
    }
  }

  return (
    <div className={`dataset-macro-field ${className ?? ""}`} onKeyDown={onKeyDown}>
      <div className="dataset-macro-editor">
        <MarkdownField
          {...fieldProps}
          ref={fieldRef}
          value={value}
          onChange={change}
          onCommit={onCommit}
          className={className}
        />
        <button
          type="button"
          className="btn btn-ghost dataset-macro-trigger"
          aria-label="Insert dataset parameter"
          aria-haspopup="listbox"
          aria-expanded={open}
          disabled={names.length === 0}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => {
            setTrigger(null);
            setActiveIndex(0);
            setOpen((current) => !current);
          }}
        >
          {"${"}
        </button>
      </div>
      {open && (
        <div className="dataset-macro-menu" role="listbox" aria-label="Dataset parameters">
          {filtered.length > 0 ? filtered.map((name, index) => (
            <button
              type="button"
              key={name}
              role="option"
              aria-selected={index === activeIndex}
              className={`dataset-macro-option${index === activeIndex ? " dataset-macro-option-active" : ""}`}
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => insert(name)}
            >
              {name}
            </button>
          )) : <p className="dataset-macro-empty">No matching parameters</p>}
        </div>
      )}
      {renderDatasetReferences(value, names)}
    </div>
  );
}
