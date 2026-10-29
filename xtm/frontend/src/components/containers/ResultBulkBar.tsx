import { useState } from "react";

import { RUN_STATUSES } from "./constants";

// ResultBulkBar sets one result on everything selected (#365).
//
// A dataset of three parameters over three rows expands to nine iterations,
// each with its own steps, so a run that went the same way everywhere was nine
// picks and more. It appears only once something is selected, the way the
// member table's own bulk toolbar does, so an untouched table keeps its rows
// the width they were.
//
// It owns the status it is about to apply, because that choice means nothing
// until Apply and should not survive the selection being cleared.
export function ResultBulkBar({
  label,
  count,
  onApply,
  onClear,
}: {
  label: string;
  count: number;
  onApply: (status: string) => void;
  onClear: () => void;
}) {
  const [status, setStatus] = useState(RUN_STATUSES[0]);

  if (count === 0) return null;
  return (
    <div className="parameterized-bulk-bar" role="group" aria-label={label}>
      <span className="parameterized-bulk-count">{count} selected</span>
      <select
        className="run-select"
        aria-label={`${label}: result to apply`}
        value={status}
        onChange={(event) => setStatus(event.target.value)}
      >
        {RUN_STATUSES.map((option) => (
          <option key={option} value={option}>
            {option}
          </option>
        ))}
      </select>
      <button type="button" className="btn btn-primary" onClick={() => onApply(status)}>
        Apply
      </button>
      <button type="button" className="btn btn-ghost" onClick={onClear}>
        Clear
      </button>
    </div>
  );
}
