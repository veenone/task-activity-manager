import type { Conflict } from "../api";

interface Props {
  profileId: string;
  conflict: Conflict;
  disabled: boolean;
}

// ConflictCard is filled in by the conflict task; until then a held issue
// shows its key and the note from the banner.
export function ConflictCard({ conflict }: Props) {
  return (
    <section className="pending-card pending-card-conflict" role="group" aria-label={conflict.key}>
      <div className="pending-card-head">
        <span className="b">{conflict.key}</span>
        <span className="chip chip-conflict">Conflict</span>
        <span className="pending-card-summary">{conflict.summary}</span>
      </div>
    </section>
  );
}
