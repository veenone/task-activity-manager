package sprintreport

import (
	"context"
	"fmt"

	"agile-suite/tam/internal/backend"
)

// fetch reads one sprint's issues with their changelogs, a page at a time
// until the search is exhausted.
//
// The query is "sprint = N", which answers with whoever is in the sprint
// now. What that cannot see is section 3 of this phase's design and is
// carried through the series itself: a card dragged out on day four and left
// out is never returned here, so nothing downstream can know it existed.
// This is the one place the query is written, and widening it is the change
// that would fix that.
//
// Paging advances by what actually came back rather than by the page size
// asked for, so an instance that clamps maxResults lower than PageSize is
// handled by the same arithmetic; a page that comes back empty ends the loop
// rather than repeating the same request forever.
func (s *Service) fetch(ctx context.Context, sprint backend.Sprint, phase string) ([]backend.IssueHistory, error) {
	jql := fmt.Sprintf("sprint = %d", sprint.ID)
	frame := Progress{Phase: phase, SprintID: sprint.ID, SprintName: sprint.Name}

	out := []backend.IssueHistory{}
	startAt, total := 0, -1
	for total < 0 || startAt < total {
		page, n, err := s.b.SearchIssuesWithHistory(ctx, jql, startAt, s.PageSize)
		if err != nil {
			return nil, fmt.Errorf("read sprint %d's history: %w", sprint.ID, err)
		}
		total = n
		out = append(out, page...)
		startAt += len(page)
		frame.Fetched, frame.Total = len(out), total
		s.emit(frame)
		if len(page) == 0 {
			break
		}
	}
	frame.Done = true
	s.emit(frame)
	return out, nil
}
