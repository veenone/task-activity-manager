package testrepo_test

import (
	"reflect"
	"testing"

	"agile-suite/xtm/internal/testrepo"
)

// The member table filters an execution's tests by label offline, so each
// board row carries its test's labels from the local cache (#446).
func TestBoardRowsCarryTheirTestsLabels(t *testing.T) {
	repo := seedPlanBoard(t)
	if err := repo.UpsertTests("p1", []testrepo.TestCase{
		{Key: "QA-1", ID: "1", Summary: "Login", Status: "Approved", Labels: []string{"smoke", "auth"}},
	}); err != nil {
		t.Fatal(err)
	}
	board, err := repo.GetContainerBoard("p1", "QA-TP-1")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, r := range board.Rows {
		got[r.TestKey] = r.Labels
	}
	if !reflect.DeepEqual(got["QA-1"], []string{"smoke", "auth"}) || got["QA-2"] == nil || len(got["QA-2"]) != 0 {
		t.Fatalf("labels = %#v", got)
	}
}
