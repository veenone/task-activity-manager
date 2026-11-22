import { useId, useState } from "react";

interface PagerProps {
  page: number; // 0-based
  pageSize: number;
  total: number;
  onPage: (page: number) => void;
  // Without it the size is fixed and no select is drawn.
  onPageSize?: (size: number) => void;
  pageSizeOptions?: number[];
  // The select's accessible name, and how each option reads.
  sizeName?: string;
  sizeLabel?: (size: number) => string;
  // A read under way: the steps hold still and the range says so.
  loading?: boolean;
  // Wraps the range onto its own line, for narrow side panels.
  compact?: boolean;
}

const DEFAULT_OPTIONS = [10, 15, 25, 50, 100];

// Pager is the one pagination control both apps use (#542): the page size,
// first / previous / next / last, a page box that goes to a typed page as
// soon as it holds a valid one, and the rows on screen. Where it sits (a
// footer band, a modal's foot, a side panel) is the caller's to style.
export function Pager({
  page,
  pageSize,
  total,
  onPage,
  onPageSize,
  pageSizeOptions = DEFAULT_OPTIONS,
  sizeName = "Rows per page",
  sizeLabel = (n) => `${n} / page`,
  loading = false,
  compact = false,
}: PagerProps) {
  const id = useId();
  const lastPage = Math.max(0, Math.ceil(total / pageSize) - 1);
  const current = Math.min(Math.max(0, page), lastPage);
  // What is being typed into the page box. An empty or out-of-range value is
  // a half-typed number, not a request to jump to an edge, so it stays as
  // typed until the box is left.
  const [draft, setDraft] = useState<string | null>(null);
  const first = total === 0 ? 0 : current * pageSize + 1;
  const last = Math.min(total, (current + 1) * pageSize);

  function go(n: number) {
    setDraft(null);
    onPage(Math.min(lastPage, Math.max(0, n)));
  }

  return (
    <div className={`pager${compact ? " pager-compact" : ""}`}>
      {onPageSize && (
        <select
          className="detail-input pager-size"
          aria-label={sizeName}
          title={sizeName}
          value={pageSize}
          onChange={(e) => onPageSize(Number(e.target.value))}
        >
          {pageSizeOptions.map((n) => (
            <option key={n} value={n}>
              {sizeLabel(n)}
            </option>
          ))}
        </select>
      )}
      <span className="pager-buttons">
        <button type="button" className="btn pager-step" aria-label="First page" title="First page"
          disabled={loading || current === 0} onClick={() => go(0)}>«</button>
        <button type="button" className="btn pager-step" aria-label="Previous page" title="Previous page"
          disabled={loading || current === 0} onClick={() => go(current - 1)}>‹</button>
        <span className="pager-page">
          <label className="muted" htmlFor={`${id}-page`}>Page</label>
          <input
            id={`${id}-page`}
            type="number"
            className="detail-input pager-page-input"
            min={1}
            max={lastPage + 1}
            disabled={loading}
            value={draft ?? current + 1}
            onChange={(e) => {
              const raw = e.target.value;
              setDraft(raw);
              const n = Number(raw);
              if (raw !== "" && Number.isInteger(n) && n >= 1 && n <= lastPage + 1) onPage(n - 1);
            }}
            onBlur={() => setDraft(null)}
            onKeyDown={(e) => {
              if (e.key === "Enter") e.currentTarget.blur();
            }}
          />
          <span className="muted">{`of ${(lastPage + 1).toLocaleString()}`}</span>
        </span>
        <button type="button" className="btn pager-step" aria-label="Next page" title="Next page"
          disabled={loading || current >= lastPage} onClick={() => go(current + 1)}>›</button>
        <button type="button" className="btn pager-step" aria-label="Last page" title="Last page"
          disabled={loading || current >= lastPage} onClick={() => go(lastPage)}>»</button>
      </span>
      <span className="muted pager-range">
        {loading
          ? "Loading…"
          : total === 0
            ? "0 of 0"
            : `${first.toLocaleString()} to ${last.toLocaleString()} of ${total.toLocaleString()}`}
      </span>
    </div>
  );
}
