// Package issueexport writes a backlog to a workbook: the rows the grid
// is showing, as a file somebody can plan a session around.
//
// It is plain excelize rather than the styled template internal/reportout
// embeds. A sprint report is a document people publish and read; this is a
// list people sort, filter and paste from, and a template would be
// styling nobody asked for over data they are about to rearrange.
package issueexport

import (
	"bytes"
	"fmt"
	"strconv"
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

// header is the column order. It opens with what the grid shows, in the
// grid's own order, so a reader finds the same columns in the same place,
// and then adds what the grid has no room for.
var header = []string{
	"Key", "Type", "Summary", "Status", "Assignee", "Sprint", "Points",
	"Estimated (h)", "Logged (h)", "Parent", "Labels", "Project", "Description",
}

// Workbook is the rows as a .xlsx file.
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
	for row, cells := range append([][]any{widen(header)}, rowsOf(issues)...) {
		cell, err := excelize.CoordinatesToCellName(1, row+1)
		if err != nil {
			return nil, err
		}
		if err := f.SetSheetRow(sheetName, cell, &cells); err != nil {
			return nil, fmt.Errorf("write row %d: %w", row+1, err)
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("write the workbook: %w", err)
	}
	return buf.Bytes(), nil
}

func rowsOf(issues []backend.Issue) [][]any {
	out := make([][]any, 0, len(issues))
	for _, iss := range issues {
		out = append(out, []any{
			iss.Key, iss.Type, iss.Summary, iss.Status, iss.Assignee, iss.SprintName,
			number(iss.StoryPoints), hours(iss.OriginalEstimateSeconds), hours(iss.TimeSpentSeconds),
			iss.ParentKey, strings.Join(iss.Labels, ", "), iss.Project, description(iss),
		})
	}
	return out
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

func hours(seconds *int) any {
	if seconds == nil {
		return ""
	}
	// Trimmed the way the grid trims a whole number, so eight hours is 8
	// and ninety minutes is 1.5.
	return trim(float64(*seconds) / hour)
}

func trim(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
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

// widen turns the header into what SetSheetRow takes, a slice of any.
func widen(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}
