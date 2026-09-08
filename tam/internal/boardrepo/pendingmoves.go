package boardrepo

import (
	"strconv"

	"agile-suite/tam/internal/backend"
)

// Putting the journal's pending board moves onto a list of cards: the keys
// a sprint move brought in, the transition and sprint overrides, and the
// order a rank asks for. It is its own file because two callers need it and
// they want different things from the rest of a board read: view.go draws
// lanes, caps, and counts, while cellorder.go wants nothing but the order
// the commit pass ranks against.

// withMovedIn adds the keys of every card a pending sprint move has brought
// into the sprint being viewed. A sprint's cards come from its own
// board_issue rows, which are Jira's from the last sync, so a card that just
// moved in is not among them and no placing logic further down can rescue
// it: the key has to be in the list before the cards are read.
func withMovedIn(boardKeys []string, moves []backend.PendingMove, sprintID string) []string {
	if sprintID == "" {
		return boardKeys
	}
	have := make(map[string]bool, len(boardKeys))
	for _, k := range boardKeys {
		have[k] = true
	}
	keys := boardKeys
	for _, m := range moves {
		if !m.HasSprint || m.SprintID != sprintID || have[m.Key] {
			continue
		}
		if len(keys) == len(boardKeys) {
			keys = append([]string(nil), boardKeys...)
		}
		have[m.Key] = true
		keys = append(keys, m.Key)
	}
	return keys
}

// applyMoves overrides each card with the intent the journal holds for it,
// and drops the cards a pending sprint move has taken out of the sprint
// being viewed. A whole-board view has no sprint to leave, so it drops
// nothing.
//
// Each id is overridden with the name journaled beside it. DonePoints
// counts by backend.IsDone over the status name, so a card whose id said
// Done while its name still said In Progress would be drawn in one column
// and counted as if it were in another, on any path that repaints a row
// without replaying the journal onto it.
func applyMoves(cards []backend.Issue, moves []backend.PendingMove, sprintID string) []backend.Issue {
	if len(moves) == 0 {
		return cards
	}
	byKey := make(map[string]backend.PendingMove, len(moves))
	for _, m := range moves {
		byKey[m.Key] = m
	}
	out := make([]backend.Issue, 0, len(cards))
	for _, c := range cards {
		if m, ok := byKey[c.Key]; ok {
			if m.HasTransition {
				c.StatusID, c.Status = m.StatusID, m.StatusName
			}
			if m.HasSprint {
				if sprintID != "" && m.SprintID != sprintID {
					continue
				}
				c.SprintID, c.SprintName = m.SprintID, m.SprintName
			}
		}
		out = append(out, c)
	}
	return out
}

// rankCards puts every card with a pending rank immediately before or after
// the card it was dropped against. A cell renders the cards in the order
// they arrive in, so reordering the slice is the only way to honour a rank
// that was never written to the cache. A neighbour that is not in the same
// cell leaves the order alone: a rank measured against a card the user
// cannot see would move this one somewhere nobody asked for.
func rankCards(cards []backend.Issue, moves []backend.PendingMove, byStatus map[string]int, draftColumn int, swimlane string) []backend.Issue {
	ranked := false
	for _, m := range moves {
		if m.HasRank {
			ranked = true
			break
		}
	}
	if !ranked {
		return cards
	}
	// A cell is a column and a lane. The column index cannot contain a
	// slash, so joining the two on one is unambiguous whatever the lane's
	// own value is.
	cellOf := func(c backend.Issue) (string, bool) {
		col, ok := placeCard(c, byStatus, draftColumn)
		if !ok {
			return "", false
		}
		id, _ := laneOf(c, swimlane)
		return strconv.Itoa(col) + "/" + id, true
	}
	out := append([]backend.Issue(nil), cards...)
	for _, m := range moves {
		if !m.HasRank {
			continue
		}
		i, j := indexOfKey(out, m.Key), indexOfKey(out, m.RankNeighbour)
		if i < 0 || j < 0 || i == j {
			continue
		}
		here, okHere := cellOf(out[i])
		there, okThere := cellOf(out[j])
		if !okHere || !okThere || here != there {
			continue
		}
		card := out[i]
		out = append(out[:i], out[i+1:]...)
		at := indexOfKey(out, m.RankNeighbour)
		if !m.RankBefore {
			at++
		}
		out = append(out[:at], append([]backend.Issue{card}, out[at:]...)...)
	}
	return out
}

// indexOfKey is where a key sits in the card order, or -1.
func indexOfKey(cards []backend.Issue, key string) int {
	for i, c := range cards {
		if c.Key == key {
			return i
		}
	}
	return -1
}
