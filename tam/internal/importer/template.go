package importer

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	"agile-suite/tam/internal/boardrepo"
)

// TemplateHeaders are the columns of the generated workbook, in the order it
// writes them and the order the dialog maps them. They are spelled the way
// AutoMap's synonyms expect, so a file saved from this template maps itself
// with no clicking.
var TemplateHeaders = []string{"Key", "Type", "Summary", "Description", "Priority", "Labels", "Assignee", "Story Points", "Parent", "Sprint"}

// templateWidths are the column widths in characters. Summary, Description,
// and Acceptance-criteria-sized prose get the room they need, so the file
// opens readable rather than as a wall of ##### and clipped text.
var templateWidths = []float64{26, 14, 46, 60, 12, 22, 16, 12, 22, 20}

const (
	templateSheet = "Issues"
	notesSheet    = "How to use"
)

// TemplateXLSX builds the starter workbook: an Issues sheet with the header,
// a few example rows, and a Type dropdown, plus a sheet explaining what each
// column does and what happens to a row that carries a Key.
//
// requirementType is the profile's own name for its requirement level, so
// the dropdown offers the word that project actually uses rather than a
// generic "Requirement" the import would then reject.
//
// open is the profile's open sprints, which become the Sprint column's
// dropdown, so the one column whose valid values are per profile can be
// picked instead of remembered. A profile with no synced boards gets the
// column without a list rather than an empty one nobody can satisfy.
//
// The example rows leave Key, Parent, and Sprint empty on purpose. All
// three are checked against what is already cached for the profile, so any
// value written in here would be wrong for everyone but the machine it was
// written on, and would greet a first-time user with validation errors.
func TemplateXLSX(requirementType string, open []boardrepo.SprintChoice) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	idx, err := f.NewSheet(templateSheet)
	if err != nil {
		return nil, err
	}
	f.SetActiveSheet(idx)
	if err := f.DeleteSheet("Sheet1"); err != nil {
		return nil, err
	}

	head, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"2F5597"}},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	if err != nil {
		return nil, err
	}
	body, err := f.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{Vertical: "top", WrapText: true}})
	if err != nil {
		return nil, err
	}

	for i, h := range TemplateHeaders {
		col, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return nil, err
		}
		if err := f.SetCellStr(templateSheet, col+"1", h); err != nil {
			return nil, err
		}
		if err := f.SetColWidth(templateSheet, col, col, templateWidths[i]); err != nil {
			return nil, err
		}
	}
	last, err := excelize.ColumnNumberToName(len(TemplateHeaders))
	if err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(templateSheet, "A1", last+"1", head); err != nil {
		return nil, err
	}
	if err := f.SetRowHeight(templateSheet, 1, 22); err != nil {
		return nil, err
	}

	rows := templateRows(requirementType)
	for r, row := range rows {
		for i, v := range row {
			col, err := excelize.ColumnNumberToName(i + 1)
			if err != nil {
				return nil, err
			}
			if err := f.SetCellStr(templateSheet, fmt.Sprintf("%s%d", col, r+2), v); err != nil {
				return nil, err
			}
		}
	}
	if err := f.SetCellStyle(templateSheet, "A2", fmt.Sprintf("%s%d", last, len(rows)+1), body); err != nil {
		return nil, err
	}

	// The header stays visible while the user scrolls a long backlog.
	if err := f.SetPanes(templateSheet, &excelize.Panes{
		Freeze: true, Split: false, XSplit: 0, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft",
	}); err != nil {
		return nil, err
	}

	// A dropdown on Type over a generous range, so rows pasted in below the
	// examples are validated too. Priority gets no list: its values are per
	// instance, and a wrong list would block valid input.
	dv := excelize.NewDataValidation(true)
	dv.Sqref = "B2:B1000"
	if err := dv.SetDropList(templateTypes(requirementType)); err != nil {
		return nil, err
	}
	dv.SetError(excelize.DataValidationErrorStyleStop, "Type", "Pick one of the listed types, or clear the cell for Task.")
	if err := f.AddDataValidation(templateSheet, dv); err != nil {
		return nil, err
	}

	if err := addSprintList(f, sprintNames(open)); err != nil {
		return nil, err
	}

	if err := writeNotes(f, requirementType); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// sprintListLimit is how long an inline dropdown may be. Excel stores an
