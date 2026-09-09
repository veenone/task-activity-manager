import { errMsg, useNotice } from "@agile-suite/core";
import type { Issue, Sprint } from "../api";
import { useMoveToSprint } from "../queries/boards";

interface Props {
  profileId: string;
  issue: Issue;
  // sprints are the board's open sprints, handed down by whoever holds a
  // board. Away from a board there is no such list, and the panel prints the
  // sprint's name instead of offering a choice it cannot fill.
  sprints: Sprint[];
  // busy is a sync or a commit in flight, the same guard the edit form takes:
  // the row refresh that follows either one would land on top of the move.
  busy: boolean;
}

// SprintField is the detail panel's sprint, as a choice rather than a fact.
// It journals through the same MoveIssueToSprint binding the card's own move
// menu uses, so the pending row, the discard, and the commit path are the
// ones Phase 3b already built; nothing here reaches Jira.
export function SprintField({ profileId, issue, sprints, busy }: Props) {
  const move = useMoveToSprint(profileId);
  const { notice } = useNotice();
  // The issue's own sprint may be a closed one, which the board's picker
  // never offers. Dropping it would leave the select showing the backlog for
  // a card that is in a sprint, so it is added as its own option.
  const known = sprints.some((s) => String(s.id) === issue.sprintId);
  const disabled = busy || move.isPending;

  return (
    <select
      className="detail-input detail-input-inline"
      aria-label="Sprint"
      value={issue.sprintId}
      disabled={disabled}
      onChange={(e) =>
        move.mutate(
          { key: issue.key, sprintId: e.target.value },
          {
            onError: (err) =>
              void notice({ title: "The card could not be moved", message: errMsg(err), tone: "error" }),
          },
        )
      }
    >
      <option value="">The backlog</option>
      {!known && issue.sprintId && (
        <option value={issue.sprintId}>{issue.sprintName || `Sprint ${issue.sprintId}`}</option>
      )}
      {sprints.map((s) => (
        <option key={s.id} value={String(s.id)}>{s.name}</option>
      ))}
    </select>
  );
}
