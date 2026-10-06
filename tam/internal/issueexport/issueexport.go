// Package issueexport writes a backlog to a workbook: the rows the grid
// is showing, as a file somebody can plan a session around.
//
// It is not the sprint report's embedded .xltx template. That template is
// a document people read, and restyling it is an edit to an Excel file; a
// backlog is the list people were just looking at, so this one is styled
// from the app's own palette and tracks it. palette.go holds the colours
// and the two rules that pick them, with a test against the stylesheet
// they are copied from.
package issueexport

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	"agile-suite/tam/internal/backend"
)

// sheetName is the one sheet the rows go on. It is a constant because
// Excel caps a sheet name at 31 characters and forbids several a project
// or filter name may hold.
const sheetName = "Backlog"

// hour is what the stored seconds are divided by. The time columns are in
// hours because that is the unit an estimate is discussed in; a column of
// 28800 is a column nobody can plan from.
const hour = 3600

// column is one column of the sheet: its heading, how wide it opens, and
// whether what it holds is a number, which Excel right-aligns and which
// has to stay a number so the column still sums.
type column struct {
	head    string
	width   float64
	numeric bool
}

// columns open with what the grid shows, in the grid's own order, so a
// reader finds the same columns in the same place, and then add what the
// grid has no room for. The widths are what each one holds: a key is
// short, a summary is a sentence, a description is a paragraph nobody
// wants spilling across the sheet.
var columns = []column{
	{head: "Key", width: 14},
	{head: "Type", width: 14},
	{head: "Summary", width: 52},
	{head: "Status", width: 16},
	{head: "Assignee", width: 18},
	{head: "Sprint", width: 20},
	{head: "Points", width: 8, numeric: true},
	{head: "Estimated (h)", width: 13, numeric: true},
	{head: "Logged (h)", width: 11, numeric: true},
	{head: "Parent", width: 14},
	{head: "Labels", width: 24},
	{head: "Project", width: 10},
	{head: "Description", width: 60},
}

// Workbook is the rows as a styled .xlsx file.
//
// A list that matched nothing still gets its header: refusing would leave
// the user guessing whether the export failed or the filter was empty.
func Workbook(issues []backend.Issue) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return nil, fmt.Errorf("make the sheet: %w", err)
	}
	f.SetActiveSheet(index)
	// NewFile opens with a default sheet this one replaces, so the
	// workbook has the one sheet its rows are on and no empty second.
	if def := "Sheet1"; def != sheetName {
		_ = f.DeleteSheet(def)
	}
	styles, err := newStyles(f)
	if err != nil {
		return nil, err
	}
	if err := writeHeader(f, styles); err != nil {
		return nil, err
	}
	for i, iss := range issues {
		if err := writeRow(f, styles, i+2, iss); err != nil {
			return nil, err
		}
	}
	if err := finish(f, len(issues)); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("write the workbook: %w", err)
	}
	return buf.Bytes(), nil
}

// styles are the cell styles the sheet uses, made once: Excel stores one
// record per style, and asking for a new one per cell would write a few
// thousand identical records into a backlog's worth of rows.
type styles struct {
	head   int
	text   int
	number int
	// chip holds one style per palette class, made on demand, since a
	// sheet only pays for the colours its rows actually wear.
	chip map[string]int
	file *excelize.File
}

func newStyles(f *excelize.File) (*styles, error) {
	head, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: chromeText, Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{chromeBG}},
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border:    bottomBorder(borderLine),
	})
	if err != nil {
		return nil, fmt.Errorf("header style: %w", err)
	}
	// The body carries the grid's own hairline under each row and nothing
	// else: a backlog is read across, and a box around every cell turns a
	// list into a grid of boxes.
	text, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border:    bottomBorder(rowLine),
	})
	if err != nil {
		return nil, fmt.Errorf("body style: %w", err)
	}
	number, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border:    bottomBorder(rowLine),
	})
	if err != nil {
		return nil, fmt.Errorf("number style: %w", err)
	}
	return &styles{head: head, text: text, number: number, chip: map[string]int{}, file: f}, nil
}

// chipStyle is the style for one palette class, the way a .chip-* rule
// paints it: the fill behind the chip's own text colour, centred, since a
// chip is a label rather than prose.
func (s *styles) chipStyle(class string) (int, error) {
	if id, ok := s.chip[class]; ok {
		return id, nil
	}
	c, ok := chips[class]
	if !ok {
		return s.text, nil
	}
	id, err := s.file.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: c.text, Bold: true, Size: 10},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{c.fill}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    bottomBorder(rowLine),
	})
	if err != nil {
		return 0, fmt.Errorf("chip style %s: %w", class, err)
	}
	s.chip[class] = id
	return id, nil
}

