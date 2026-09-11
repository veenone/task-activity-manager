// Package donerule is the board's own definition of finished: a status the
// board's last column collects. It is a package rather than a method
// because two features now turn on the same answer. The sprint completion
// uses it to decide which cards move out before a sprint closes, and the
// sprint report uses it to decide what "completed" means on a burndown, and
// a completion and a report disagreeing about one sprint is precisely the
// argument that would follow.
//
// It is not the only rule in TAM that answers to the name, and the other
// two are each different from it in a different way.
//
// backend.IsDone matches on the status *name* ("done", "closed",
// "resolved") and powers the Backlog grid's chip, the Epics tree's counts,
// the board's done points and the Sprints view's per-sprint numbers; the
// frontend's statusClass mirrors that list for the chip it paints. It
// stays where it is: it answers a question about a single issue with no
// board in hand, from a status name the cache already carries, and most of
// the places that ask it are not looking at a board at all.
//
// The frontend's lib/unfinished.ts is this rule rather than that one. It
// asks whether the board's last column collects a card's status id, which
// is the question this package answers, on the far side of the bindings
// where the Boards and Sprints views decide what to draw. Two
// implementations of one rule is a real cost, and it is written down here
// so whoever changes either one knows the other is there.
//
// The name rule and the column rule can disagree, and the sprint report is
// the surface that makes it visible: a board whose last column collects a
// status named Released has cards this package calls done that
// backend.IsDone does not. That is a real difference between "the team's
// board says this is finished" and "the issue's status reads as closed",
// and it is recorded rather than smoothed over, because a user comparing
// the Sprints view's done count against a sprint report's completed figure
// deserves to find the reason written down.
package donerule

import "agile-suite/tam/internal/backend"

// Done reports whether a status id is one the board counts as finished,
// which is a status collected by the board's last column.
//
// It returns nil when the columns cannot answer, which is a board with no
// cached columns at all or one whose last column collects no statuses.
// Neither is an answer of "nothing is done": one means the board was never
// synced and the other means the board is configured in a way this rule
// cannot read, and a caller acting on a silent false would move every card
// out of a sprint or draw a burndown that never descends. Callers word that
// refusal themselves, because what a user should do about it differs by
// caller, and LastColumn below is what they word it from.
func Done(cols []backend.BoardColumn) func(statusID string) bool {
	last, ok := LastColumn(cols)
	if !ok || len(last.StatusIDs) == 0 {
		return nil
	}
	set := make(map[string]bool, len(last.StatusIDs))
	for _, id := range last.StatusIDs {
		set[id] = true
	}
	return func(statusID string) bool { return set[statusID] }
}

// LastColumn is the column Done reads, and false when the board has none.
// A caller that has to say why Done returned nil needs the column's name
// for the sentence and needs to tell a board with no columns apart from a
// last column that collects nothing, so both answers come from here rather
// than from a second copy of the same indexing beside each refusal.
func LastColumn(cols []backend.BoardColumn) (backend.BoardColumn, bool) {
	if len(cols) == 0 {
		return backend.BoardColumn{}, false
	}
	return cols[len(cols)-1], true
}
