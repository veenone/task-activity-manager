import { useState } from "react";

const SIZES = [25, 50, 100];

export interface Paging {
  page: number; // 0-based
  pageSize: number;
  total: number;
  sizes: number[];
  onPage: (page: number) => void;
  onPageSize: (size: number) => void;
}

// usePaged cuts items into pages. A page that a filter or a new run took away
// falls back to the last one left, so the list never shows empty by mistake.
// start is the index of a row the first page shown must hold.
export function usePaged<T>(items: T[], start = 0, sizes = SIZES): { rows: T[]; pager: Paging } {
  const [pageSize, setPageSize] = useState(sizes[0]);
  const [page, setPage] = useState(() => Math.floor(start / sizes[0]));
  const at = Math.min(page, Math.max(0, Math.ceil(items.length / pageSize) - 1));
  return {
    rows: items.slice(at * pageSize, (at + 1) * pageSize),
    pager: {
      page: at,
      pageSize,
      total: items.length,
      sizes,
      onPage: setPage,
      onPageSize: (size) => {
        setPageSize(size);
        setPage(0);
      },
    },
  };
}

// Pager moves through a paged list. It hides itself while everything fits on
// one page of the smallest size.
export function Pager({ label, page, pageSize, total, sizes, onPage, onPageSize }: Paging & { label: string }) {
  const [typed, setTyped] = useState<string | null>(null);
  if (total <= sizes[0]) return null;
  const last = Math.ceil(total / pageSize) - 1;
  const go = (p: number) => onPage(Math.min(last, Math.max(0, p)));
  const commit = () => {
    if (typed) go(Number(typed) - 1);
    setTyped(null);
  };

  return (
    <nav className="pie-pager" aria-label={`${label} pages`}>
      <label>
        Rows per page
        <select className="detail-input" value={pageSize} onChange={(e) => onPageSize(Number(e.target.value))}>
          {sizes.map((n) => (
            <option key={n} value={n}>
              {n}
            </option>
          ))}
        </select>
      </label>
      <span className="muted">{`${page * pageSize + 1}-${Math.min(total, (page + 1) * pageSize)} of ${total}`}</span>
      <button type="button" className="btn btn-ghost" disabled={page === 0} onClick={() => go(0)} aria-label="First page">
        «
      </button>
      <button type="button" className="btn btn-ghost" disabled={page === 0} onClick={() => go(page - 1)} aria-label="Previous page">
        ‹
      </button>
      <label>
        Page
        <input
          className="detail-input pie-page-input"
          inputMode="numeric"
          value={typed ?? String(page + 1)}
          onChange={(e) => setTyped(e.target.value.replace(/\D/g, ""))}
          onBlur={commit}
          onKeyDown={(e) => e.key === "Enter" && commit()}
        />
      </label>
      <span className="muted">of {last + 1}</span>
      <button type="button" className="btn btn-ghost" disabled={page >= last} onClick={() => go(page + 1)} aria-label="Next page">
        ›
      </button>
      <button type="button" className="btn btn-ghost" disabled={page >= last} onClick={() => go(last)} aria-label="Last page">
        »
      </button>
    </nav>
  );
}
