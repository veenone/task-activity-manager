// Package workbook writes what a PIE view shows to an Excel workbook. The
// frontend decides the rows, since it holds the filters; this package only
// lays them out.
package workbook

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/xuri/excelize/v2"
)

// Sheet is one worksheet: a header row and the rows under it, as text. A
// whole number is stored as a number, so Excel can sum and sort it.
type Sheet struct {
	Name   string     `json:"name"`
	Header []string   `json:"header"`
	Rows   [][]string `json:"rows"`
}

// maxName is Excel's limit on a sheet name.
const maxName = 31

// Workbook returns the sheets as an .xlsx file. Each header row is bold,
// frozen and carries an autofilter.
func Workbook(sheets []Sheet) ([]byte, error) {
	if len(sheets) == 0 {
		return nil, errors.New("there is nothing to export")
	}
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	bold, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	for i, s := range sheets {
		name := sheetName(s.Name, i)
		if i == 0 {
			if err := f.SetSheetName("Sheet1", name); err != nil {
				return nil, err
			}
		} else if _, err := f.NewSheet(name); err != nil {
			return nil, err
		}
		if err := write(f, name, s, bold); err != nil {
			return nil, fmt.Errorf("sheet %s: %w", name, err)
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func write(f *excelize.File, name string, s Sheet, bold int) error {
	header := make([]any, len(s.Header))
	for i, h := range s.Header {
		header[i] = h
	}
	if err := f.SetSheetRow(name, "A1", &header); err != nil {
		return err
	}
	for r, row := range s.Rows {
		cells := make([]any, len(row))
		for c, v := range row {
			cells[c] = cell(v)
		}
		if err := f.SetSheetRow(name, fmt.Sprintf("A%d", r+2), &cells); err != nil {
			return err
		}
	}
	if len(s.Header) == 0 {
		return nil
	}
	last, err := excelize.ColumnNumberToName(len(s.Header))
	if err != nil {
		return err
	}
	if err := f.SetCellStyle(name, "A1", last+"1", bold); err != nil {
		return err
	}
	if err := f.SetPanes(name, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return err
	}
	if err := f.AutoFilter(name, fmt.Sprintf("A1:%s%d", last, len(s.Rows)+1), nil); err != nil {
		return err
	}
	return f.SetColWidth(name, "A", last, 18)
}

// cell stores a whole number as a number. "0042" stays text, since its
// leading zero is part of what it says.
func cell(v string) any {
	n, err := strconv.Atoi(v)
	if err != nil || (len(v) > 1 && v[0] == '0') || v[0] == '+' {
		return v
	}
	return n
}

// sheetName fits a name to Excel's 31 characters, or names an unnamed sheet
// by its place.
func sheetName(name string, i int) string {
	r := []rune(name)
	if len(r) == 0 {
		return fmt.Sprintf("Sheet %d", i+1)
	}
	if len(r) > maxName {
		r = r[:maxName]
	}
	return string(r)
}
