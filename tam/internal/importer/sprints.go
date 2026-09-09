package importer

import (
	"fmt"
	"strings"

	"agile-suite/tam/internal/boardrepo"
)

// The Sprint column is matched by name, because a name is what a person
// types into a spreadsheet: nobody knows a sprint's numeric id, and a file
// written a week ago would name the wrong one if they did. This file is
// where a cell becomes a sprint, and it is the only place that knows the
// matching is case-insensitive.

// sprintIndex is the profile's open sprints, ready to answer a cell. The
// same sprint arrives once per board whose filter reaches it, so entries
// are folded by id; two genuinely different sprints sharing a name are kept
// apart, and asking for that name is an error rather than a coin toss.
type sprintIndex struct {
	byName map[string][]boardrepo.SprintChoice
	// offered is what the error message lists, each sprint with the board
	// it belongs to, in the order the sprint list offered them.
	offered []string
}

func newSprintIndex(open []boardrepo.SprintChoice) sprintIndex {
	idx := sprintIndex{byName: map[string][]boardrepo.SprintChoice{}, offered: []string{}}
	for _, s := range open {
		name := strings.ToLower(strings.TrimSpace(s.Name))
		if name == "" {
			continue
		}
		seen := false
		for _, held := range idx.byName[name] {
			if held.ID == s.ID {
				seen = true
			}
		}
		if seen {
			continue
		}
		idx.byName[name] = append(idx.byName[name], s)
		idx.offered = append(idx.offered, fmt.Sprintf("%s (%s)", strings.TrimSpace(s.Name), s.BoardName))
	}
	return idx
}

// lookup reads one Sprint cell: the sprint it names, or the message that
// disqualifies the row. An empty cell is the backlog, which is a
// destination and not an absence, so it is neither an error nor a sprint.
func (idx sprintIndex) lookup(raw string) (boardrepo.SprintChoice, string) {
	cell := strings.TrimSpace(raw)
	if cell == "" {
		return boardrepo.SprintChoice{}, ""
	}
	found := idx.byName[strings.ToLower(cell)]
	if len(found) == 1 {
		return found[0], ""
	}
	if len(found) > 1 {
		boards := make([]string, 0, len(found))
		for _, s := range found {
			boards = append(boards, s.BoardName)
		}
		return boardrepo.SprintChoice{}, fmt.Sprintf(
			"Sprint %q is open on more than one board (%s), so the row cannot say which one it means",
			cell, strings.Join(boards, ", "))
	}
	if len(idx.offered) == 0 {
		return boardrepo.SprintChoice{}, fmt.Sprintf(
			"Sprint %q is not open: this profile has no synced boards, so refresh the Boards view or clear the cell for the backlog", cell)
	}
	return boardrepo.SprintChoice{}, fmt.Sprintf(
		"Sprint %q is not an open sprint; use %s, or clear the cell for the backlog",
		cell, idx.offerList())
}

// offerListLimit is how many sprints the unknown-sprint message names before
// it starts counting instead. The message is repeated once per bad row, and a
// profile with thirty open sprints would turn a row error into a paragraph.
const offerListLimit = 10

func (idx sprintIndex) offerList() string {
	if len(idx.offered) <= offerListLimit {
		return strings.Join(idx.offered, ", ")
	}
	return fmt.Sprintf("%s, and %d more",
		strings.Join(idx.offered[:offerListLimit], ", "), len(idx.offered)-offerListLimit)
}

// sprintNames are the names the import will actually accept, in offer order.
// The template's dropdown lists these and nothing else, and two kinds of
// entry are left out on purpose. A name carrying its board beside it is one
// the cell match would turn away, since the cell is matched by name alone.
// And a name two different sprints share is one lookup refuses as ambiguous,
// so offering it would put a value in the picker that fails every row it is
// used on. The same sprint reaching two boards is still one sprint and stays,
// which is why this reads the index rather than counting names itself.
func sprintNames(open []boardrepo.SprintChoice) []string {
	idx := newSprintIndex(open)
	names := []string{}
	seen := map[string]bool{}
	for _, s := range open {
		name := strings.TrimSpace(s.Name)
		folded := strings.ToLower(name)
		if name == "" || seen[folded] {
			continue
		}
		seen[folded] = true
		if len(idx.byName[folded]) != 1 {
			continue
		}
		names = append(names, name)
	}
	return names
}
