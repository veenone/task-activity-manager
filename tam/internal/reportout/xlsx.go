package reportout

import (
	"bytes"
	"fmt"

	"github.com/xuri/excelize/v2"
)

// sheetName is the one sheet a report is written to. It is a constant and
// not the sprint's name because Excel caps a sheet name at 31 characters
// and forbids several a sprint name may hold, and a spreadsheet that fails
// to save over a sprint called "Q3 / hardening" is worse than one whose tab
// always reads the same.
const sheetName = "Sprint report"

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
	f := excelize.NewFile()
	defer f.Close()
	idx, err := f.NewSheet(sheetName)
	if err != nil {
		return nil, err
	}
	f.SetActiveSheet(idx)
	if err := f.DeleteSheet("Sheet1"); err != nil {
		return nil, err
	}
	bold, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	// Wide enough that a caveat and a sprint name are read without widening
	// a column by hand, which is the first thing a reader would otherwise do.
	if err := f.SetColWidth(sheetName, "A", "A", 40); err != nil {
		return nil, err
	}

	w := &sheet{f: f, bold: bold, row: 1}
	w.line(d.Title, true)
	for _, s := range d.Sections {
		w.blank()
		w.line(s.Heading, true)
		for _, line := range s.Lines {
			w.line(line, false)
		}
		if len(s.Table.Columns) > 0 {
			w.cells(s.Table.Columns, true)
			for _, row := range s.Table.Rows {
				w.cells(row, false)
			}
		}
		for _, note := range s.Notes {
			w.line(note, false)
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
	f    *excelize.File
	bold int
	row  int
	err  error
}

func (w *sheet) blank() { w.row++ }

func (w *sheet) line(text string, heading bool) {
	if text == "" {
		return
	}
	w.cells([]string{text}, heading)
}

func (w *sheet) cells(values []string, heading bool) {
	for i, v := range values {
		col, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			w.keep(err)
			return
		}
		at := fmt.Sprintf("%s%d", col, w.row)
		w.keep(w.f.SetCellStr(sheetName, at, v))
		if heading {
			w.keep(w.f.SetCellStyle(sheetName, at, at, w.bold))
		}
	}
	w.row++
}

func (w *sheet) keep(err error) {
	if w.err == nil {
		w.err = err
	}
}
