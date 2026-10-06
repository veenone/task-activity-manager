package issueexport_test

import (
	"bytes"
	"strconv"
	"strings"
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

// styledRows are one row per colour the file has to carry: a bug, a done
// issue, and a type TAM does not model.
func styledRows() []backend.Issue {
	return []backend.Issue{
		{Key: "PLAT-1", Type: backend.TypeBug, Summary: "Order total wrong", Status: "In Progress", StatusCategory: "indeterminate", Labels: []string{}},
		{Key: "PLAT-2", Type: backend.TypeStory, Summary: "Guest checkout", Status: "Erledigt", StatusCategory: "done", Labels: []string{}},
		{Key: "PLAT-3", Type: "Improvement", Summary: "Trim the checkout step", Status: "Offen", StatusCategory: "new", Labels: []string{}},
		// A row synced before the category existed, which is a status
		// nothing here can bucket by Jira's own word for it.
		{Key: "PLAT-4", Type: backend.TypeTask, Summary: "Rotate keys", Status: "Wachten", Labels: []string{}},
	}
}

// fillOf is the background colour Excel will paint a cell, as a hex
// string, empty when the cell carries no fill of its own.
func fillOf(t *testing.T, f *excelize.File, cell string) string {
	t.Helper()
	id, err := f.GetCellStyle("Backlog", cell)
	if err != nil {
		t.Fatalf("style of %s: %v", cell, err)
	}
	style, err := f.GetStyle(id)
	if err != nil {
		t.Fatalf("read style %d: %v", id, err)
	}
	if len(style.Fill.Color) == 0 {
		return ""
	}
	return strings.ToUpper(strings.TrimPrefix(style.Fill.Color[0], "FF"))
}

// The header is the app's own chrome band, and the sheet opens ready to
// be sorted: the first thing anyone does with an exported backlog.
func TestTheHeaderIsStyledAndTheSheetOpensSortable(t *testing.T) {
	data, err := issueexport.Workbook(styledRows())
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	f := read(t, data)
	if got := fillOf(t, f, "A1"); got != "1B2638" {
		t.Errorf("header fill = %q, want the app's chrome band", got)
	}
	id, _ := f.GetCellStyle("Backlog", "A1")
	style, _ := f.GetStyle(id)
	if style.Font == nil || !style.Font.Bold {
		t.Error("the header is not bold")
	}
	panes, err := f.GetPanes("Backlog")
	if err != nil {
		t.Fatalf("panes: %v", err)
	}
	if panes.YSplit != 1 || !panes.Freeze {
		t.Errorf("panes = %+v, want the header row frozen", panes)
	}
}

// Type and Status are the two columns that carry meaning by colour on
// screen, so they carry it in the file.
func TestTypeAndStatusWearTheGridsColours(t *testing.T) {
	data, err := issueexport.Workbook(styledRows())
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	f := read(t, data)
	// A bug is the red chip, and its In Progress status is the blue one.
	if got := fillOf(t, f, "B2"); got != "FEE2E2" {
		t.Errorf("bug type fill = %q, want the red chip", got)
	}
	if got := fillOf(t, f, "D2"); got != "DBEAFE" {
		t.Errorf("in-progress status fill = %q, want the blue chip", got)
	}
	// A done issue is the green chip.
	if got := fillOf(t, f, "D3"); got != "DCFCE7" {
		t.Errorf("done status fill = %q, want the green chip", got)
	}
	// A type TAM does not model is coloured rather than grey: the bug #79
	// fixed on screen, which an unstyled export brought straight back.
	improvement := fillOf(t, f, "B4")
	if improvement == "" || improvement == "FAFBFC" {
		t.Errorf("Improvement type fill = %q, want one of the alternate chips", improvement)
	}
	if improvement == fillOf(t, f, "B2") {
		t.Errorf("Improvement wears the bug's colour (%s)", improvement)
	}
}

// The styling must not turn a number into text, or the column stops
// summing in the spreadsheet somebody exported it for.
func TestTheNumbersStaySummable(t *testing.T) {
	rows := []backend.Issue{{
		Key: "PLAT-1", Type: backend.TypeStory, Summary: "One", Status: "To Do", Labels: []string{},
		StoryPoints: pts(5), OriginalEstimateSeconds: secs(28800),
	}}
	data, err := issueexport.Workbook(rows)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	f := read(t, data)
	cols, err := f.GetRows("Backlog")
	if err != nil {
		t.Fatalf("rows: %v", err)
	}
	// Excel leaves a numeric cell untyped and marks a text one, so the
	// check is that the two differ and that the figure parses: a styled
	// column that merely right-aligned text would look like a number
	// column and sum to nothing.
	typeOf := func(head string) excelize.CellType {
		for i, name := range cols[0] {
			if name != head {
				continue
			}
			cell, _ := excelize.CoordinatesToCellName(i+1, 2)
			typ, err := f.GetCellType("Backlog", cell)
			if err != nil {
				t.Fatalf("cell type of %s: %v", head, err)
			}
			return typ
		}
		t.Fatalf("no %s column", head)
		return excelize.CellTypeUnset
	}
	valueOf := func(head string) string {
		for i, name := range cols[0] {
			if name == head {
				return cols[1][i]
			}
		}
		return ""
	}
	if typeOf("Points") == typeOf("Summary") {
		t.Errorf("the Points cell is typed like the Summary cell, so it is text")
	}
	for _, head := range []string{"Points", "Estimated (h)"} {
		if _, err := strconv.ParseFloat(valueOf(head), 64); err != nil {
			t.Errorf("%s = %q, which is not a number", head, valueOf(head))
		}
	}
}
