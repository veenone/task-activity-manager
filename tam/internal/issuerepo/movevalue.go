package issuerepo

import (
	"strconv"
	"strings"
)

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

// MoveRawName is the name half exactly as it was journaled, empty when the
// cache never held a name for the id. It is what goes into a name column:
// writing the id there instead would fabricate a status name that the next
// drop on the same column then reads back and journals as if it were real.
func MoveRawName(value string) string {
	_, name, _ := strings.Cut(value, moveSep)
	return name
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

// RankValue renders a rank's after_val as "side|neighbour|board". The
// neighbour is the key the card was dropped against, the side says which of
// it, and the board is the one the drop was made on. The board is not
// decoration: one key can sit on two boards whose orders disagree, and the
// commit pass re-derives the neighbour from the board's final order, so
// there is nowhere else for it to learn which board that is.
func RankValue(neighbourKey string, before bool, boardID int) string {
	side := RankSideAfter
	if before {
		side = RankSideBefore
	}
	return side + moveSep + neighbourKey + moveSep + strconv.Itoa(boardID)
}

// ParseRank reads a rank's after_val back. A value with no third segment is
// board 0: that is what a row journaled before the board id joined the value
// carries, and it must still parse rather than take a drag down with it.
func ParseRank(value string) (neighbourKey string, before bool, boardID int) {
	side, rest, _ := strings.Cut(value, moveSep)
	neighbourKey, board, _ := strings.Cut(rest, moveSep)
	boardID, _ = strconv.Atoi(board)
	return neighbourKey, side == RankSideBefore, boardID
}
