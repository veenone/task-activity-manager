interface Props {
  // line is the whole sentence: where the card cannot go and what Jira
  // offered instead. Empty for no warning at all.
  line: string;
  canPutBack: boolean;
  busy: boolean;
  onPutBack: () => void;
}

// BoardMoveBanner is the one warning above the board: a card was dropped
// somewhere Jira says it cannot reach from where it is. One banner, not a
// stack, because a standup's worth of drags would otherwise bury the board
// it is warning about, and the card carries its own marker as well.
export function BoardMoveBanner({ line, canPutBack, busy, onPutBack }: Props) {
  if (!line) return null;
  return (
    <div className="pending-banner pending-banner-warn" role="status">
      <p>
        {line}{" "}
        {canPutBack && (
          <button type="button" className="btn" disabled={busy} onClick={onPutBack}>Put it back</button>
        )}
      </p>
      <p className="muted small">Commit would refuse this move, so it is worth undoing now rather than at the end of the standup.</p>
    </div>
  );
}