// inline list as one formula string and refuses it past 255 characters,
// which a project with a dozen open sprints reaches; past that the column
// keeps its notes-sheet explanation and loses only the picker.
const sprintListLimit = 255

// addSprintList puts the open sprints on the Sprint column, over the same
// generous range the Type list uses so pasted rows are validated too. A
// profile with no synced sprints gets no list at all: an empty dropdown
// would refuse every value including the ones the import accepts.
//
// A name carrying a comma is left out. Excel stores an inline list as one
// comma-joined string, so "Sprint 12, phase two" would arrive in the picker
// as two entries, neither of which names a sprint.
func addSprintList(f *excelize.File, names []string) error {
	usable := make([]string, 0, len(names))
	for _, n := range names {
		if !strings.Contains(n, ",") {
			usable = append(usable, n)
		}
	}
	names = usable
	if len(names) == 0 || len(strings.Join(names, ",")) > sprintListLimit {
		return nil
	}
	col, err := excelize.ColumnNumberToName(len(TemplateHeaders))
	if err != nil {
		return err
	}
	dv := excelize.NewDataValidation(true)
	dv.Sqref = fmt.Sprintf("%s2:%s1000", col, col)
	if err := dv.SetDropList(names); err != nil {
		return err
	}
	dv.SetError(excelize.DataValidationErrorStyleStop, "Sprint",
		"Pick one of this profile's open sprints, or clear the cell for the backlog.")
	return f.AddDataValidation(templateSheet, dv)
}

func templateTypes(requirementType string) []string {
	return []string{"Task", "Story", "Bug", "Epic", requirementLabel(requirementType)}
}

// templateRows are the examples: four creates that show a filled row of each
// shape, and one update row whose Key cell is empty but whose comment column
// explains what to paste there. They are ordinary rows, so a user who wants
// to start clean deletes them and keeps the header.
func templateRows(requirementType string) [][]string {
	req := requirementLabel(requirementType)
	return [][]string{
		{"", "Epic", "Promotions and discounts", "Everything about promo codes", "High", "promo", "", "", "", ""},
		{"", "Story", "Apply promo code at payment step", "As a shopper I can enter a promo code and see the discount before paying.", "High", "checkout, promo", "jdoe", "5", "", ""},
		{"", "Task", "Rotate the payment gateway keys", "Rotate before the audit window closes.", "Medium", "security", "jdoe", "2", "", ""},
		{"", "Bug", "Promo code field accepts whitespace", "Trim the input before validating.", "Low", "promo", "", "1", "", ""},
		{"", req, "Promo codes are single-use per customer", "Enforced at redemption.", "High", "promo", "", "", "", ""},
	}
}

