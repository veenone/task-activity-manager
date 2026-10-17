package reportout

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

// percentFormat is Excel's built-in "0%" number format.
const percentFormat = 9

// barColor is the data bar's fill, the app's info blue.
const barColor = "#3884DE"

// table writes a section's figures. A table with a bar column gets one more
// column: each row's ratio as a number shown as a percentage, under an Excel
// data bar scaled from 0% to 100%. The number is what a reader can sort and
// sum; the bar is the visual indicator the People tab draws on screen.
func (w *sheet) table(t Table) {
	if len(t.Bars) == 0 {
		w.cells(t.Columns, styleTableHead)
		for _, row := range t.Rows {
			w.cells(row, styleTableCell)
		}
		return
	}
	w.cells(append(append([]string{}, t.Columns...), t.BarHeading), styleTableHead)
	col, err := excelize.ColumnNumberToName(len(t.Columns) + 1)
	if err != nil {
		w.keep(err)
		return
	}
	first := w.row
	pct := w.percentStyle()
	for i, row := range t.Rows {
		at := fmt.Sprintf("%s%d", col, w.row)
		w.keep(w.f.SetCellStyle(sheetName, at, at, pct))
		if r, ok := t.Bars[i].ratio(); ok {
			w.keep(w.f.SetCellFloat(sheetName, at, r, -1, 64))
		}
		w.cells(row, styleTableCell)
	}
	ref := fmt.Sprintf("%s%d:%s%d", col, first, col, w.row-1)
	w.keep(w.f.SetConditionalFormat(sheetName, ref, []excelize.ConditionalFormatOptions{{
		Type:     "data_bar",
		Criteria: "=",
		MinType:  "num",
		MinValue: "0",
		MaxType:  "num",
		MaxValue: "1",
		BarColor: barColor,
		BarSolid: true,
	}}))
}

// percentStyle is the table cell's style from the template with the number
// shown as a percentage, made once per workbook.
func (w *sheet) percentStyle() int {
	if id, ok := w.styles[stylePercent]; ok {
		return id
	}
	base, err := w.f.GetStyle(w.styles[styleTableCell])
	if err != nil {
		w.keep(err)
		return w.styles[styleTableCell]
	}
	base.NumFmt = percentFormat
	// excelize reads some template fills back with a pattern it then
	// refuses to write; the table cells carry no fill worth keeping.
	if base.Fill.Pattern < 0 || base.Fill.Pattern > 18 {
		base.Fill = excelize.Fill{}
	}
	// Borders read back as ARGB ("00C9D6DF"), which NewStyle takes for a
	// different colour; the template's grey is the last six digits.
	for i, b := range base.Border {
		if len(b.Color) == 8 {
			base.Border[i].Color = b.Color[2:]
		}
	}
	id, err := w.f.NewStyle(base)
	if err != nil {
		w.keep(err)
		return w.styles[styleTableCell]
	}
	w.styles[stylePercent] = id
	return id
}
