package reports

import (
	"strings"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/sprintdate"
)

// sprintEnd uses the actual close for a closed sprint. Older cached sprints
// may have no completion date; only that absence permits the planned fallback.
// A present but invalid completion date must surface as an unreadable date.
func sprintEnd(sprint backend.Sprint) (time.Time, error) {
	if Closed(sprint) && strings.TrimSpace(sprint.CompleteDate) != "" {
		return sprintdate.Parse(sprint.CompleteDate)
	}
	return sprintdate.Parse(sprint.EndDate)
}
