import type { BoardView, Sprint } from "../api";
import { MAX_CARDS_PER_VIEW } from "../api";
import { day, formatWhen, plural, points } from "../lib/format";

interface SummaryProps {
  view: BoardView;
  sprint: Sprint | undefined;
  lastSynced: string;
}

// BoardSummaryLine is the line that orients a standup: which sprint, how far
// through it, and how fresh the cards under it are.
export function BoardSummaryLine({ view, sprint, lastSynced }: SummaryProps) {
  const sentences: string[] = [];
  if (sprint) {
    const ends = sprint.state === "future" ? day(sprint.startDate) : day(sprint.endDate);
    const when = ends ? `, ${sprint.state === "future" ? "starts" : "ends"} ${ends}` : "";
    sentences.push(`${sprint.name}, ${sprint.state}${when}.`);
  }
  // Both halves come from the backend, which had every card in hand:
  // walking the drawn cells here would read "200 of 900 points done" on a
  // board whose Done column is capped.
  const total = view.columns.reduce((sum, c) => sum + c.points, 0);
  if (total > 0) {
    sentences.push(`${points(view.donePoints)} of ${points(total)} points done.`);
  }
  const synced = formatWhen(lastSynced);
  if (synced) sentences.push(`Synced ${synced}.`);
  if (sentences.length === 0) return null;
  return <p className="board-summary">{sentences.join(" ")}</p>;
}

// BoardNotes is the honesty block under the board: whichever counts are not
// zero, each naming what it leaves out, and the caveat that says what this
// board is and is not.
export function BoardNotes({ view }: { view: BoardView }) {
  const unmapped = view.unmapped > 0
    ? `${plural(view.unmapped, "card is", "cards are")} not on the board (${view.unmappedStatuses.join(", ")})`
    : "";
  const notSynced = view.notSynced > 0
    ? `${plural(view.notSynced, "card", "cards")} on this board ${view.notSynced === 1 ? "has" : "have"} not been synced`
    : "";
  return (
    <div className="board-unmapped">
      {unmapped && <p className="muted small">{unmapped}</p>}
      {notSynced && <p className="muted small">{notSynced}</p>}
      {view.capped && (
        <p className="muted small">{`Showing the first ${MAX_CARDS_PER_VIEW} cards of this board`}</p>
      )}
    </div>
  );
}
