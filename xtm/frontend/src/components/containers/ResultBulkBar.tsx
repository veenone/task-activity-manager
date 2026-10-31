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
//
// statuses is the vocabulary to offer. It defaults to Xray's six for the
// parameterized rows, whose iterations only exist on Xray; a backend with its
// own result names passes them in, because offering a name it has no id for
// means the pick fails at Commit (#371 Phase 5).
export function ResultBulkBar({
  label,
  count,
  statuses = RUN_STATUSES,
  onApply,
  onClear,
}: {
  label: string;
  count: number;
  statuses?: string[];
  onApply: (status: string) => void;
  onClear: () => void;
}) {
  const offered = statuses.length > 0 ? statuses : RUN_STATUSES;
  const [picked, setPicked] = useState("");
  // Derived rather than held: the bar can mount before the profile's
  // capabilities load, so a status stored on first render would keep the
  // fallback vocabulary's first name even once the backend's own arrived, and
  // Apply would send a name it has no id for.
  const status = offered.includes(picked) ? picked : offered[0];

  if (count === 0) return null;
  return (
    <div className="parameterized-bulk-bar" role="group" aria-label={label}>
      <span className="parameterized-bulk-count">{count} selected</span>
      <select
        className="run-select"
        aria-label={`${label}: result to apply`}
        value={status}
        onChange={(event) => setPicked(event.target.value)}
      >
        {offered.map((option) => (
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
