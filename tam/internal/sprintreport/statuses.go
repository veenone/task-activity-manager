package sprintreport

import (
	"context"
	"log"
	"strings"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/reports"
)

// statusCategory is the lookup the People section replays a card's status
// changes against.
//
// It starts from the categories the fetched cards carry for their current
// statuses, which every backend answers with, and lays the instance's full
// status list over them when the backend can give one. The list is what
// names a status a card passed through and has since left; without it such
// a status is unknown and its time is not counted as in progress. A list
// that cannot be read leaves the cards' own categories, since a report is
// worth more than the hours it is short.
func (s *Service) statusCategory(ctx context.Context, issues []backend.IssueHistory) reports.StatusCategory {
	byID, byName := map[string]string{}, map[string]string{}
	add := func(id, name, category string) {
		if category == "" {
			return
		}
		if id != "" {
			byID[id] = category
		}
		if name != "" {
			byName[strings.ToLower(name)] = category
		}
	}
	for _, h := range issues {
		add(h.Issue.StatusID, h.Issue.Status, h.Issue.StatusCategory)
	}
	if sb, ok := s.b.(backend.StatusBackend); ok {
		statuses, err := sb.Statuses(ctx)
		if err != nil {
			log.Printf("tam: the instance's statuses could not be read, so the People section counts in-progress time only for statuses a sprint card holds now: %v", err)
		}
		for _, st := range statuses {
			add(st.ID, st.Name, st.Category)
		}
	}
	return func(id, name string) string {
		if c, ok := byID[id]; ok && id != "" {
			return c
		}
		return byName[strings.ToLower(name)]
	}
}
