package donerule_test

import (
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/donerule"
)

func columns() []backend.BoardColumn {
	return []backend.BoardColumn{
		{Name: "To Do", StatusIDs: []string{"1"}},
		{Name: "In Progress", StatusIDs: []string{"3"}},
		{Name: "Released", StatusIDs: []string{"10001", "10002"}},
	}
}

func TestOnlyTheLastColumnsStatusesCountAsDone(t *testing.T) {
	done := donerule.Done(columns())
	if done == nil {
		t.Fatal("a board with a last column that collects statuses can answer")
	}
	for _, id := range []string{"10001", "10002"} {
		if !done(id) {
			t.Errorf("status %s is in the last column and should be done", id)
		}
	}
	for _, id := range []string{"1", "3", "", "10003"} {
		if done(id) {
			t.Errorf("status %s is not in the last column and should not be done", id)
		}
	}
}

func TestABoardWithNoColumnsCannotAnswer(t *testing.T) {
	if donerule.Done(nil) != nil {
		t.Error("a board with no cached columns has no rule to hand out")
	}
	if _, ok := donerule.LastColumn(nil); ok {
		t.Error("a board with no columns has no last column")
	}
}

func TestALastColumnThatCollectsNothingCannotAnswer(t *testing.T) {
	cols := append(columns()[:2:2], backend.BoardColumn{Name: "Released"})
	if donerule.Done(cols) != nil {
		t.Error("a last column collecting no statuses cannot say what finished")
	}
	last, ok := donerule.LastColumn(cols)
	if !ok || last.Name != "Released" {
		t.Errorf("the refusal needs the column's name to word itself; got %q, %v", last.Name, ok)
	}
}

func TestTheRuleIsAStatusIDAndNotAStatusName(t *testing.T) {
	done := donerule.Done(columns())
	if done("Released") {
		t.Error("the board's rule reads status ids; a name matching the column's is not one")
	}
}
