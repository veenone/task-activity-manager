package sprintreport

import (
	"context"
	"fmt"
	"log"

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
	s.attachSubtaskWorklogs(ctx, out)
	return out, nil
}

// attachSubtaskWorklogs reads what the sub-tasks of these issues logged,
// for the cards the report will scope by their family.
//
// Only the issues that need it are asked about: one whose family spent
// matches its own has no child carrying hours, and one whose children
// are in the sprint beside it is counted as itself anyway. A backend
// that cannot answer, or a Jira that refuses the query, leaves the burn
// counting what is in the sprint, which is what it did before this
// existed: a sprint report is worth more than the hours it is short.
func (s *Service) attachSubtaskWorklogs(ctx context.Context, issues []backend.IssueHistory) {
	b, ok := s.b.(backend.FamilyWorklogBackend)
	if !ok {
		return
	}
	inSprint := make(map[string]bool, len(issues))
	for _, h := range issues {
		inSprint[h.Issue.Key] = true
	}
	hasChild := map[string]bool{}
	for _, h := range issues {
		if h.Issue.ParentKey != "" && inSprint[h.Issue.ParentKey] {
			hasChild[h.Issue.ParentKey] = true
		}
	}
	var parents []string
	for _, h := range issues {
		if hasChild[h.Issue.Key] {
			continue
		}
		if t := h.Issue.Time(); t.Family {
			parents = append(parents, h.Issue.Key)
		}
	}
	if len(parents) == 0 {
		return
	}
	byParent, err := b.SubtaskWorklogs(ctx, parents)
	if err != nil {
		log.Printf("tam: the sub-task worklogs of sprint %s could not be read, so its hours burndown counts only what is in the sprint: %v", issues[0].Issue.SprintName, err)
		return
	}
	for i := range issues {
		if logs, ok := byParent[issues[i].Issue.Key]; ok {
			issues[i].SubtaskWorklogs = logs
		}
	}
}
