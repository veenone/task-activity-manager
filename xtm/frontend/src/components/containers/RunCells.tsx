import { useEffect, useRef, useState } from "react";
import { onActivateKey } from "@agile-suite/core";
import type { Bucket, ExecMemberRun } from "../../api";


// ExecRunCells renders the four read-only run-context cells (Date, By,
// Environment, Fix Version) for one member row in a Test Execution. Rendered
// as a fragment so the cells sit inline in the <tr> alongside the editable
// result cell.
export function ExecRunCells({
  run,
  activeFixVersion,
}: {
  run: ExecMemberRun | undefined;
  activeFixVersion: string;
}) {
  const dateStr = run?.finishedAt || run?.startedAt || "";
  const fvs = run?.fixVersions ?? [];
  const fvLabel = fvs.length > 0 ? fvs.join(", ") : "(none)";
  return (
    <>
      <td className="muted board-run-date">
        {dateStr ? formatRunDate(dateStr) : "(none)"}
      </td>
      <td className="muted board-run-by">
        {run?.executedBy || "(none)"}
      </td>
      <td className="muted board-run-env">
        {run?.environment || "(none)"}
      </td>
      <td
        className={`muted board-run-fv${activeFixVersion && fvs.includes(activeFixVersion) ? " board-run-fv--match" : ""}`}
        title={fvs.length > 1 ? fvs.join(", ") : undefined}
      >
        {fvLabel}
      </td>
    </>
  );
}

// DefectsCell renders one member row's linked defects as removable chips plus
// a "＋" control that opens the LinkBugPicker for that row (RND_P_4TFINT_05-296).
// Each chip links to Jira when a real profile URL exists (mirrors
// ContainersView.openBug's isDemo / NEW- key gating).
export function DefectsCell({
  testKey,
  defects,
  isDemo,
  onOpenBug,
  onUnlink,
  onLinkClick,
}: {
  testKey: string;
  defects: string[];
  isDemo: boolean;
  onOpenBug: (bugKey: string) => void;
  onUnlink: (testKey: string, bugKey: string) => void;
  onLinkClick: () => void;
}) {
  return (
    <td className="board-defects-cell">
      <div className="defect-chip-row">
        {defects.map((d) => (
          <span key={d} className="defect-chip">
            {!isDemo && !d.startsWith("NEW-") ? (
              <button
                type="button"
                className="defect-chip-key"
                onClick={() => onOpenBug(d)}
                title={`Open ${d} in Jira`}
              >
                {d}
              </button>
            ) : (
              <span className="defect-chip-key">{d}</span>
            )}
            <button
              type="button"
              className="defect-chip-remove"
              onClick={() => onUnlink(testKey, d)}
              title={`Unlink ${d} from this run`}
              aria-label={`Unlink ${d} from this run`}
            >
              ✕
            </button>
          </span>
        ))}
        <button
          type="button"
          className="btn btn-ghost defect-chip-add"
          onClick={onLinkClick}
          title="Link an existing bug to this run"
          aria-label="Link an existing bug to this run"
        >
          ＋
        </button>
      </div>
    </td>
  );
}

// RemarksCell is an inline-editable text control for one member row's run
// comment, seeded from the loaded/staged value and saved on blur or Enter
// (RND_P_4TFINT_05-296). The draft resyncs to the incoming comment whenever
// the field is not actively focused, so a refresh (after save, sync, or
// another edit) shows up immediately without clobbering in-progress typing.
export function RemarksCell({
  testKey,
  comment,
  onSave,
}: {
  testKey: string;
  comment: string;
  onSave: (testKey: string, comment: string) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(comment);
  // Set on Escape so the immediately-following blur discards the edit instead
  // of saving it. A ref (not state) so the blur handler sees it even if it
  // fires before React re-renders with the reset draft.
  const cancelled = useRef(false);

  useEffect(() => {
    if (!editing) setDraft(comment);
  }, [comment, editing]);

  function handleBlur() {
    setEditing(false);
    if (cancelled.current) {
      cancelled.current = false;
      return;
    }
    const value = draft.trim();
    if (value !== comment) onSave(testKey, value);
  }

  return (
    <td className="board-remarks-cell">
      <input
        className="remarks-input"
        value={draft}
        placeholder="Add remark…"
        title={comment || undefined}
        onFocus={() => setEditing(true)}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={handleBlur}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            (e.target as HTMLInputElement).blur();
          } else if (e.key === "Escape") {
            cancelled.current = true;
            setDraft(comment);
            (e.target as HTMLInputElement).blur();
          }
        }}
      />
    </td>
  );
}

