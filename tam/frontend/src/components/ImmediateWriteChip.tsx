// ImmediateWriteChip marks the handful of actions that do not wait for
// Commit. Everything else in TAM is journaled: an edit, a move, a create all
// sit in the pending list until the user pushes them, and the whole app is
// built so that closing the window loses nothing. The five sprint writes are
// the exception, for the reasons internal/sprints' package doc gives, and a
// user who has learned that TAM never surprises Jira needs telling.
//
// It is painted in the accent, never in amber. Amber in this app has one
// meaning already: held locally, waiting for Commit, which is the pending
// dot, the draft chip and the moved-row flash. This chip says the exact
// opposite, so wearing that colour would teach the wrong thing at the one
// moment it matters.
//
// It belongs in a dialog's head, beside the title, and in the confirmation
// that stands in for one. It is never put on a menu item: a menu item is a
// way into a dialog, and the dialog is where the claim can be read next to
// what it is a claim about.
export function ImmediateWriteChip() {
  return <span className="chip chip-now">Sends to Jira now</span>;
}
