import type { ReactNode } from "react";
import { fieldLabel } from "../api";
import type { PendingChange } from "../api";
import type { PendingGroup } from "../queries/pending";
import { moveWords } from "../lib/moveValue";
import { worklogEntryLine } from "./WorkLogSection";
import { boardDraftLine, draftLine, sprintChangeLine, sprintChangeName, sprintChangeNote, sprintDraftLine } from "./pendingWords";

interface Props {
  group: PendingGroup;
  project: string;
  heldReasons: string[];
  unpushableIds: Set<number>;
  busy: boolean;
  onDiscard: (row: PendingChange) => void;
}

// A Line is one row of the table: what changed, from what, to what, the
// notes that qualify it, and the Discard that throws it away.
interface Line {
  row: PendingChange;
  field: string;
  before: string;
  after: ReactNode;
  notes?: ReactNode[];
  discardLabel: string;
}

const note = (text: string, className = "muted small") => <p key={text} className={className}>{text}</p>;

// PendingGroupRows is one issue, sprint or board in the Pending changes
// table: a row group labelled with its name, whose first cell spans every
// row and carries the name and its chips. The columns are XTM's: Field,
// Before struck through, After marked, and the row's Discard.
export function PendingGroupRows({ group: g, project, heldReasons, unpushableIds, busy, onDiscard }: Props) {
  const { name, chips, lines } = describe(g, project, heldReasons, unpushableIds);
  return (
    <tbody aria-label={name}>
      {lines.map((l, i) => (
        <tr key={l.row.id}>
          {i === 0 && (
            <th scope="rowgroup" rowSpan={lines.length} className="pending-issue">
              <span className="b">{name}</span>
              {chips}
            </th>
          )}
          <td>{l.field}</td>
          <td className="pending-before">{l.before}</td>
          <td>
            <span className="pending-after">{l.after}</span>
            {l.notes}
          </td>
          <td className="pending-row-actions">
            <button type="button" className="btn" disabled={busy} aria-label={l.discardLabel} onClick={() => onDiscard(l.row)}>
              Discard
            </button>
          </td>
        </tr>
      ))}
    </tbody>
  );
}

function describe(g: PendingGroup, project: string, heldReasons: string[], unpushableIds: Set<number>) {
  const sprintLines = (rows: PendingChange[], name: string): Line[] =>
    rows.map((row) => ({
      row, field: "Sprint", before: "", after: sprintChangeLine(row),
      notes: [note(sprintChangeNote(row))], discardLabel: `Discard the change to ${name}`,
    }));

  if (g.boardRow) {
    const name = g.board?.name ?? "Draft board";
    return {
      name,
      chips: <span className="chip chip-draft">Draft board</span>,
      lines: [{
        row: g.boardRow, field: "New board", before: "", after: boardDraftLine(g.board),
        notes: [note("Commit creates its filter and the board in Jira before anything else.")], discardLabel: `Discard ${name}`,
      }],
    };
  }
  if (g.sprintRow) {
    const name = g.sprint?.name ?? "Draft sprint";
    return {
      name,
      chips: <span className="chip chip-draft">Draft sprint</span>,
      lines: [{
        row: g.sprintRow, field: "New sprint", before: "", after: sprintDraftLine(g.sprint),
        notes: [note("Commit creates it in Jira first, then moves its cards into it. Discarding it puts those cards back.")],
        discardLabel: `Discard ${name}`,
      }, ...sprintLines(g.sprintChanges, name)],
    };
  }
  if (g.sprintChanges.length > 0) {
    const name = sprintChangeName(g.sprintChanges[0]);
    return { name, chips: null, lines: sprintLines(g.sprintChanges, name) };
  }

  const held = heldReasons.map((r) => note(`${r.charAt(0).toUpperCase()}${r.slice(1)}.`, "small pending-held"));
  const chips = (
    <>
      {g.createRow && <span className="chip chip-draft">Draft</span>}
      {heldReasons.length > 0 && <span className="chip chip-held">Waiting</span>}
    </>
  );
  if (g.draft && g.createRow) {
    return {
      name: g.key,
      chips,
      lines: [{
        row: g.createRow, field: "New issue", before: "",
        after: <span className="b">{g.draft.summary}</span>,
        notes: [
          note(draftLine(g.draft, project)),
          note("Commit creates it in Jira and swaps the temporary key for the real one."),
          ...held,
        ],
        discardLabel: `Discard ${g.key}`,
      }],
    };
  }

  const lines: Line[] = [
    ...g.links.map(({ row, link }) => ({
      row, field: "Link", before: "", after: `${link.type} (${link.direction}) ${link.toKey} ${link.toSummary}`,
      discardLabel: `Discard link to ${link.toKey}`,
    })),
    ...g.worklogs.map(({ row, log }) => ({
      row, field: "Work log", before: "", after: worklogEntryLine(log),
      discardLabel: `Discard the ${log.timeSpent} entry on ${g.key}`,
    })),
    ...g.moves.map((row) => {
      const { label, from, to } = moveWords(row.entityType, row.beforeVal, row.afterVal);
      return { row, field: label, before: from, after: to, discardLabel: `Discard the ${label.toLowerCase()} move on ${row.entityKey}` };
    }),
    ...g.edits.map((row) => ({
      row, field: fieldLabel(row.field), before: row.beforeVal || "(none)", after: row.afterVal || "(none)",
      notes: unpushableIds.has(row.id)
        ? [note(`Jira will not take ${fieldLabel(row.field)} on ${g.key}: the field is not on that issue's edit screen. The change is kept here until you discard it.`, "small pending-unpushable")]
        : undefined,
      discardLabel: `Discard ${row.field} on ${g.key}`,
    })),
  ];
  // What the issue waits for belongs on the issue, so it rides on its first row.
  if (held.length > 0 && lines.length > 0) lines[0] = { ...lines[0], notes: [...(lines[0].notes ?? []), ...held] };
  return { name: g.key, chips, lines };
}
