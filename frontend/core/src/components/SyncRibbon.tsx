interface Props {
  // The sentence to show. core/jira's Humanize produces it from what Jira
  // answered; anything it does not recognise arrives here as the raw error,
  // which is still better than nothing.
  message: string;
}

// SyncRibbon is how both apps report a failed sync: the status bar's whole
// width, in the place the bar already occupies. A failure is the one thing
// the bar reports that the user has to act on, and as a red string in the
// corner beside a count and a timestamp it read as a stray value (#313).
//
// The caller owns the bar: it adds statusbar-failed to its own footer, which
// is what colours the chrome and lets it grow for a sentence that wraps.
// Only the footer can do that, so this is the ribbon's contents, not the bar.
export function SyncRibbon({ message }: Props) {
  return (
    <div className="sync-ribbon" role="alert" data-testid="sync-error">
      <span className="sync-ribbon-mark" aria-hidden="true">
        !
      </span>
      <span>
        <strong>Sync failed.</strong> {message}
      </span>
    </div>
  );
}
