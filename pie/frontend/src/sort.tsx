import { useState } from "react";

type Value = string | number;

// compare orders text as people read it, ignoring case and with numbers
// inside it by value (A-2 before A-10), numbers by value, and blanks last.
export function compare(a: Value, b: Value): number {
  return blanks(a, b) || order(a, b);
}

const blank = (v: Value) => v === "";
const blanks = (a: Value, b: Value) => Number(blank(a)) - Number(blank(b));
function order(a: Value, b: Value): number {
  if (typeof a === "number" && typeof b === "number") return a - b;
  return String(a).localeCompare(String(b), undefined, { numeric: true, sensitivity: "base" });
}

// useSort sorts rows by the column whose header was clicked; a second click
// reverses it. Rows keep the order they came in until then (#454). by maps a
// column id to the value its rows sort on.
export function useSort<T>(rows: T[], by: Record<string, (row: T) => Value>) {
  const [sort, setSort] = useState<{ col: string; desc: boolean } | null>(null);
  const get = sort ? by[sort.col] : undefined;
  const sorted =
    sort && get
      ? [...rows].sort((a, b) => {
          const [x, y] = [get(a), get(b)];
          // Blanks stay last whichever way the column runs.
          return blanks(x, y) || (sort.desc ? -1 : 1) * order(x, y);
        })
      : rows;
  const th = (col: string, label: string) => {
    const active = sort?.col === col;
    return (
      <th key={col} scope="col" aria-sort={active ? (sort.desc ? "descending" : "ascending") : "none"}>
        <button
          type="button"
          className={`pie-sort${active ? " pie-sort-active" : ""}`}
          aria-label={`Sort by ${label}`}
          title={`Sort by ${label}`}
          onClick={() => setSort({ col, desc: active ? !sort.desc : false })}
        >
          {label}
          <span className="pie-sort-caret" aria-hidden="true">
            {active ? (sort.desc ? "▾" : "▴") : "▸"}
          </span>
        </button>
      </th>
    );
  };
  return { sorted, th };
}
