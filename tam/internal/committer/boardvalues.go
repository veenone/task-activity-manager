package committer

import (
	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// What a journaled board value means against what Jira holds now: the three
// answers a remote read gives one row, and the words a card, a dialog, or a
// conflict prints it in. The pass itself is in boards.go and ranks.go.

// backlogLabel is what an empty sprint value reads as. Leaving every sprint
// is a destination, not an absence, and a conflict card that showed an
// empty cell for it would be unreadable.
const backlogLabel = "Backlog"

// The three answers a fresh remote read gives one journaled board move.
const (
	moveSatisfied = iota
	movePush
	moveConflict
)

// classifyMove compares the journal's ids with the remote's, never the
// "id|Name" text: a status name that differs between the board
// configuration and the cached row would make a card that never moved look
// like a conflict.
func classifyMove(p journal.PendingChange, remoteID string) int {
	switch {
	case issuerepo.MoveID(p.AfterVal) == remoteID:
		return moveSatisfied
	case issuerepo.MoveID(p.BeforeVal) == remoteID:
		return movePush
	default:
		return moveConflict
	}
}

// remoteValue is the id a board row is checked against.
func remoteValue(entityType string, remote backend.Issue) string {
	if entityType == issuerepo.EntitySprintMove {
		return remote.SprintID
	}
	return remote.StatusID
}

// moveLabel is what a journaled board value reads as. issuerepo.FieldValue
// is not used anywhere in this file: it knows the six editable fields and
// would render an empty string for a status or a sprint.
func moveLabel(entityType, value string) string {
	if entityType == issuerepo.EntitySprintMove && issuerepo.MoveID(value) == "" {
		return backlogLabel
	}
	return issuerepo.MoveName(value)
}

// remoteLabel is what Jira holds now, for the conflict card.
func remoteLabel(entityType string, remote backend.Issue) string {
	if entityType == issuerepo.EntitySprintMove {
		switch {
		case remote.SprintID == "":
			return backlogLabel
		case remote.SprintName != "":
			return remote.SprintName
		}
		return remote.SprintID
	}
	if remote.Status != "" {
		return remote.Status
	}
	return remote.StatusID
}