// notes is the second sheet: one row per column, plus the rules that are not
// obvious from a column name. It is written as cells rather than one blob of
// text so the user can widen, sort, or copy out of it.
var notes = [][]string{
	{"Column", "What it does"},
	{"Key", "Leave empty to create a new issue. Fill it in with an existing issue key (RND-123) or its browse URL (https://jira/browse/RND-123) to update that issue instead of creating a second one."},
	{"Type", "Task, Story, Bug, Epic, or the project's requirement type. Empty means Task. Ignored on a row that has a Key: an existing issue keeps the type it has."},
	{"Summary", "Required on a row that creates. On a row with a Key it is optional, and changes the issue's summary when filled in."},
	{"Description", "Free text. Line breaks inside the cell are kept."},
	{"Priority", "The priority name as your Jira spells it, for example Highest, High, Medium, Low."},
	{"Labels", "Comma-separated. Jira labels cannot contain spaces."},
	{"Assignee", "The Jira username, not the display name. It is what appears in the user's profile URL, for example jdoe, not John Doe."},
	{"Story Points", "A number. Leave empty for no estimate."},
	{"Parent", "The epic this issue belongs to, by key. An epic row must leave it empty. An epic named here has to exist in the project already, or be created by an earlier row of this same file."},
	{"Sprint", "The sprint name, exactly as your Jira spells it, from any board this profile has synced. Empty means the backlog. Ignored on a row that has a Key: a sprint is a board write, not a field, so move an existing issue between sprints from the Backlog, the Epics tree, or a board instead. The dropdown on this column can come up empty on a profile with many open sprints (past 255 combined characters) or leave out a sprint whose own name has a comma; the column still accepts a typed name in either case."},
	{"", ""},
	{"Rules", ""},
	{"Empty cells", "On a row with a Key, an empty cell means leave that field alone. It never clears a value; clear a field in the app instead."},
	{"Nothing is pushed yet", "Every row lands as a pending change. Review them in Pending changes, then Commit to send them to Jira."},
	{"Re-importing", "Safe. A create that already exists as a draft is skipped, and an update whose values already match records nothing."},
	{"Bad rows", "A row that fails validation is listed with its spreadsheet row number and skipped. The rest of the file still imports."},
	{"Extra columns", "Ignored. Keep whatever else your spreadsheet carries; map only the columns above."},
}

// rulesRow is the 1-based sheet row the "Rules" heading landed on, or 0
// when the notes no longer carry one.
func rulesRow() int {
	for i, row := range notes {
		if len(row) > 0 && row[0] == "Rules" {
			return i + 1
		}
	}
	return 0
}

func writeNotes(f *excelize.File, requirementType string) error {
	if _, err := f.NewSheet(notesSheet); err != nil {
		return err
	}
	head, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return err
	}
	wrap, err := f.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{Vertical: "top", WrapText: true}})
	if err != nil {
		return err
	}
	if err := f.SetColWidth(notesSheet, "A", "A", 20); err != nil {
		return err
	}
	if err := f.SetColWidth(notesSheet, "B", "B", 96); err != nil {
		return err
	}
	req := requirementLabel(requirementType)
	for r, row := range notes {
		for i, v := range row {
			col := "A"
			if i == 1 {
				col = "B"
			}
			if err := f.SetCellStr(notesSheet, fmt.Sprintf("%s%d", col, r+1), strings.ReplaceAll(v, "the project's requirement type", req)); err != nil {
				return err
			}
		}
	}
	if err := f.SetCellStyle(notesSheet, "A1", "B1", head); err != nil {
		return err
	}
	// The Rules heading is found rather than numbered: the column rows above
	// it grow whenever the importer learns a new column, and a hard-coded
	// row would quietly bold a rule instead.
	if rules := rulesRow(); rules > 0 {
		if err := f.SetCellStyle(notesSheet, fmt.Sprintf("A%d", rules), fmt.Sprintf("B%d", rules), head); err != nil {
			return err
		}
	}
	return f.SetCellStyle(notesSheet, "B2", fmt.Sprintf("B%d", len(notes)), wrap)
}

// TemplateCSV is the same starter file for anyone who would rather not open
// a workbook: the same columns and the same example rows, minus the notes.
func TemplateCSV(requirementType string) []byte {
	var b strings.Builder
	writeRow := func(cells []string) {
		for i, c := range cells {
			if i > 0 {
				b.WriteByte(',')
			}
			if strings.ContainsAny(c, `",`+"\n") {
				b.WriteString(`"` + strings.ReplaceAll(c, `"`, `""`) + `"`)
			} else {
				b.WriteString(c)
			}
		}
		b.WriteByte('\n')
	}
	writeRow(TemplateHeaders)
	for _, row := range templateRows(requirementType) {
		writeRow(row)
	}
	return []byte(b.String())
}
