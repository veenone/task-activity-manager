import type { BoardView, Sprint } from "../api";
import { formatWhen, plural } from "../lib/format";

// MAX_CARDS_PER_VIEW mirrors boardrepo.MaxCardsPerView, so the capped line
// names the same number the backend stopped at.
const MAX_CARDS_PER_VIEW = 2000;

// day renders a sprint's own start or end as a calendar day. It is not a
// second wording for the sync age, which formatWhen owns and this file uses
// for exactly that.
function day(iso: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleDateString(undefined, { day: "numeric", month: "short" });
}

// points trims a float that is really a whole number, so 27 does not print
// as 27.0 and 2.5 still prints as 2.5.
function points(n: number): string {
  return String(Math.round(n * 10) / 10);
}

interface SummaryProps {
  view: BoardView;
  sprint: Sprint | undefined;
  lastSynced: string;
}

// BoardSummaryLine is the line that orients a standup: which sprint, how far
// through it, and how fresh the cards under it are. The done points are the
// last column's, which is the board's own definition of done: whatever its
// rightmost column collects.
export function BoardSummaryLine({ view, sprint, lastSynced }: SummaryProps) {
  const sentences: string[] = [];
  if (sprint) {
    const ends = sprint.state === "future" ? day(sprint.startDate) : day(sprint.endDate);
    const when = ends ? `, ${sprint.state === "future" ? "starts" : "ends"} ${ends}` : "";
    sentences.push(`${sprint.name}, ${sprint.state}${when}.`);
  }
  const total = view.columns.reduce((sum, c) => sum + c.points, 0);
  if (total > 0) {
    const done = view.columns.length > 0 ? view.columns[view.columns.length - 1].points : 0;
    sentences.push(`${points(done)} of ${points(total)} points done.`);
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
      <p className="muted small">
        Columns and cards follow your board's configuration. Quick filters and the board's own swimlanes are not applied.
      </p>
    </div>
  );
}
