package issueexport_test

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issueexport"
)

func pts(v float64) *float64 { return &v }
func secs(v int) *int        { return &v }

func rows() []backend.Issue {
	return []backend.Issue{
		{
			Key: "PLAT-412", Project: "PLAT", Type: backend.TypeStory, Summary: "Apply promo code",
			Status: "In Progress", Assignee: "R. Anand", SprintName: "Sprint 12", ParentKey: "PLAT-350",
			Labels: []string{"checkout", "promo"}, StoryPoints: pts(5),
			OriginalEstimateSeconds: secs(28800), TimeSpentSeconds: secs(7200),
		},
		{Key: "PLAT-409", Project: "PLAT", Type: backend.TypeTask, Summary: "Rotate keys", Status: "To Do", Labels: []string{}},
	}
}

// read opens what the writer produced, which is the only way to know it
// wrote a workbook rather than a file with the right extension.
func read(t *testing.T, data []byte) *excelize.File {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open the workbook: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func TestTheWorkbookCarriesAHeaderAndARowPerIssue(t *testing.T) {
	data, err := issueexport.Workbook(rows())
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	f := read(t, data)
	sheet := f.GetSheetName(0)
	cells, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("read rows: %v", err)
	}
	if len(cells) != 3 {
		t.Fatalf("rows = %d, want a header and the two issues", len(cells))
	}
	if cells[0][0] != "Key" || cells[0][1] != "Type" {
		t.Errorf("header = %v", cells[0])
	}
	if cells[1][0] != "PLAT-412" {
		t.Errorf("first row = %v", cells[1])
	}
	// The labels are one cell, not one column each: a spreadsheet of
	// issues has a labels column, and a row with three labels must not
	// push the columns beside it out of line.
	joined := false
	for _, c := range cells[1] {
		if c == "checkout, promo" {
			joined = true
		}
	}
	if !joined {
		t.Errorf("row = %v, want the labels in one cell", cells[1])
	}
}

// The time columns read as the hours a person thinks in, not as seconds.
// A spreadsheet of 28800s is a spreadsheet nobody can plan from.
func TestTheTimeColumnsAreWrittenAsHours(t *testing.T) {
	data, err := issueexport.Workbook(rows())
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	f := read(t, data)
	cells, _ := f.GetRows(f.GetSheetName(0))
	header, row, blank := cells[0], cells[1], cells[2]
	for i, name := range header {
		switch name {
		case "Estimated (h)":
			if row[i] != "8" {
				t.Errorf("estimate = %q, want 8", row[i])
			}
			// An issue with no estimate leaves the cell empty rather
			// than writing a zero somebody would sum.
			if i < len(blank) && blank[i] != "" {
				t.Errorf("unestimated row estimate = %q, want nothing", blank[i])
			}
		case "Logged (h)":
			if row[i] != "2" {
				t.Errorf("logged = %q, want 2", row[i])
			}
		}
	}
}

// A filter that matched nothing still writes a workbook, with the header
// alone. Refusing would make the user guess whether the export failed or
// the filter was simply empty.
func TestAnEmptyExportIsAWorkbookWithItsHeader(t *testing.T) {
	data, err := issueexport.Workbook(nil)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	cells, _ := read(t, data).GetRows("Backlog")
	if len(cells) != 1 || cells[0][0] != "Key" {
		t.Errorf("rows = %v, want the header alone", cells)
	}
}
