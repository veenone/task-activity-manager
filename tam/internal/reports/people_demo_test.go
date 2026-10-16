package reports_test

import (
	"context"
	"testing"
	"time"

	demobackend "agile-suite/tam/internal/backend/demo"
	"agile-suite/tam/internal/reports"
)

// TestTheDemoSprintHasPeople is the offline People section: the demo's one
// closed sprint names who held its cards and counts somebody's time in
// progress, so the section can be seen without a Jira instance (#287).
func TestTheDemoSprintHasPeople(t *testing.T) {
	ctx := context.Background()
	b := demobackend.New("PLAT")
	sprints, err := b.BoardSprints(ctx, 1)
	if err != nil {
		t.Fatalf("demo sprints: %v", err)
	}
	var closed = sprints[0]
	for _, sp := range sprints {
		if sp.ID == 11 {
			closed = sp
		}
	}
	issues, _, err := b.SearchIssuesWithHistory(ctx, "sprint = 11", 0, 50)
	if err != nil {
		t.Fatalf("demo history: %v", err)
	}
	statuses, err := b.Statuses(ctx)
	if err != nil {
		t.Fatalf("demo statuses: %v", err)
	}
	byID := map[string]string{}
	for _, s := range statuses {
		byID[s.ID] = s.Category
	}
	byName := map[string]string{}
	for _, s := range statuses {
		byName[s.Name] = s.Category
	}
	category := func(id, name string) string {
		if c, ok := byID[id]; ok {
			return c
		}
		return byName[name]
	}

	people, err := reports.People(closed, issues, category, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("people: %v", err)
	}
	if len(people) < 2 {
		t.Fatalf("people = %+v, want the sprint's assignees", people)
	}
	var cards, inProgress int
	for _, p := range people {
		cards += p.Cards
		inProgress += p.InProgressSeconds
	}
	if cards != len(issues) {
		t.Errorf("rows hold %d cards, want every one of the sprint's %d", cards, len(issues))
	}
	if inProgress == 0 {
		t.Error("PLAT-385 sat in progress until the 15th, so somebody's in-progress time is above zero")
	}
}
