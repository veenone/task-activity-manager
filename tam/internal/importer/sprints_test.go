package importer_test

import (
	"context"
	"strings"
	"testing"

	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/importer"
)

// openSprints is what the profile's boards offer. Sprint 11 is closed, so it
// is not in here at all: the read only ever hands over the open ones, which
// is why naming a closed sprint reads like naming one that does not exist.
func openSprints() []boardrepo.SprintChoice {
	return []boardrepo.SprintChoice{
		{ID: 12, Name: "Sprint 12", BoardName: "PLAT Scrum", State: "active"},
		{ID: 13, Name: "Sprint 13", BoardName: "PLAT Scrum", State: "future"},
	}
}

func sprintRecords() [][]string {
	return [][]string{
		{"Type", "Summary", "Sprint"},
		{"Task", "Into the active sprint", "sprint 12"},
		{"Task", "Into a sprint that closed", "Sprint 11"},
		{"Task", "Into the backlog", ""},
		{"Task", "Into a sprint nobody has", "Sprint 99"},
	}
}

func TestTheSprintColumnMatchesAnOpenSprintByNameAndTurnsAwayTheRest(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	m := importer.AutoMap(sprintRecords()[0])
	if m.Sprint != "Sprint" {
		t.Fatalf("the Sprint column is mapped: %+v", m)
	}

	res, err := importer.Run(ctx, repo, "p1", "PLAT", "", openSprints(), sprintRecords(), m, "plan.csv", false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Join(res.Created, ",") != "TAM-NEW-1,TAM-NEW-2" || len(res.Errors) != 2 {
		t.Fatalf("two rows landed and two were turned away: %+v", res)
	}

	// The cell is matched without regard for case, and the draft carries the
	// id the move will be pushed with beside the name a reader sees.
	first, err := repo.GetIssue(ctx, "p1", "TAM-NEW-1")
	if err != nil || first.SprintID != "12" || first.SprintName != "Sprint 12" {
		t.Errorf("the first draft is in the active sprint: %+v %v", first, err)
	}
	// An empty cell is the backlog, which is a destination and not an error.
	second, err := repo.GetIssue(ctx, "p1", "TAM-NEW-2")
	if err != nil || second.Summary != "Into the backlog" || second.SprintID != "" || second.SprintName != "" {
		t.Errorf("the second draft is in the backlog: %+v %v", second, err)
	}

	byRow := map[int]string{}
	for _, e := range res.Errors {
		byRow[e.Row] = e.Message
	}
	for _, tc := range []struct {
		row   int
		named string
	}{{3, `"Sprint 11"`}, {5, `"Sprint 99"`}} {
		msg := byRow[tc.row]
		if !strings.Contains(msg, tc.named) {
			t.Errorf("row %d names the sprint it was given: %q", tc.row, msg)
		}
		if !strings.Contains(msg, "Sprint 12 (PLAT Scrum)") || !strings.Contains(msg, "Sprint 13 (PLAT Scrum)") {
			t.Errorf("row %d lists what was available: %q", tc.row, msg)
		}
	}
}

func TestASprintNameOpenOnTwoBoardsIsRefusedRatherThanGuessed(t *testing.T) {
	repo := newRepo(t)
	open := []boardrepo.SprintChoice{
		{ID: 12, Name: "Sprint 12", BoardName: "PLAT Scrum", State: "active"},
		{ID: 12, Name: "Sprint 12", BoardName: "PLAT Kanban", State: "active"},
		{ID: 44, Name: "Sprint 12", BoardName: "MOB Scrum", State: "active"},
	}
	recs := [][]string{
		{"Type", "Summary", "Sprint"},
		{"Task", "Which team?", "Sprint 12"},
	}
	res, err := importer.Run(context.Background(), repo, "p1", "PLAT", "", open, recs, importer.AutoMap(recs[0]), "plan.csv", true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// The same sprint handed to two boards is still one sprint; a second
	// sprint with the same name on a third board is what cannot be resolved.
	if len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Message, "more than one board") {
		t.Fatalf("the ambiguous name is refused: %+v", res.Errors)
	}
	if !strings.Contains(res.Errors[0].Message, "PLAT Scrum") || !strings.Contains(res.Errors[0].Message, "MOB Scrum") {
		t.Errorf("the message names both boards: %q", res.Errors[0].Message)
	}
}

func TestASprintCellOnAProfileWithNoBoardsSaysToRefreshThem(t *testing.T) {
	repo := newRepo(t)
	recs := [][]string{
		{"Type", "Summary", "Sprint"},
		{"Task", "Into a sprint TAM has never seen", "Sprint 12"},
	}
	res, err := importer.Run(context.Background(), repo, "p1", "PLAT", "", nil, recs, importer.AutoMap(recs[0]), "plan.csv", true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Message, "no synced boards") {
		t.Fatalf("the row says why there was nothing to match: %+v", res.Errors)
	}
}

func TestTheTemplateCarriesTheSprintColumnAndItsOpenSprints(t *testing.T) {
	if importer.TemplateHeaders[len(importer.TemplateHeaders)-1] != "Sprint" {
		t.Fatalf("Sprint is the last template column: %v", importer.TemplateHeaders)
	}
	// The workbook builds with a dropdown on the Sprint column and without
	// one; neither is allowed to fail the save.
	for _, open := range [][]boardrepo.SprintChoice{openSprints(), nil} {
		data, err := importer.TemplateXLSX("Business Requirement", open)
		if err != nil || len(data) == 0 {
			t.Fatalf("TemplateXLSX with %d sprints: %v", len(open), err)
		}
	}
	if !strings.Contains(string(importer.TemplateCSV("Business Requirement")), "Sprint") {
		t.Error("the CSV template carries the column too")
	}
}
