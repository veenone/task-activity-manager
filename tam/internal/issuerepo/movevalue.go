package issuerepo

import "strings"

// The values a board move journals are packed into the one before_val and
// after_val column the journal gives every row, the way a link row already
// packs its three parts into a field. This file is the only place that
// knows the packing: the writes, the reverts, the pending-move read, and
// the commit pass all go through it, so there is one definition of what a
// board row's text means.

// moveSep separates the parts of a board row's value.
const moveSep = "|"

// The two sides a rank can be dropped on, as the after_val spells them.
const (
	RankSideBefore = "before"
	RankSideAfter  = "after"
)

// MoveValue renders a transition's or a sprint move's value as "id|Name":
// the committer splits on the pipe and pushes the id, and the Pending
// changes dialog and the Activity tab read the name. A move to the backlog
// has neither and renders empty, which is a destination and not an absence.
func MoveValue(id, name string) string {
	if id == "" && name == "" {
		return ""
	}
	return id + moveSep + name
}

// MoveID is the id half of a value MoveValue built. Every comparison
// between two board values goes through here rather than over the whole
// text: a status name that differs between the board configuration and the
// cached issue row would otherwise make a card dragged home look like a
// move to somewhere new.
func MoveID(value string) string {
	id, _, _ := strings.Cut(value, moveSep)
	return id
}

// MoveName is the name half, falling back to the id when the cache never
// held a name for it, so a dialog prints something a person can look up
// rather than an empty cell.
func MoveName(value string) string {
	id, name, ok := strings.Cut(value, moveSep)
	if !ok || name == "" {
		return id
	}
	return name
}

// RankValue renders a rank's after_val as "side|neighbour". The neighbour
// is the key the card was dropped against; the side says which of it.
func RankValue(neighbourKey string, before bool) string {
	side := RankSideAfter
	if before {
		side = RankSideBefore
	}
	return side + moveSep + neighbourKey
}

// ParseRank reads a rank's after_val back. Anything past the neighbour is
// ignored, so a later segment can be added to the row without this having
// to be the thing that breaks.
func ParseRank(value string) (neighbourKey string, before bool) {
	side, rest, _ := strings.Cut(value, moveSep)
	neighbourKey, _, _ = strings.Cut(rest, moveSep)
	return neighbourKey, side == RankSideBefore
}
