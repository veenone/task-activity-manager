package importer_test

import (
	"context"
	"testing"

	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/importer"
)

// A draft sprint is an open sprint like any other to the Sprint column: the
// imported draft carries its negative id, and Commit creates the sprint
// before the draft is moved into it.
func TestTheSprintColumnMatchesADraftSprint(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	open := append(openSprints(), boardrepo.SprintChoice{ID: -1, Name: "Sprint 15", BoardName: "PLAT Scrum", State: "future", Draft: true})
	records := [][]string{{"Type", "Summary", "Sprint"}, {"Task", "Into the draft sprint", "sprint 15"}}
	res, err := importer.Run(ctx, repo, "p1", "PLAT", "", open, records, importer.AutoMap(records[0]), "plan.csv", false)
	if err != nil || len(res.Created) != 1 || len(res.Errors) != 0 {
		t.Fatalf("Run = %+v, %v", res, err)
	}
	iss, err := repo.GetIssue(ctx, "p1", res.Created[0])
	if err != nil || iss.SprintID != "-1" || iss.SprintName != "Sprint 15" {
		t.Errorf("the imported draft is in the draft sprint: %+v, %v", iss, err)
	}
}
