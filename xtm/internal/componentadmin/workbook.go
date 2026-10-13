package componentadmin

import (
	"errors"
	"strings"

	"agile-suite/core/importfile"
	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/testrepo"
)

// templateHeader is the column row of the component import template, and the
// names ParseWorkbook looks for (in any order and any case).
var templateHeader = []string{"Name", "Description", "Lead username", "Default assignee"}

// assigneeLabels maps the default assignee wording the form shows to Jira's
// value. ParseWorkbook also takes Jira's own values.
var assigneeLabels = map[string]string{
	"project default": "PROJECT_DEFAULT",
	"component lead":  "COMPONENT_LEAD",
	"project lead":    "PROJECT_LEAD",
	"unassigned":      "UNASSIGNED",
}

// TemplateWorkbook is the XLSX a user downloads, fills in and imports. It is
// generated so its columns cannot drift from what ParseWorkbook reads.
func TemplateWorkbook() ([]byte, error) {
	return testrepo.WriteXLSXSheets([]testrepo.NamedRows{{
		Name:   "Components",
		Header: templateHeader,
		Rows: [][]string{
			{"API", "Public REST API", "jdoe", "Component lead"},
			{"Frontend", "Web client", "", "Project default"},
		},
	}})
}

// ParseWorkbook reads component rows from the first sheet of an XLSX file.
// Fully empty rows are skipped; a row with a description but no name is kept,
// so Check can say so instead of the row silently disappearing.
func ParseWorkbook(data []byte) ([]backend.ComponentInput, error) {
	records, err := importfile.ParseRecords(data, true)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, errors.New("The workbook is empty.")
	}
	col := map[string]int{}
	for i, h := range records[0] {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	if _, ok := col["name"]; !ok {
		return nil, errors.New("The workbook has no Name column. Start from the template.")
	}
	cell := func(rec []string, name string) string {
		i, ok := col[name]
		if !ok || i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}
	rows := []backend.ComponentInput{}
	for _, rec := range records[1:] {
		in := backend.ComponentInput{
			Name:         cell(rec, "name"),
			Description:  cell(rec, "description"),
			LeadUserName: cell(rec, "lead username"),
			AssigneeType: assigneeValue(cell(rec, "default assignee")),
		}
		if in == (backend.ComponentInput{}) {
			continue
		}
		rows = append(rows, in)
	}
	return rows, nil
}

// assigneeValue turns the form's wording or Jira's value into Jira's value.
// Anything else is returned as written, for Check to reject.
func assigneeValue(s string) string {
	if v, ok := assigneeLabels[strings.ToLower(s)]; ok {
		return v
	}
	if validAssignee(strings.ToUpper(s)) {
		return strings.ToUpper(s)
	}
	return s
}
