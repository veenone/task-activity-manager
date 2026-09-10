package sprints

import (
	"context"
	"fmt"
	"log"

	"agile-suite/tam/internal/errtext"
)

// The board cache's bookkeeping after a ceremony: which cards each scope
// holds now, and the board's sprint list. All of it is written after Jira
// has already moved, so none of it may fail the ceremony that produced it.

// refreshMembership records where the cards a completion pushed have gone:
// out of the sprint they were in, and into the scope they were sent to. Both
// halves are needed, and only the first one used to be written. The banner
// says twelve cards moved to Sprint 15, the picker switches to Sprint 15,
// and Sprint 15 draws exactly what it drew before, because the ceremony
// bypasses the journal and leaves no pending move for the view to fold in.
//
// moveTo is the destination: a sprint id, or the empty string for the
// board's own list, which is where a completion into the backlog leaves
// them and is a scope like any other.
func (s *Service) refreshMembership(ctx context.Context, profileID string, boardID int, sprintID, moveTo string, moved []string) {
	if len(moved) == 0 {
		return
	}
	s.rewriteScope(ctx, profileID, boardID, sprintID, func(cached []string) []string {
		return without(cached, moved)
	})
	s.rewriteScope(ctx, profileID, boardID, moveTo, func(cached []string) []string {
		return appended(cached, moved)
	})
}

// rewriteScope reads one scope of the board's cached membership and writes
// back what fn makes of it.
//
// It reads and edits rather than replacing, because the cached list and the
// completion's own are not the same list. The cached scope is the board's
// rank order, which is the order the view reads back, and it came from the
// board's own filter; the completion's search orders by key and is scoped by
// project and issue type, so writing its answer back would leave the sprint
// alphabetical until the next boards sync and could insert keys the board
// never drew.
//
// It is best effort and logged: the cards have moved in Jira whatever the
// cache says, and failing the ceremony over a local write would report a
// move that happened as one that did not. A read that fails writes nothing,
// since a guessed membership is worse than a stale one.
func (s *Service) rewriteScope(ctx context.Context, profileID string, boardID int, scopeID string, fn func([]string) []string) {
	cached, err := s.store.SprintIssues(ctx, profileID, boardID, scopeID)
	if err != nil {
		log.Printf("tam: the cached cards of %s could not be read after a completion moved cards: %v", scopeName(scopeID), err)
		return
	}
	if err := s.store.ReplaceSprintIssues(ctx, profileID, boardID, scopeID, fn(cached)); err != nil {
		log.Printf("tam: the cached cards of %s could not be rewritten after a completion moved cards: %v", scopeName(scopeID), err)
	}
}

// scopeName says which scope a log line is about: a sprint by its id, or the
// board's own list for the empty scope the backlog writes back to.
func scopeName(scopeID string) string {
	if scopeID == "" {
		return "the board's own list"
	}
	return "sprint " + scopeID
}

// without is the cached order with the keys that left taken out of it.
func without(cached, gone []string) []string {
	left := make(map[string]bool, len(gone))
	for _, key := range gone {
		left[key] = true
	}
	out := make([]string, 0, len(cached))
	for _, key := range cached {
		if !left[key] {
			out = append(out, key)
		}
	}
	return out
}

// appended is the cached order with the keys that arrived added to its end,
// each of them once. They go last because the destination's rank order is
// Jira's to hand back at the next boards sync, and the end is where a card
// moved into a sprint sits until it does.
func appended(cached, arrived []string) []string {
	held := make(map[string]bool, len(cached))
	for _, key := range cached {
		held[key] = true
	}
	out := append(make([]string, 0, len(cached)+len(arrived)), cached...)
	for _, key := range arrived {
		if held[key] {
			continue
		}
		held[key] = true
		out = append(out, key)
	}
	return out
}

// refreshSprints re-reads that board's sprints and nothing else, so the
// picker shows the state the ceremony just produced. It is not a boards
// sync: that walks every board's columns, sprints and membership, takes
// minutes on a real project, and would be refused outright by the lock the
// ceremony itself is holding.
//
// It answers with what went wrong rather than only logging it. The ceremony
// has happened and cannot be failed over a cache read, but a sprint list
// that was not re-read still says "future" for the sprint now running, so
// the toolbar offers Start for it and Jira answers the second start with a
// 400. The caller carries this back as a note beside its own success, which
// is how the user finds out to press Refresh.
func (s *Service) refreshSprints(ctx context.Context, b lifecycle, profileID string, boardID int) error {
	return s.reReadSprints(ctx, b, profileID, boardID, "the ceremony", true)
}

// refreshSprintsAllowEmpty is the same re-read without the refusal, for the
// one caller that can make an empty answer true: a delete that has just
// removed a board's only sprint leaves BoardSprints with nothing left to
// report, and that emptiness is the sprint's real absence rather than the
// ambiguous 400 the refusal exists to distrust.
func (s *Service) refreshSprintsAllowEmpty(ctx context.Context, b lifecycle, profileID string, boardID int) error {
	return s.reReadSprints(ctx, b, profileID, boardID, "a sprint was deleted", false)
}

// reReadSprints is the body both of the above share, so the call, the write
// and the two error sentences exist once rather than twice. occasion names
// what has just happened, for the log lines; refuseEmpty is the only real
// difference between the two callers, and the block it guards carries the
// reason.
func (s *Service) reReadSprints(ctx context.Context, b lifecycle, profileID string, boardID int, occasion string, refuseEmpty bool) error {
	list, err := b.BoardSprints(ctx, boardID)
	if err != nil {
		log.Printf("tam: board %d sprints could not be re-read after %s: %v", boardID, occasion, err)
		return fmt.Errorf("the board's sprint list could not be re-read: %w", err)
	}
	if refuseEmpty && len(list) == 0 {
		// A ceremony has just started or completed a sprint on this board, so
		// the board demonstrably has one and an empty list is not the truth
		// about it. It is what a single 400 on the sprint endpoint looks like
		// from here: core/jira turns any 400 on the first page into
		// ErrNoSprints, for the kanban board that genuinely has none, and the
		// backend turns that into an empty slice and no error. Writing it
		// would delete every sprint row of the board, empty the picker, and
		// drop the sprint length the date suggestion is built from, with
		// nothing reported anywhere because the call did not fail.
		log.Printf("tam: board %d answered the ceremony with no sprints at all, which cannot be true of a board a ceremony has just run on; the cached list is left as it was", boardID)
		return fmt.Errorf("the board answered with no sprints at all, so its cached list was left as it was")
	}
	if err := s.store.ReplaceSprints(ctx, profileID, boardID, list); err != nil {
		log.Printf("tam: board %d sprints could not be cached after %s: %v", boardID, occasion, err)
		return fmt.Errorf("the board's sprint list could not be cached: %w", err)
	}
	return nil
}

// note is what a ceremony reports beside its own success when the cache
// bookkeeping after it did not land: one line, and the one thing the user
// can do about it. Jira's own words reach here off the wire, so they go
// through the same reduction a sync summary's do.
//
// It says "TAM's local copy" rather than naming the sprint list specifically,
// because it wraps more than one failure: refreshSprints and
// refreshSprintsAllowEmpty leave the list stale, but a delete's note can also
// carry a failed clearIssues, where what is stale is the sprint name still
// showing on the cards that were in it rather than anything in the list
// itself.
func note(err error) string {
	if err == nil {
		return ""
	}
	return errtext.Line(err) + ", so TAM's local copy may be out of date; press Refresh."
}
