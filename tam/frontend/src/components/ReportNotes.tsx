// ReportNotes is a report section's notes in a box of their own, set apart
// from the figures they qualify, so a reader can tell the caveats from the
// data at a glance. role="note" names it for a screen reader the same way.
export function ReportNotes({ notes }: { notes: string[] }) {
  if (notes.length === 0) return null;
  return (
    <div className="report-notes" role="note">
      {notes.map((n) => <p key={n}>{n}</p>)}
    </div>
  );
}
