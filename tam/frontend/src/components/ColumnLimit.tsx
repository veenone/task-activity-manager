import { errMsg, useNotice, usePrompt, useProfile } from "@agile-suite/core";
import type { ColumnView, Profile, Settings } from "../api";
import { limitBreach, limitLine, limitSource } from "../lib/columnLimit";
import { useSetColumnLimit } from "../queries/boards";

// ColumnLimit is the WIP limit under a column's name on the board, and the one
// place a team sets a limit of their own.
//
// A limit Jira sets is printed and nothing else: TAM never writes board
// configuration, so a control here would offer something it cannot do, and a
// local number cannot overrule the board's rule anyway. A column the board
// limits not at all is a button, whether it has a local limit yet or not, and
// that is also how a board with no limits anywhere says so: every column reads
// "No limit, set one" instead of leaving a blank line under its name.
//
// The words and the precedence are lib/columnLimit's, shared with the report's
// capacity table, so the same figure is never attributed one way on screen and
// another on a published page.
export function ColumnLimit({ column, boardId }: { column: ColumnView; boardId: number }) {
  const { activeId } = useProfile<Profile, Settings>();
  const { prompt } = usePrompt();
  const { notice } = useNotice();
  const setLimit = useSetColumnLimit(activeId);
  const line = limitLine(column);
  const breach = limitBreach(column) ? " board-column-limit-breached" : "";

  if (limitSource(column) === "jira") {
    return <span className={`board-column-limit${breach}`}>{line}</span>;
  }

  async function edit() {
    const typed = await prompt({
      title: `Card limit for ${column.name}`,
      defaultValue: column.localMax == null ? "" : String(column.localMax),
      placeholder: "A number of cards, or nothing to remove the limit",
      submitLabel: "Save limit",
    });
    // null is the dialog dismissed, which changes nothing. Empty text is the
    // limit cleared, which is the one destructive thing here: it is what the
    // user typed rather than a side effect of anything else, and typing the
    // number again brings it back.
    if (typed === null) return;
    try {
      await setLimit.mutateAsync({ boardId, column: column.name, limit: typed });
    } catch (e) {
      // Go refuses a word, a negative number and one past its ceiling, and the
      // person who typed the value is who needs to read why.
      await notice({ title: "The limit was not saved", message: errMsg(e), tone: "error" });
    }
  }

  // The label carries the column, because five heads offering "No limit, set
  // one" are five buttons a reader cannot tell apart from their names alone.
  // The visible text leads it, which is what keeps the two the same control.
  const label = line || "No limit, set one";
  return (
    <button
      type="button"
      className={`board-column-limit link-btn${breach}`}
      aria-label={`${label}, for ${column.name}`}
      disabled={setLimit.isPending}
      onClick={() => void edit()}
    >
      {label}
    </button>
  );
}
