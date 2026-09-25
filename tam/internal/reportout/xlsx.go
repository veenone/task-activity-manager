package reportout

import (
	"bytes"
	_ "embed"
	"fmt"

	"github.com/xuri/excelize/v2"
)

// sheetTemplate is the look of the export: fonts, colours, borders and
// column widths, authored in Excel rather than in Go literals. Restyling
// the spreadsheet is an edit to this file.
//
// Rows 1 to 6 of column A are its style key, one styled cell per style this
// renderer uses, in the order styleKey lists them. The renderer reads each
// anchor's style index, deletes the six rows and then writes the report.
// Going through the file means a style can be changed in Excel and seen
// there, which a table of hex codes in Go could not offer.
//
//go:embed sheet.xltx
var sheetTemplate []byte

// sheetName is the one sheet a report is written to, and the name the
// template's own sheet carries. It is a constant and not the sprint's name
// because Excel caps a sheet name at 31 characters and forbids several a
// sprint name may hold, and a spreadsheet that fails to save over a sprint
// called "Q3 / hardening" is worse than one whose tab always reads the same.
const sheetName = "Sprint report"

// The style key's anchors, in the rows the template puts them in.
const (
	styleTitle = iota + 1
	styleHeading
	styleLine
	styleTableHead
	styleTableCell
	styleNote
	styleKeyRows = styleNote
)

// XLSX is a report as a spreadsheet: one sheet, read top to bottom, with
// each section's heading, its sentences, its table and the caveats on it in
// the order they are read on screen.
//
// One sheet rather than one per section, because the caveats are what a
// sheet of bare figures would lose: a tab of numbers whose qualification is
// on another tab is a tab that looks authoritative and is not. The rows sit
// in real cells, so the table can still be sorted, filtered or pivoted.
func XLSX(d Document) ([]byte, error) {
	if err := d.Check(); err != nil {
		return nil, err
	}
	f, err := excelize.OpenReader(bytes.NewReader(sheetTemplate))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// Naming the file is what turns the template back into a workbook:
	// excelize takes the main part's content type from this extension when
	// it writes, and a workbook still typed as a template opens in Excel as
	// a new unsaved copy rather than as the export the user asked for.
	f.Path = "report.xlsx"

	w := &sheet{f: f, row: 1}
	if w.readStyleKey(); w.err != nil {
		return nil, w.err
	}

	w.line(d.Title, styleTitle)
	for _, s := range d.Sections {
		w.blank()
		w.line(s.Heading, styleHeading)
		for _, line := range s.Lines {
			w.line(line, styleLine)
		}
		if len(s.Table.Columns) > 0 {
			w.cells(s.Table.Columns, styleTableHead)
			for _, row := range s.Table.Rows {
				w.cells(row, styleTableCell)
			}
		}
		for _, note := range s.Notes {
			w.line(note, styleNote)
		}
	}
	if w.err != nil {
		return nil, w.err
	}
	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// sheet writes rows down one sheet and keeps the first error, so the writer
// above reads as the document does rather than as a wall of error checks.
type sheet struct {
	f      *excelize.File
	styles map[int]int
	row    int
	err    error
}

// readStyleKey takes the template's styles from its anchor cells and then
// removes them, so the export starts on an empty sheet that still carries
// every style the template defined.
func (w *sheet) readStyleKey() {
	w.styles = map[int]int{}
	for anchor := 1; anchor <= styleKeyRows; anchor++ {
		id, err := w.f.GetCellStyle(sheetName, fmt.Sprintf("A%d", anchor))
		if err != nil {
			w.keep(err)
			return
		}
		w.styles[anchor] = id
	}
	for range styleKeyRows {
		w.keep(w.f.RemoveRow(sheetName, 1))
	}
}

func (w *sheet) blank() { w.row++ }

func (w *sheet) line(text string, style int) {
	if text == "" {
		return
	}
	w.cells([]string{text}, style)
}

func (w *sheet) cells(values []string, style int) {
	for i, v := range values {
		col, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			w.keep(err)
			return
		}
		at := fmt.Sprintf("%s%d", col, w.row)
		w.keep(w.f.SetCellStr(sheetName, at, v))
		w.keep(w.f.SetCellStyle(sheetName, at, at, w.styles[style]))
	}
	w.row++
}

func (w *sheet) keep(err error) {
	if w.err == nil {
		w.err = err
	}
}
