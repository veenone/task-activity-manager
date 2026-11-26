import { ProgressBar, StatusBadge } from "@agile-suite/core";
import type { VersionSummary } from "../api";
import { releaseDates, releaseStatus } from "../lib/releases";

interface Props {
  releases: VersionSummary[];
  selectedId: number;
  onSelect: (id: number) => void;
  // waiting maps a release id to what Commit will do to it.
  waiting: Map<number, string>;
}

// ReleaseList is the project's releases as a table: name, status, dates,
// and how many of its cached issues are finished. Picking a row shows its
// issues under the table. The name is a button so the row can be picked
// from the keyboard; the rest of the row repeats the click for a pointer.
export function ReleaseList({ releases, selectedId, onSelect, waiting }: Props) {
  return (
    <table className="release-table" aria-label="Releases">
      <thead>
        <tr>
          <th scope="col">Release</th>
          <th scope="col">Status</th>
          <th scope="col">Dates</th>
          <th scope="col">Progress</th>
        </tr>
      </thead>
      <tbody>
        {releases.map((v) => {
          const status = releaseStatus(v);
          const chip = waiting.get(v.id);
          const selected = v.id === selectedId;
          const progress = v.total > 0 ? `${v.done} of ${v.total} done` : "No issues";
          return (
            <tr key={v.id} className={`release-row${selected ? " release-row-selected" : ""}`} onClick={() => onSelect(v.id)}>
              <td>
                <button type="button" className="release-name" aria-pressed={selected} onClick={(e) => { e.stopPropagation(); onSelect(v.id); }}>
                  {v.name}
                </button>
                {v.description && <span className="release-description muted small" title={v.description}>{v.description}</span>}
              </td>
              <td>
                <span className="release-badges">
                  <StatusBadge tone={status.tone} label={status.label} />
                  {chip && <span className="chip chip-draft">{chip}</span>}
                </span>
              </td>
              <td className="muted small">{releaseDates(v)}</td>
              <td>
                <span className="release-progress">
                  {v.total > 0 && (
                    <span className="release-progress-bar">
                      <ProgressBar value={v.done} max={v.total} label={`Finished issues in ${v.name}`} valueText={progress} tone="progress" />
                    </span>
                  )}
                  <span className="muted small">{progress}</span>
                </span>
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}
