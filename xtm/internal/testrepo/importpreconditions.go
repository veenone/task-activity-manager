package testrepo

import (
	"fmt"
	"strings"
)

// PreconditionImportMapping maps precondition fields to spreadsheet column
// headers. An empty value means the field is unmapped. Summary is required.
type PreconditionImportMapping struct {
	Summary     string `json:"summary"`
	Type        string `json:"type"`
	Condition   string `json:"condition"`
	Description string `json:"description"`
	Labels      string `json:"labels"`
}

// PreconditionImportRow is one data row as the import sees it. Status is
// "new", "existing" (a precondition with the same summary is already cached,
// or an earlier row in the file creates it) or "error".
type PreconditionImportRow struct {
	Row     int    `json:"row"`
	Summary string `json:"summary"`
	Type    string `json:"type"`
	Status  string `json:"status"`
}

// PreconditionImportResult reports a precondition import or its dry run.
// Created stays 0 on a dry run, and on a real run that found any error.
type PreconditionImportResult struct {
	Created       int                     `json:"created"`
	NewCount      int                     `json:"newCount"`
	ExistingCount int                     `json:"existingCount"`
	Rows          []PreconditionImportRow `json:"rows"`
	Errors        []ImportError           `json:"errors"`
	// UnknownTypes lists Type values that are not Manual, Generic or Cucumber.
	UnknownTypes []string `json:"unknownTypes"`
}

// preconditionTypes is Xray's precondition type vocabulary.
var preconditionTypes = []string{"Manual", "Generic", "Cucumber"}

// PreconditionImportTemplateCSV returns a starter CSV with every column the
// precondition import reads.
func PreconditionImportTemplateCSV() string {
	return "Summary,Type,Condition,Description,Labels\n" +
		"Card reader is powered on,Manual,The reader LED is green,Check before every session,hardware\n" +
		"User is logged in,Cucumber,Given the user is authenticated,,smoke login\n"
}

// ImportPreconditions queues each new row of an uploaded precondition file as
// a pending create, committed from the Pending list like one made in the
// dialog. A row whose summary matches a cached precondition, after
// normalisation, is existing and skipped; so is a row repeating an earlier
// one, which keeps a re-import from creating anything twice.
//
// Every row is checked before anything is written. A blank summary or a Type
// outside Xray's three is an error, and any error stops a real run with
// nothing queued. A run that passes queues all its rows in one transaction, so
// a failure midway leaves nothing behind. dryRun only reports.
//
// fields are the create screen's required fields, applied to every row. A
// Labels column rides along in the same create fields, space-separated as
// labels are everywhere else.
func (r *Repository) ImportPreconditions(profileID, projectKey string, records [][]string, mapping PreconditionImportMapping, dryRun bool, fields map[string]any) (PreconditionImportResult, error) {
	res := PreconditionImportResult{Rows: []PreconditionImportRow{}, Errors: []ImportError{}, UnknownTypes: []string{}}
	if len(records) < 2 {
		return res, fmt.Errorf("the file has no data rows")
	}
	header := records[0]
	summaryCol := headerColumn(header, mapping.Summary)
	if summaryCol < 0 {
		return res, fmt.Errorf("the Summary field must be mapped to a column")
	}
	typeCol := headerColumn(header, mapping.Type)
	conditionCol := headerColumn(header, mapping.Condition)
	descriptionCol := headerColumn(header, mapping.Description)
	labelsCol := headerColumn(header, mapping.Labels)

	seen, err := r.preconditionSummaries(profileID)
	if err != nil {
		return res, err
	}
	type pending struct {
		summary, ptype, condition, description string
		labels                                 []string
	}
	var queue []pending
	unknown := map[string]bool{}
	for i, rec := range records[1:] {
		rowNum := i + 2
		cell := func(col int) string {
			if col < 0 || col >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[col])
		}
		p := pending{
			summary:     cell(summaryCol),
			condition:   cell(conditionCol),
			description: cell(descriptionCol),
			labels:      strings.Fields(cell(labelsCol)),
		}
		if strings.Join(rec, "") == "" {
			continue
		}
		row := PreconditionImportRow{Row: rowNum, Summary: p.summary}
		ptype, ok := preconditionType(cell(typeCol))
		row.Type = ptype
		switch {
		case p.summary == "":
			row.Status = "error"
			res.Errors = append(res.Errors, ImportError{Row: rowNum, Message: "Summary is empty."})
		case !ok:
			row.Status = "error"
			res.Errors = append(res.Errors, ImportError{Row: rowNum,
				Message: fmt.Sprintf("Type %q is not Manual, Generic or Cucumber.", ptype)})
			if !unknown[ptype] {
				unknown[ptype] = true
				res.UnknownTypes = append(res.UnknownTypes, ptype)
			}
		case seen[normalizeSummary(p.summary)]:
			row.Status = "existing"
			res.ExistingCount++
		default:
			row.Status = "new"
			res.NewCount++
			seen[normalizeSummary(p.summary)] = true
			p.ptype = ptype
			queue = append(queue, p)
		}
		res.Rows = append(res.Rows, row)
	}
	if dryRun || len(res.Errors) > 0 || len(queue) == 0 {
		return res, nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return res, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, p := range queue {
		rowFields := make(map[string]any, len(fields)+1)
		for k, v := range fields {
			rowFields[k] = v
		}
		if len(p.labels) > 0 {
			rowFields["labels"] = p.labels
		}
		key, err := createPreconditionTx(tx, profileID, projectKey, p.summary, p.ptype, p.description, rowFields)
		if err != nil {
			return res, err
		}
		if p.condition != "" {
			// Journaled as the dialog does it: an ordinary condition edit on the
			// temp key, pushed through Xray's resolved condition field on commit.
			if _, err := tx.Exec(`UPDATE precondition SET condition = ? WHERE profile_id = ? AND jira_key = ?`,
				p.condition, profileID, key); err != nil {
				return res, fmt.Errorf("set condition: %w", err)
			}
			if err := upsertPendingChange(tx, profileID, entityPreconditionEdit, key, "condition", "", p.condition, ""); err != nil {
				return res, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return res, fmt.Errorf("commit precondition import: %w", err)
	}
	res.Created = len(queue)
	return res, nil
}

// preconditionSummaries returns the normalised summaries of every cached
// precondition, the set an import dedupes against.
func (r *Repository) preconditionSummaries(profileID string) (map[string]bool, error) {
	rows, err := r.db.Query(`SELECT summary FROM precondition WHERE profile_id = ?`, profileID)
	if err != nil {
		return nil, fmt.Errorf("list precondition summaries: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out[normalizeSummary(s)] = true
	}
	return out, rows.Err()
}

// preconditionType returns Xray's spelling of a Type cell, Manual when it is
// blank, and false with the value as written when Xray has no such type.
func preconditionType(s string) (string, bool) {
	if s == "" {
		return "Manual", true
	}
	for _, t := range preconditionTypes {
		if strings.EqualFold(s, t) {
			return t, true
		}
	}
	return s, false
}
