package reports_test

import (
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/reports"
)

func open(key, status, category, display, user string, pts *float64, est, rem, spent int) backend.Issue {
	return backend.Issue{
		Key: key, Status: status, StatusCategory: category, Assignee: display, AssigneeName: user,
		StoryPoints: pts, OriginalEstimateSeconds: secs(est), RemainingEstimateSeconds: secs(rem), TimeSpentSeconds: secs(spent),
	}
}

// TestAllocationIsTheBoardsOpenWorkPerAssignee is #288: every unresolved
// card on the board, one row per assignee, sorted by name with Unassigned
// last, and each row's cards behind it.
func TestAllocationIsTheBoardsOpenWorkPerAssignee(t *testing.T) {
	rows := reports.Allocation([]backend.Issue{
		open("PLAT-1", "In Progress", "indeterminate", "Rita Anand", "ranand", points(5), 8, 3, 6),
		open("PLAT-2", "To Do", "new", "Rita Anand", "ranand", points(3), 4, 4, 0),
		open("PLAT-3", "Done", "done", "Rita Anand", "ranand", points(8), 10, 0, 10),
		// No category cached yet: the status name decides, as IsDone does.
		open("PLAT-4", "Closed", "", "maria Ortiz", "mortiz", points(2), 2, 0, 2),
		open("PLAT-5", "To Do", "", "maria Ortiz", "mortiz", nil, 0, 0, 0),
		open("PLAT-6", "To Do", "new", "", "", points(1), 1, 1, 0),
	})
	if len(rows) != 3 || rows[0].Name != "maria Ortiz" || rows[1].Name != "Rita Anand" || rows[2].Name != "" {
		t.Fatalf("rows = %+v, want maria Ortiz, Rita Anand, Unassigned", rows)
	}
	rita := rows[1]
	if rita.Cards != 2 || rita.Points != 8 || rita.EstimateSeconds != 12*3600 || rita.RemainingSeconds != 7*3600 || rita.SpentSeconds != 6*3600 {
		t.Errorf("Rita = %+v, want her two open cards: 8 points, 12h estimated, 7h left, 6h spent", rita)
	}
	if len(rita.Issues) != 2 || rita.Issues[0].Key != "PLAT-1" || rita.Issues[1].Key != "PLAT-2" {
		t.Errorf("Rita's cards = %+v, want PLAT-1 and PLAT-2 in board order", rita.Issues)
	}
	if rows[0].Cards != 1 {
		t.Errorf("Maria = %+v, want only her card that is not Closed", rows[0])
	}
}

func TestAllocationOfABoardWithNothingOpenIsEmpty(t *testing.T) {
	if rows := reports.Allocation([]backend.Issue{open("PLAT-3", "Done", "done", "Rita Anand", "ranand", nil, 0, 0, 0)}); len(rows) != 0 {
		t.Errorf("rows = %+v, want none", rows)
	}
}
