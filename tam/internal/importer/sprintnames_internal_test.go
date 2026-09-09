package importer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"agile-suite/tam/internal/boardrepo"
)

// The template's dropdown and the cell match have to agree about what a name
// means, or the picker offers values the import turns away on every row that
// uses them. These two live in the same file for that reason, and this is the
// test that holds them together, so it reaches sprintNames directly rather
// than reading a data validation back out of a workbook.
func TestTheDropdownOffersOnlyNamesTheImportWillAccept(t *testing.T) {
	open := []boardrepo.SprintChoice{
		// One sprint that two boards both draw. Still one sprint, still
		// resolvable, offered once.
		{ID: 12, Name: "Sprint 12", BoardName: "PLAT Scrum", State: "active"},
		{ID: 12, Name: "Sprint 12", BoardName: "PLAT Kanban", State: "active"},
		// Two different sprints that happen to share a name. Nothing can tell
		// them apart from a cell, so neither is offered.
		{ID: 40, Name: "Hardening", BoardName: "PLAT Scrum", State: "future"},
		{ID: 41, Name: "hardening", BoardName: "MOB Scrum", State: "future"},
		// Unambiguous, and last so the offer order is visible.
		{ID: 13, Name: "Sprint 13", BoardName: "PLAT Scrum", State: "future"},
	}

	got := sprintNames(open)
	if strings.Join(got, "|") != "Sprint 12|Sprint 13" {
		t.Fatalf("only the resolvable names are offered, in offer order: %v", got)
	}

	// The half that makes the omission worth making: the name left out is
	// one the cell match refuses, so offering it would have been a picker
	// entry that fails every row it lands in.
	if _, msg := newSprintIndex(open).lookup("Hardening"); !strings.Contains(msg, "more than one board") {
		t.Errorf("the omitted name is the one lookup refuses: %q", msg)
	}
	if _, msg := newSprintIndex(open).lookup("Sprint 12"); msg != "" {
		t.Errorf("the offered name still resolves: %q", msg)
	}
}

// A sprint named with a comma cannot go in an inline Excel list, which is one
// comma-joined string, so it is left out of the dropdown rather than arriving
// as two entries that name nothing.
func TestASprintNameWithACommaIsLeftOutOfTheDropdown(t *testing.T) {
	data, err := TemplateXLSX("Business Requirement", []boardrepo.SprintChoice{
		{ID: 12, Name: "Sprint 12, phase two", BoardName: "PLAT Scrum", State: "active"},
		{ID: 13, Name: "Sprint 13", BoardName: "PLAT Scrum", State: "future"},
	})
	if err != nil {
		t.Fatalf("TemplateXLSX: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer f.Close()
	col, err := excelize.ColumnNumberToName(len(TemplateHeaders))
	if err != nil {
		t.Fatalf("ColumnNumberToName: %v", err)
	}
	dvs, err := f.GetDataValidations(templateSheet)
	if err != nil {
		t.Fatalf("GetDataValidations: %v", err)
	}
	found := ""
	for _, dv := range dvs {
		if strings.HasPrefix(dv.Sqref, col+"2:") && dv.Formula1 != "" {
			found = dv.Formula1
		}
	}
	if found == "" {
		t.Fatal("the Sprint column has a list at all")
	}
	if strings.Contains(found, "phase two") {
		t.Errorf("the comma-bearing name is not in the list: %q", found)
	}
	if !strings.Contains(found, "Sprint 13") {
		t.Errorf("the usable name still is: %q", found)
	}
}
