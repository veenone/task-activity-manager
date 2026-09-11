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

// VelocitySprints is which sprints a velocity table is built from and in
// what order: the closed ones whose start and end dates can both be read,
// the last Depth of those, oldest row first, which is the order a bar
// chart is read in.
//
// It is exported because a caller assembling the table out of stored
// reports has to know which sprints to look for before it has any of them
// in hand, and a second copy of this rule would be a table whose rows and
// whose order could drift from the one it was built against. It is the
// only place the rule is written.
//
// Two kinds of sprint are dropped here rather than shown as zeros. One
// that is not closed does not belong in a table of finished work, and one
// whose dates TAM cannot read cannot be reconstructed at all. Both dates
// are parsed, not only the end, because a sprint dropped here is a sprint
// nobody fetches: an unreadable start that got through would cost several
// pages of the heaviest read in the app before Build refused the series
// anyway. A zero row would read as a sprint that delivered nothing, which
// is a worse answer than a shorter table.
//
// One unreportable sprint still gets through: one whose dates both parse
// and run backwards, an end before its start. Build refuses that with the
// same ErrNoDates, and a caller assembling a table has to drop it there,
// because nothing short of parsing both dates here would tell it apart
// from a sprint that is fine.
func VelocitySprints(sprints []backend.Sprint) []backend.Sprint {
	type dated struct {
		sprint backend.Sprint
		end    time.Time
	}
	var closed []dated
	for _, sp := range sprints {
		if !Closed(sp) {
			continue
		}
		if _, err := sprintdate.Parse(sp.StartDate); err != nil {
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
	out := make([]backend.Sprint, 0, len(closed))
	for i := len(closed) - 1; i >= 0; i-- {
		out = append(out, closed[i].sprint)
	}
	return out
}

// Row is the one place a reconstructed series becomes a velocity row.
//
// It is exported for the same reason VelocitySprints is: a table whose
// rows come partly from series built just now and partly from series read
// back out of the store has to turn both into a row the same way, or the
// two halves of one table can disagree about a sprint for no reason a
// reader could see.
func Row(s Series) VelocityRow {
	return VelocityRow{
		SprintID:   s.SprintID,
		SprintName: s.SprintName,
		Unit:       s.Unit,
		UnitReason: s.UnitReason,
		Committed:  s.Committed,
		Completed:  s.Completed,
		Truncated:  len(s.Truncated) > 0,
	}
}

// Closed matches Jira's own sprint state the way internal/sprints does,
// without regard to case and with the spaces trimmed off.
//
// It is exported because storing a sprint's series and putting that sprint
// in a velocity table are the same question asked twice: a caller that
// caches what this package calls closed, on its own reading of the word,
// could cache a sprint the table below will not show and refuse to cache
// one it will.
func Closed(sp backend.Sprint) bool {
	return strings.EqualFold(strings.TrimSpace(sp.State), "closed")
}
