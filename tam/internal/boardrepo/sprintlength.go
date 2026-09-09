package boardrepo

import (
	"context"
	"fmt"
	"math"
	"sort"

	"agile-suite/tam/internal/sprintdate"
)

// recentSprints is how many closed sprints the length is taken from. Three
// is enough to shrug off one holiday sprint and short enough that a cadence
// the team changed last month is the cadence it suggests.
const recentSprints = 3

// closedSprintsSQL reads the closed sprints of one board that carry both
// dates. A sprint missing either is not evidence of anything, so it is left
// out here rather than counted as a zero-day sprint.
const closedSprintsSQL = `
	SELECT start_date, end_date FROM sprint
	WHERE profile_id = ? AND board_id = ? AND state = 'closed'
	  AND start_date <> '' AND end_date <> ''`

// SprintLength is the median whole-day length of the board's last three
// closed sprints, which is what the start dialog offers as an end date.
//
// A board with no closed sprints, or none whose dates can be read, answers
// zero. Zero is not an error and not a length: it means this board has no
// history to suggest from, and what to do about that is the caller's, which
// is the sprints service offering a fortnight and telling the dialog there
// was nothing behind it.
//
// The rows are ordered here rather than in SQL because the dates are stored
// as Jira wrote them, and Jira's Agile datetime and the RFC 3339 a fixture
// carries do not sort against each other as text.
func (r *Repository) SprintLength(ctx context.Context, profileID string, boardID int) (int, error) {
	rows, err := r.db.QueryContext(ctx, closedSprintsSQL, profileID, boardID)
	if err != nil {
		return 0, fmt.Errorf("board %d sprint lengths: %w", boardID, err)
	}
	defer rows.Close()
	type ran struct {
		startedAt int64
		days      int
	}
	var past []ran
	for rows.Next() {
		var startText, endText string
		if err := rows.Scan(&startText, &endText); err != nil {
			return 0, err
		}
		start, serr := sprintdate.Parse(startText)
		end, eerr := sprintdate.Parse(endText)
		if serr != nil || eerr != nil {
			continue
		}
		// Rounded rather than truncated: a sprint that crosses a daylight
		// saving change runs 13 days and 23 hours, which is a fortnight
		// everywhere but in integer division.
		days := int(math.Round(end.Sub(start).Hours() / 24))
		if days <= 0 {
			continue
		}
		past = append(past, ran{startedAt: start.Unix(), days: days})
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(past) == 0 {
		return 0, nil
	}
	sort.Slice(past, func(i, j int) bool { return past[i].startedAt > past[j].startedAt })
	if len(past) > recentSprints {
		past = past[:recentSprints]
	}
	lengths := make([]int, 0, len(past))
	for _, p := range past {
		lengths = append(lengths, p.days)
	}
	sort.Ints(lengths)
	// The middle one, and for an even count the longer of the two middles: a
	// team that has just lengthened its sprint is better served by the date
	// it will move out to than by the one it has left behind.
	return lengths[len(lengths)/2], nil
}
