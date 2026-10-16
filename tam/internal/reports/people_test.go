package reports_test

import (
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/reports"
)

// categories is the instance's status buckets as /rest/api/2/status would
// report them: To Do new, In Progress and In Review indeterminate, Done done.
func categories(id, name string) string {
	switch {
	case id == "1" || name == "To Do":
		return "new"
	case id == "3" || id == "4" || name == "In Progress" || name == "In Review":
		return "indeterminate"
	case id == "10001" || name == "Done":
		return "done"
	}
	return ""
}

func secs(h int) *int { v := h * 3600; return &v }

func assigned(h backend.IssueHistory, display, user, email string) backend.IssueHistory {
	h.Issue.Assignee, h.Issue.AssigneeName, h.Issue.AssigneeEmail = display, user, email
	return h
}

func people(t *testing.T, issues ...backend.IssueHistory) []reports.Person {
	t.Helper()
	got, err := reports.People(sprint11(), issues, categories, afterTheSprint)
	if err != nil {
		t.Fatalf("people: %v", err)
	}
	return got
}

func row(t *testing.T, rows []reports.Person, username string) reports.Person {
	t.Helper()
	for _, r := range rows {
		if r.Username == username {
			return r
		}
	}
	t.Fatalf("no row for %q in %+v", username, rows)
	return reports.Person{}
}

func TestPeopleAreSortedByNameWithUnassignedLast(t *testing.T) {
	rows := people(t,
		assigned(card("PLAT-1", "To Do", "1", "11", points(3)), "Rita Anand", "ranand", "r.anand@example.com"),
		card("PLAT-2", "To Do", "1", "11", points(1)),
		assigned(card("PLAT-3", "To Do", "1", "11", points(5)), "maria Ortiz", "mortiz", ""),
		assigned(card("PLAT-4", "To Do", "1", "11", points(2)), "Rita Anand", "ranand", "r.anand@example.com"),
	)
	var order []string
	for _, r := range rows {
		order = append(order, r.Name)
	}
	if len(order) != 3 || order[0] != "maria Ortiz" || order[1] != "Rita Anand" || order[2] != "" {
		t.Fatalf("order = %q, want maria Ortiz, Rita Anand, then the unassigned row (sorted ignoring case)", order)
	}
	rita := row(t, rows, "ranand")
	if rita.Cards != 2 || rita.Points != 5 || rita.Email != "r.anand@example.com" {
		t.Errorf("Rita = %+v, want 2 cards, 5 points and her email", rita)
	}
	if unassigned := rows[2]; unassigned.Cards != 1 || unassigned.Points != 1 {
		t.Errorf("unassigned = %+v, want 1 card of 1 point", unassigned)
	}
}

func TestPeopleAddUpEachCardsOwnEstimates(t *testing.T) {
	c1 := assigned(card("PLAT-1", "To Do", "1", "11", nil), "Rita Anand", "ranand", "")
	c1.Issue.OriginalEstimateSeconds, c1.Issue.RemainingEstimateSeconds = secs(8), secs(3)
	c2 := assigned(card("PLAT-2", "To Do", "1", "11", nil), "Rita Anand", "ranand", "")
	c2.Issue.OriginalEstimateSeconds = secs(4)
	// The family aggregates are the parent and its children together; a
	// child in the sprint is a row of its own, so adding the aggregate too
	// would count its hours twice.
	c2.Issue.AggregateEstimateSeconds = secs(40)
	rita := row(t, people(t, c1, c2), "ranand")
	if rita.EstimateSeconds != 12*3600 || rita.RemainingSeconds != 3*3600 {
		t.Errorf("Rita = %+v, want 12h estimated and 3h remaining", rita)
	}
}

func TestLoggedIsWorkTheAuthorLoggedInsideTheSprint(t *testing.T) {
	c := assigned(card("PLAT-1", "To Do", "1", "11", nil), "Rita Anand", "ranand", "")
	c.Worklogs = []backend.Worklog{
		{Author: "ranand", AuthorName: "Rita Anand", Started: on("2026-08-04", 10), Seconds: 2 * 3600},
		// Logged on Rita's card by somebody else, who gets a row of their
		// own even with nothing assigned: the time is theirs.
		{Author: "skim", AuthorName: "S. Kim", AuthorEmail: "s.kim@example.com", Started: on("2026-08-05", 10), Seconds: 3600},
		// Before the sprint started, and after it ended: not this sprint's.
		{Author: "ranand", AuthorName: "Rita Anand", Started: on("2026-07-30", 10), Seconds: 5 * 3600},
		{Author: "ranand", AuthorName: "Rita Anand", Started: on("2026-08-20", 10), Seconds: 7 * 3600},
	}
	c.SubtaskWorklogs = []backend.Worklog{
		{Author: "ranand", AuthorName: "Rita Anand", Started: on("2026-08-06", 10), Seconds: 3600},
	}
	rows := people(t, c)
	if rita := row(t, rows, "ranand"); rita.LoggedSeconds != 3*3600 || rita.Cards != 1 {
		t.Errorf("Rita = %+v, want 3h logged in the sprint (2h on the card, 1h on a sub-task) and her one card", rita)
	}
	if kim := row(t, rows, "skim"); kim.LoggedSeconds != 3600 || kim.Cards != 0 || kim.Name != "S. Kim" || kim.Email != "s.kim@example.com" {
		t.Errorf("Kim = %+v, want 1h logged, no cards, and the name and email the worklog carried", kim)
	}
}