// formatRunDate formats an ISO date/time string to a compact local date+time
// string (YYYY-MM-DD HH:MM), matching the pattern used elsewhere in the app
// (e.g. TestDetail run-history rows). Returns "" for empty/invalid input.
export function formatRunDate(iso: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  const pad = (n: number) => String(n).padStart(2, "0");
  return (
    `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ` +
    `${pad(d.getHours())}:${pad(d.getMinutes())}`
  );
}

// RunBar is a compact stacked bar of run-status proportions for the selected
// container, a glanceable view of how its tests are doing. When onPick is
// given, each segment is clickable and filters the member table to that run
// status; the active segment stays lit while the rest dim.
export function RunBar({
  counts,
  active,
  onPick,
}: {
  counts: Bucket[];
  active?: string;
  onPick?: (label: string) => void;
}) {
  const sum = counts.reduce((a, b) => a + b.count, 0) || 1;
  const filtering = !!active;
  return (
    <div
      className={`run-bar${onPick ? " run-bar--clickable" : ""}${filtering ? " run-bar--filtering" : ""}`}
      title={
        onPick
          ? "Run-status distribution. Click a segment to filter the list"
          : "Run-status distribution"
      }
    >
      {counts.map((b) => (
        <span
          key={b.label}
          className={`${runSegClass(b.label)}${active === b.label ? " run-seg--active" : ""}`}
          style={{ width: `${(b.count / sum) * 100}%` }}
          title={`${b.label}: ${b.count}${onPick ? " (click to filter)" : ""}`}
          role={onPick ? "button" : undefined}
          tabIndex={onPick ? 0 : undefined}
          aria-label={onPick ? `${b.label}: ${b.count}` : undefined}
          aria-pressed={onPick ? active === b.label : undefined}
          onClick={onPick ? () => onPick(b.label) : undefined}
          onKeyDown={onPick ? onActivateKey(() => onPick(b.label)) : undefined}
        />
      ))}
    </div>
  );
}

export function runSegClass(label: string): string {
  if (label === "(not run)") return "run-seg";
  return `run-seg run-${label.toLowerCase()}`;
}

// RunBadge shows a run-status count. When onPick is given it acts as a filter
// toggle for the member table (active = currently filtering to this status);
// without onPick it is a plain, non-interactive label (e.g. the roll-up).
export function RunBadge({
  status,
  count,
  active,
  onPick,
  pickHint,
}: {
  status: string;
  count: number;
  active?: boolean;
  onPick?: () => void;
  // Overrides the click tooltip. Defaults to the member-table filter wording.
  pickHint?: string;
}) {
  const base =
    status === "(not run)" ? "run-badge" : `run-badge run-${status.toLowerCase()}`;
  const cls = `${base}${onPick ? " run-badge--click" : ""}${active ? " run-badge--active" : ""}`;
  const label = status === "(not run)" ? "not run" : status;
  return (
    <span
      className={cls}
      role={onPick ? "button" : undefined}
      tabIndex={onPick ? 0 : undefined}
      aria-pressed={onPick ? !!active : undefined}
      onClick={onPick}
      onKeyDown={onPick ? onActivateKey(onPick) : undefined}
      title={onPick ? (pickHint ?? `Filter the list to ${label}`) : undefined}
    >
      {label} {count}
    </span>
  );
}