func bottomBorder(colour string) []excelize.Border {
	return []excelize.Border{{Type: "bottom", Color: colour, Style: 1}}
}

func writeHeader(f *excelize.File, s *styles) error {
	for i, c := range columns {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return err
		}
		if err := f.SetCellStr(sheetName, cell, c.head); err != nil {
			return fmt.Errorf("write the header: %w", err)
		}
		if err := f.SetCellStyle(sheetName, cell, cell, s.head); err != nil {
			return fmt.Errorf("style the header: %w", err)
		}
	}
	return nil
}

// writeRow writes one issue and dresses its two coloured columns. Type
// and Status are the columns that carry meaning by colour on screen, so
// they are the ones that carry it here; the rest is plain.
func writeRow(f *excelize.File, s *styles, row int, iss backend.Issue) error {
	values := []any{
		iss.Key, iss.Type, iss.Summary, iss.Status, iss.Assignee, iss.SprintName,
		number(iss.StoryPoints), hours(iss.OriginalEstimateSeconds), hours(iss.TimeSpentSeconds),
		iss.ParentKey, strings.Join(iss.Labels, ", "), iss.Project, description(iss),
	}
	for i, v := range values {
		cell, err := excelize.CoordinatesToCellName(i+1, row)
		if err != nil {
			return err
		}
		if err := f.SetCellValue(sheetName, cell, v); err != nil {
			return fmt.Errorf("write %s: %w", cell, err)
		}
		style, err := styleFor(s, i, iss)
		if err != nil {
			return err
		}
		if err := f.SetCellStyle(sheetName, cell, cell, style); err != nil {
			return fmt.Errorf("style %s: %w", cell, err)
		}
	}
	return nil
}

// styleFor picks a cell's style by its column: the chip colours for Type
// and Status, right alignment for a number, and plain text for the rest.
func styleFor(s *styles, col int, iss backend.Issue) (int, error) {
	switch columns[col].head {
	case "Type":
		return s.chipStyle(typeChipClass(iss.Type))
	case "Status":
		if strings.TrimSpace(iss.Status) == "" {
			return s.text, nil
		}
		return s.chipStyle(statusClass(iss.Status, iss.StatusCategory))
	}
	if columns[col].numeric {
		return s.number, nil
	}
	return s.text, nil
}

// finish sets the column widths, freezes the header and puts an
// autofilter over the table. Sorting and filtering is the first thing
// anyone does with an exported backlog, and a sheet that opens ready for
// it saves them finding the button.
func finish(f *excelize.File, rows int) error {
	for i, c := range columns {
		name, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return err
		}
		if err := f.SetColWidth(sheetName, name, name, c.width); err != nil {
			return fmt.Errorf("width of %s: %w", c.head, err)
		}
	}
	if err := f.SetPanes(sheetName, &excelize.Panes{
		Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft",
	}); err != nil {
		return fmt.Errorf("freeze the header: %w", err)
	}
	last, err := excelize.ColumnNumberToName(len(columns))
	if err != nil {
		return err
	}
	// Over the header alone when there are no rows: an autofilter still
	// belongs on a sheet somebody is about to paste into.
	end := rows + 1
	if end < 1 {
		end = 1
	}
	if err := f.AutoFilter(sheetName, fmt.Sprintf("A1:%s%d", last, end), nil); err != nil {
		return fmt.Errorf("add the filter: %w", err)
	}
	return nil
}

// number and hours leave a cell empty where the issue carries no value.
// A zero in a column somebody is about to sum is a figure the issue never
// had.
func number(v *float64) any {
	if v == nil {
		return ""
	}
	return *v
}

// hours is a float rather than the trimmed string it used to be: a styled
// column that right-aligns text would look like a number column and sum
// to nothing. Excel trims the trailing zeros itself, so eight hours still
// reads as 8.
func hours(seconds *int) any {
	if seconds == nil {
		return ""
	}
	// Two decimals is a minute and a half, which is finer than any
	// estimate is made to, and keeps 1.5 from arriving as 1.4999999.
	return float64(int(float64(*seconds)/hour*100+0.5)) / 100
}

// description is the text the sync cached on the row, empty when nothing
// has read one. The pointer's two meanings are the same cell here: a
// spreadsheet has no way to word "never read" that a reader would not
// take for "has none".
func description(iss backend.Issue) string {
	if iss.Description == nil {
		return ""
	}
	return *iss.Description
}
