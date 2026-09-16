package ritualsync

import (
	"testing"

	"agile-suite/tam/internal/boardrepo"
)

func TestSprintsLeavesOutADraftSprint(t *testing.T) {
	got := Sprints([]boardrepo.Sprint{
		{ID: -1, BoardID: 1, Name: "Sprint 15", State: "future", Draft: true},
		{ID: 13, BoardID: 1, Name: "Sprint 13", State: "future"},
	}, "PLAT Scrum", nil)
	if len(got) != 1 || got[0].Info.ID != 13 {
		t.Errorf("sprints = %+v, want only the one Jira holds", got)
	}
}
