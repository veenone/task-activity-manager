package reports

import (
	"fmt"
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

// window is the sprint's start, its end (the close, or the planned end
// for a sprint still running), and the stretch of time a sprint's figures are counted over: from
// its start to its close, or to now for a sprint still running (even past
// its planned end), capped at now for a close stamped in the future. A
// sprint that has not started yet is its single first instant.
func window(sprint backend.Sprint, now time.Time) (start, end, last time.Time, err error) {
	if start, err = sprintdate.Parse(sprint.StartDate); err != nil {
		return start, end, last, fmt.Errorf("sprint %d has no start date TAM can read, so there is nothing to reconstruct: %w: %w", sprint.ID, ErrNoDates, err)
	}
	if end, err = sprintEnd(sprint); err != nil {
		return start, end, last, fmt.Errorf("sprint %d has no end date TAM can read, so there is nothing to reconstruct: %w: %w", sprint.ID, ErrNoDates, err)
	}
	if end.Before(start) {
		return start, end, last, fmt.Errorf("sprint %d ends before it starts, so it has no days to walk: %w", sprint.ID, ErrNoDates)
	}
	last = end
	if strings.EqualFold(strings.TrimSpace(sprint.State), "active") || now.Before(last) {
		last = now
	}
	if last.Before(start) {
		last = start
	}
	return start, end, last, nil
}
