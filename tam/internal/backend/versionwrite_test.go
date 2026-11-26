package backend

import "testing"

func TestFinishedReadsTheCategoryFirst(t *testing.T) {
	cases := []struct {
		iss  Issue
		want bool
	}{
		{Issue{Status: "Shipped", StatusCategory: "done"}, true},
		{Issue{Status: "Done", StatusCategory: "indeterminate"}, false},
		{Issue{Status: "Done"}, true},
		{Issue{Status: "In Progress"}, false},
	}
	for _, c := range cases {
		if got := Finished(c.iss); got != c.want {
			t.Errorf("Finished(%+v) = %v, want %v", c.iss, got, c.want)
		}
	}
}
