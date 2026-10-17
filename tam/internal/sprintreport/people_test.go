package sprintreport_test

import (
	"context"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/sprintdate"
)

// statusHistory is a changelog backend that can also list the instance's
// statuses, as the Jira and demo backends can.
type statusHistory struct {
	*fakeHistory
	statuses []backend.Status
	asked    int
}

func (s *statusHistory) Statuses(context.Context) ([]backend.Status, error) {
	s.asked++
	return s.statuses, nil
}

var _ backend.StatusBackend = (*statusHistory)(nil)

// inProgressFromDayTwo is a card that went into In Progress (id "3") on the
// sprint's second day and stayed there: ten days of in-progress time.
func inProgressFromDayTwo() backend.IssueHistory {
	c := card("PLAT-101", "In Progress", "3", 1, points(3))
	c.Issue.Assignee, c.Issue.AssigneeName, c.Issue.AssigneeEmail = "Rita Anand", "ranand", "r.anand@example.com"
	c.Changes = []backend.Change{{
		At:    sprintdate.Format(firstMonday.AddDate(0, 0, 1)),
		Field: "status", From: "To Do", To: "In Progress", FromID: "1", ToID: "3",
	}}
	return c
}

func TestTheReportCarriesItsPeopleAndStoresThemWithTheSeries(t *testing.T) {
	history := &statusHistory{fakeHistory: newHistory(), statuses: []backend.Status{
		{ID: "1", Name: "To Do", Category: "new"},
		{ID: "3", Name: "In Progress", Category: "indeterminate"},
		{ID: "10001", Name: "Done", Category: "done"},
	}}
	history.issues[1] = []backend.IssueHistory{inProgressFromDayTwo()}
	store := newStore(columns(), closedSprint(1))

	got, err := service(history, store).Build(context.Background(), testProfile, testBoard, 1, false)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	people := got.Series.People
	if len(people) != 1 || people[0].Username != "ranand" || people[0].Email != "r.anand@example.com" || people[0].Cards != 1 {
		t.Fatalf("people = %+v, want Rita's one card", people)
	}
	if want := 10 * 24 * 3600; people[0].InProgressSeconds != want {
		t.Errorf("in progress = %dh, want %dh", people[0].InProgressSeconds/3600, want/3600)
	}
	if history.asked != 1 {
		t.Errorf("statuses read %d times, want once for the one series built", history.asked)
	}
	saved, ok, _ := store.Report(context.Background(), testProfile, testBoard, 1)
	if !ok || len(saved.Series.People) != 1 {
		t.Errorf("the stored series has to keep its people, so a closed sprint's report still shows them; got %+v", saved.Series.People)
	}
}

func TestABackendWithNoStatusListFallsBackToTheCardsOwnCategories(t *testing.T) {
	history := newHistory()
	c := inProgressFromDayTwo()
	c.Issue.StatusCategory = "indeterminate"
	history.issues[1] = []backend.IssueHistory{c}
	store := newStore(columns(), closedSprint(1))

	got, err := service(history, store).Build(context.Background(), testProfile, testBoard, 1, false)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(got.Series.People) != 1 || got.Series.People[0].InProgressSeconds != 10*24*3600 {
		t.Errorf("people = %+v, want the card's ten days in progress, read off its current status's category", got.Series.People)
	}
}
