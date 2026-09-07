import { plural } from "../lib/format";

interface Props {
  // error is the message a failed boards Refresh came back with, empty
  // when the last one succeeded or none has run.
  error: string;
  // dropped names the boards a pass skipped, each with its reason.
  dropped: string[];
  // unavailable is a pass that found no Agile API at all. It is only worth
  // a banner when boards are still on screen from an earlier sync; with
  // none, BoardBody's empty state already says it.
  unavailable: boolean;
  hasBoards: boolean;
  canRetry: boolean;
  onRetry: () => void;
}

// BoardsBanner is the slot above the board where a boards pass reports on
// itself: what failed, which boards it had to skip, and whether this Jira
// answered with an Agile API at all. The pass may be the board's own
// Refresh or the boards half of an ordinary sync, which is the one most
// users ever run.
//
// A Refresh that fails used to end with the label vanishing and nothing
// said, so a 403, a write that could not land, and "a commit is already
// running" all looked like a Refresh that had simply finished.
export function BoardsBanner({ error, dropped, unavailable, hasBoards, canRetry, onRetry }: Props) {
  const noAgile = unavailable && hasBoards;
  if (!error && dropped.length === 0 && !noAgile) return null;
  return (
    <>
      {error && (
        <div className="pending-banner pending-banner-warn">
          <p>
            Could not refresh the boards: {error}{" "}
            <button type="button" className="btn" disabled={!canRetry} onClick={onRetry}>Retry</button>
          </p>
        </div>
      )}
      {dropped.length > 0 && (
        <div className="pending-banner">
          <p>{`${plural(dropped.length, "board", "boards")} ${dropped.length === 1 ? "was" : "were"} skipped: ${dropped.join(", ")}`}</p>
        </div>
      )}
      {noAgile && (
        <div className="pending-banner pending-banner-warn">
          <p>This Jira answered with no boards, so nothing below was refreshed.</p>
        </div>
      )}
    </>
  );
}
