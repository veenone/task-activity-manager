package reports

import (
	"sort"
	"strings"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/sprintdate"
)

// Depth is how many closed sprints a velocity table covers.
//
// Six is Jira's own velocity chart minus one, picked because it is the
// number a user coming from Jira already expects to see and because
// nothing better was argued for. It is a placeholder in the honest sense:
// no one has measured how many sprints a team actually reads before the
// older rows stop informing the next estimate, and when someone does, this
// is the constant that changes.
const Depth = 6

// VelocityRow is one closed sprint's line in the velocity table.
//
// Unit rides on the row rather than on the table, because a board that
// switched from story points to counting cards halfway through the year
// would otherwise have two different quantities averaged into a single
// meaningless number. A reader can see the switch; an average cannot.
type VelocityRow struct {
	SprintID   int    `json:"sprintId"`
	SprintName string `json:"sprintName"`
	Unit       string `json:"unit"`
	UnitReason string `json:"unitReason"`
	// Committed and Completed come from the same reconstruction one
	// sprint's report does, so a row here and that sprint's own report
	// can never disagree.
	Committed float64 `json:"committed"`
	Completed float64 `json:"completed"`
	// Truncated is true when any of the sprint's issues came back with a
	// cut short changelog, so the row is built on a partial history and
	// the table has to say so.
	Truncated bool `json:"truncated"`
}

// Velocity reconstructs the last Depth closed sprints, oldest row first,
// which is the order a bar chart is read in.
//
// histories is each sprint's issues with their changelogs, keyed by sprint
// id. A sprint with no entry at all was not fetched and is left out; a
// sprint with an entry holding no issues was fetched and found empty, and
// gets its row.
//
// Two kinds of sprint are dropped rather than shown as zeros. One with
// dates TAM cannot read cannot be reconstructed at all, and one that is
// not closed does not belong in a velocity table, whose whole point is
// finished work. A zero row would read as a sprint that delivered nothing,
// which is a worse answer than a shorter table.
//
// It returns no error, deliberately. Every reason a sprint drops out is a
// property of that one sprint, and failing the whole table because the
// oldest of six has an unreadable start date would take away five good
// rows to report one bad one.
func Velocity(sprints []backend.Sprint, done func(string) bool, histories map[int][]backend.IssueHistory, now time.Time, loc *time.Location) []VelocityRow {
	type dated struct {
		sprint backend.Sprint
		end    time.Time
	}
	var closed []dated
	for _, sp := range sprints {
		if !isClosed(sp) {
			continue
		}
		if _, ok := histories[sp.ID]; !ok {
			continue
		}
		end, err := sprintdate.Parse(sp.EndDate)
		if err != nil {
			continue
		}
		closed = append(closed, dated{sprint: sp, end: end})
	}
	// Newest first to take the last Depth of them, with the id breaking a
	// tie so two sprints that ended at the same minute, which a team
	// closing a rollover does routinely, always come out in the same
	// order rather than in whatever order the board listed them.
	sort.SliceStable(closed, func(i, j int) bool {
		if closed[i].end.Equal(closed[j].end) {
			return closed[i].sprint.ID > closed[j].sprint.ID
		}
		return closed[i].end.After(closed[j].end)
	})
	if len(closed) > Depth {
		closed = closed[:Depth]
	}

	rows := make([]VelocityRow, 0, len(closed))
	for i := len(closed) - 1; i >= 0; i-- {
		sp := closed[i].sprint
		s, err := Build(sp, done, histories[sp.ID], now, loc)
		if err != nil {
			continue
		}
		rows = append(rows, VelocityRow{
			SprintID:   s.SprintID,
			SprintName: s.SprintName,
			Unit:       s.Unit,
			UnitReason: s.UnitReason,
			Committed:  s.Committed,
			Completed:  s.Completed,
			Truncated:  len(s.Truncated) > 0,
		})
	}
	return rows
}

// isClosed matches Jira's own sprint state the way internal/sprints does,
// without regard to case and with the spaces trimmed off.
func isClosed(sp backend.Sprint) bool {
	return strings.EqualFold(strings.TrimSpace(sp.State), "closed")
}
