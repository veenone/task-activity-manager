package cases

import (
	"reflect"
	"testing"
)

// The grid is the report's main table, so it must say what each case's own
// page says, project by project.
func TestTheGridAgreesWithEachCase(t *testing.T) {
	s, _ := open(t)
	a := caseFor(t, s, "A-1")
	b := caseFor(t, s, "B-1")
	_ = caseFor(t, s, "C-1") // a case with no projects still gets a row
	_ = s.AddProjects("p1", a, []string{"ALPHA", "BETA"})
	_ = s.AddProjects("p1", b, []string{"BETA"})
	_ = s.SetVerdict("p1", a, "BETA", Affected, "", at)
	_ = s.SetVerdict("p1", b, "BETA", NotAffected, "", at)
	_ = s.Open("p2", []string{"Z-1"}, at)

	grid, err := s.Grid("p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(grid) != 3 {
		t.Fatalf("%d rows, want 3", len(grid))
	}
	for _, row := range grid {
		own, err := s.Get("p1", row.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(row, own) {
			t.Errorf("grid row %+v\n case page %+v", row, own)
		}
	}
}
