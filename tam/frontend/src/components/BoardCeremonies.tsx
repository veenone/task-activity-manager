import { call, errMsg, useConfirm, useNotice } from "@agile-suite/core";
import { PendingInSprint } from "../api";
import type { BoardView, Issue, Sprint } from "../api";
import { useModal } from "../modals";
import { plural } from "../lib/format";
import { CompleteSprintModal } from "./CompleteSprintModal";
import { StartSprintModal } from "./StartSprintModal";

// Which ceremony dialog is open, "" for none.
export type Ceremony = "" | "start" | "complete";

interface Props {
  profileId: string;
  boardId: number;
  open: Ceremony;
  // sprint is the one on screen: the future one a start is about, or the
  // active one a completion is about.
  sprint: Sprint | undefined;
  // sprints are the board's open sprints, which is where both dialogs read
  // their context from: the sprint already active, and the futures a
  // completion can move cards into.
  sprints: Sprint[];
  // view is the board as drawn, unfiltered. The completion reads its
  // incomplete cards from it, so the toolbar's filter cannot hide a card
  // from a list whose whole point is naming every one of them.
  view: BoardView | undefined;
  onClose: () => void;
  onStarted: (sprintId: string, line: string) => void;
  onCompleted: (moveTo: string, line: string) => void;
}

// incompleteCards are the cards outside the board's last column, which is
// what "not finished" means here, on the wire, and in the dialog's own
// sentence. A board with one column has no last column to judge against and
// answers with nothing rather than with everything.
function incompleteCards(view: BoardView): Issue[] {
  const last = view.columns.length - 1;
  if (last < 1) return [];
  const out: Issue[] = [];
  for (const lane of view.lanes) {
    (lane.cells ?? []).forEach((cell, col) => {
      if (col !== last) out.push(...cell);
    });
  }
  return out;
}

// useCompleteGuard is what the Complete button asks before the dialog opens.
//
// A card dragged to Done an hour ago is Done on this board and not in Jira,
// and completing the sprint would send it to the backlog as unfinished. The
// user is stopped here rather than after they have chosen a destination, and
// pointed at the one thing that fixes it. The service re-checks and refuses
// as well, because a drag can land between this answer and the push.
export function useCompleteGuard(profileId: string) {
  const { confirm } = useConfirm();
  const { notice } = useNotice();
  const { openModal } = useModal();

  return async function askFirst(sprintId: number): Promise<boolean> {
    let pending = 0;
    try {
      pending = await call(() => PendingInSprint(profileId, sprintId));
    } catch (e) {
      // The count could not be read. The completion itself re-checks, so
      // this is worth saying and not worth blocking on.
      void notice({ title: "The pending changes could not be counted", message: errMsg(e), tone: "error" });
      return true;
    }
    if (pending === 0) return true;
    const ok = await confirm({
      title: "Commit before completing this sprint?",
      message: `${plural(pending, "pending change belongs", "pending changes belong")} to cards staying in this sprint. Until they are committed Jira has not been told those cards moved, so completing the sprint would send finished work to the backlog as unfinished.`,
      confirmLabel: "Show pending changes",
      cancelLabel: "Cancel",
      // Not a destructive confirm: the way out of this is to look at the
      // pending changes, and the red button belongs to the actions that
      // throw something away.
      danger: false,
    });
    if (ok) openModal("pending");
    return false;
  };
}

// BoardCeremonies is the two sprint dialogs and the context they need,
// gathered off BoardsView so the view keeps to the board it draws.
export function BoardCeremonies({
  profileId, boardId, open, sprint, sprints, view, onClose, onStarted, onCompleted,
}: Props) {
  if (!sprint) return null;
  if (open === "start") {
    return (
      <StartSprintModal
        profileId={profileId}
        boardId={boardId}
        sprint={sprint}
        active={sprints.find((s) => s.state === "active" && s.id !== sprint.id)}
        onClose={onClose}
        onStarted={onStarted}
      />
    );
  }
  if (open === "complete") {
    return (
      <CompleteSprintModal
        profileId={profileId}
        boardId={boardId}
        sprint={sprint}
        futures={sprints.filter((s) => s.state === "future")}
        incomplete={view ? incompleteCards(view) : []}
        lastColumn={view?.columns[view.columns.length - 1]?.name ?? "the last column"}
        onClose={onClose}
        onCompleted={onCompleted}
      />
    );
  }
  return null;
}
