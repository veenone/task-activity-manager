// Package reportout renders a sprint report somewhere other than the screen:
// a Confluence page, a spreadsheet and a deck.
//
// It words nothing. Every sentence and every table cell in a Document was
// built by the frontend, in lib/reportText and lib/reportTables, and the
// reason is in lib/reportDocument's own comment: the report already has one
// vocabulary, in TypeScript, and a Go renderer that built its own would be
// a second one for the same figures. So this package lays a document out
// and never decides what it says. Nothing here reads a clock, a database or
// Jira, and nothing here may ever call Jira at all.
//
// Notes are the caveats, and a renderer that has to drop something never
// drops those: the figures are rebuilt from a changelog walk and can
// disagree with Jira's own report, so numbers without their qualification
// look authoritative and are not.
package reportout

import "errors"

// Table is a section's figures. Columns empty means the section has no
// table, which is how a burndown with no days or a board with no closed
// sprint travels: it says so in Lines instead.
type Table struct {
	Columns []string   `json:"columns"`
	Rows    [][]string `json:"rows"`
}

// Section is one part of the report: a heading, the sentences that open it,
// its figures, and the caveats on those figures.
type Section struct {
	Heading string   `json:"heading"`
	Lines   []string `json:"lines"`
	Table   Table    `json:"table"`
	Notes   []string `json:"notes"`
}

// Document is a whole sprint report. Title is the sprint's own, and it is
// the Confluence page's title, the spreadsheet's first row and the deck's
// title slide.
type Document struct {
	Title    string    `json:"title"`
	Sections []Section `json:"sections"`
}

// Check refuses a document there is nothing to render (I1: what arrives
// over the binding is checked, not assumed). The frontend already refuses
// to build one for an unavailable report, and this is the second line of
// defence: an empty page, a spreadsheet of headings and a deck of zeroes
// all look like a report that says nothing rather than like no report.
func (d Document) Check() error {
	if d.Title == "" {
		return errors.New("this report has no title, so there is nothing to publish or export")
	}
	if len(d.Sections) == 0 {
		return errors.New("this report has no sections, so there is nothing to publish or export")
	}
	return nil
}