func TestInProgressIsTimeInTheInProgressCategoryClippedToTheSprint(t *testing.T) {
	// A day in progress and a day in review, both "indeterminate".
	worked := assigned(card("PLAT-1", "Done", "10001", "11", nil,
		backend.Change{At: on("2026-08-04", 9), Field: "status", From: "To Do", To: "In Progress", FromID: "1", ToID: "3"},
		backend.Change{At: on("2026-08-05", 9), Field: "status", From: "In Progress", To: "In Review", FromID: "3", ToID: "4"},
		backend.Change{At: on("2026-08-06", 9), Field: "status", From: "In Review", To: "Done", FromID: "4", ToID: "10001"},
	), "Rita Anand", "ranand", "")
	// Started before the sprint and never finished: counted from the
	// sprint's start to its end, eleven days, and nothing outside them.
	carried := assigned(card("PLAT-2", "In Progress", "3", "11", nil,
		backend.Change{At: on("2026-07-28", 9), Field: "status", From: "To Do", To: "In Progress", FromID: "1", ToID: "3"},
	), "Maria Ortiz", "mortiz", "")
	// Never moved: its current status held the whole sprint.
	idle := assigned(card("PLAT-3", "To Do", "1", "11", nil), "Maria Ortiz", "mortiz", "")
	// A change with no ids, from an older changelog, is matched by name.
	named := assigned(card("PLAT-4", "Done", "10001", "11", nil,
		backend.Change{At: on("2026-08-10", 9), Field: "status", From: "To Do", To: "In Progress"},
		backend.Change{At: on("2026-08-10", 15), Field: "status", From: "In Progress", To: "Done"},
	), "S. Kim", "skim", "")

	rows := people(t, worked, carried, idle, named)
	if got := row(t, rows, "ranand").InProgressSeconds; got != 48*3600 {
		t.Errorf("Rita in progress = %dh, want 48h (a day in progress, a day in review)", got/3600)
	}
	if got := row(t, rows, "mortiz").InProgressSeconds; got != 11*24*3600 {
		t.Errorf("Maria in progress = %dh, want the sprint's 264h and not the days before it", got/3600)
	}
	if got := row(t, rows, "skim").InProgressSeconds; got != 6*3600 {
		t.Errorf("Kim in progress = %dh, want 6h matched by status name", got/3600)
	}
}

// TestARowWithNoUsernameJoinsTheSameNamesRow covers a card whose assignee
// came back with a display name and no username (an older cached row, or a
// GDPR-mode user): it is the same person as the row with that name, not a
// second one.
func TestARowWithNoUsernameJoinsTheSameNamesRow(t *testing.T) {
	rows := people(t,
		assigned(card("PLAT-1", "To Do", "1", "11", points(3)), "M. Ortiz", "mortiz", ""),
		assigned(card("PLAT-2", "To Do", "1", "11", points(2)), "M. Ortiz", "", ""),
	)
	if len(rows) != 1 || rows[0].Username != "mortiz" || rows[0].Cards != 2 || rows[0].Points != 5 {
		t.Fatalf("rows = %+v, want one M. Ortiz row holding both cards", rows)
	}
}

// TestEachPersonListsTheirCards gives the People tab its breakdown: every
// card a person holds, with its own figures, and the time logged on it so
// far (actual) beside the time logged inside the sprint.
func TestEachPersonListsTheirCards(t *testing.T) {
	c := assigned(card("PLAT-7", "In Progress", "3", "11", points(5),
		backend.Change{At: on("2026-08-04", 9), Field: "status", From: "To Do", To: "In Progress", FromID: "1", ToID: "3"},
	), "Rita Anand", "ranand", "")
	c.Issue.Summary, c.Issue.StatusCategory = "Apply promo code", "indeterminate"
	c.Issue.OriginalEstimateSeconds, c.Issue.RemainingEstimateSeconds, c.Issue.TimeSpentSeconds = secs(10), secs(4), secs(7)
	c.Worklogs = []backend.Worklog{
		{Author: "ranand", Started: on("2026-08-05", 10), Seconds: 2 * 3600},
		{Author: "skim", Started: on("2026-08-06", 10), Seconds: 3600},
		{Author: "ranand", Started: on("2026-07-20", 10), Seconds: 4 * 3600},
	}
	rita := row(t, people(t, c), "ranand")
	if rita.SpentSeconds != 7*3600 {
		t.Errorf("Rita's actual = %dh, want the 7h logged on her card so far", rita.SpentSeconds/3600)
	}
	if len(rita.Issues) != 1 {
		t.Fatalf("Rita's cards = %+v, want PLAT-7", rita.Issues)
	}
	got := rita.Issues[0]
	want := reports.PersonCard{
		Key: "PLAT-7", Summary: "Apply promo code", Status: "In Progress", StatusCategory: "indeterminate",
		Points: points(5), EstimateSeconds: 10 * 3600, RemainingSeconds: 4 * 3600, SpentSeconds: 7 * 3600,
		LoggedSeconds: 3 * 3600, InProgressSeconds: 10 * 24 * 3600,
	}
	if got.Key != want.Key || got.Summary != want.Summary || got.Status != want.Status || got.StatusCategory != want.StatusCategory ||
		got.Points == nil || *got.Points != 5 || got.EstimateSeconds != want.EstimateSeconds || got.RemainingSeconds != want.RemainingSeconds ||
		got.SpentSeconds != want.SpentSeconds || got.LoggedSeconds != want.LoggedSeconds || got.InProgressSeconds != want.InProgressSeconds {
		t.Errorf("card = %+v, want %+v (logged counts every author inside the sprint)", got, want)
	}
}
