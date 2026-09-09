import { errMsg, useNotice } from "@agile-suite/core";
import type { Issue, SprintOption } from "../api";
import { useMoveToSprint } from "../queries/boards";
import { duplicateNameIds, sprintOptionLabel } from "../lib/sprintOptions";

interface Props {
  profileId: string;
  issue: Issue;
  // sprints are every open sprint the field can offer: a board's own list
  // when a board is on screen, or the profile-wide list from the Backlog and
  // the Epics tree, which have no board of their own. Both shapes are
  // SprintOption, so a caller passes either straight through with no
  // mapping; an empty list still lets the field move a card to the backlog.
  sprints: SprintOption[];
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
  const dupIds = duplicateNameIds(sprints);

  return (
    <span className="sprint-field">
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
          <option key={s.id} value={String(s.id)}>{sprintOptionLabel(s, dupIds)}</option>
        ))}
      </select>
      {/* Guidance beside the select, not a replacement for it: the backlog
          and the issue's own sprint are always there to choose, even on a
          profile whose boards have never been synced. */}
      {sprints.length === 0 && (
        <span className="muted small sprint-field-empty">No sprints yet, sync a board first</span>
      )}
    </span>
  );
}
