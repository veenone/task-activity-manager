package issueexport

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image/png"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// The dependency export (#612) is the Dependencies view as a workbook a
// manager can report from: an Issues sheet saying how each issue is linked,
// a Links sheet with one row per link, and the graph as drawn when the view
// sends a picture of it. It lives beside the backlog export to share its
// styles; the two are different reports from the same cache.

// DependencyScope says what to export. No focus is the whole profile: every
// cached issue, linked or not, and every link of any type, whatever the
// view's filters. A focus is one issue's chain as the view lights it: the
// issues in Keys, and the links of Types between two of them.
type DependencyScope struct {
	Focus string   `json:"focus"`
	Keys  []string `json:"keys"`
	Types []string `json:"types"`
}

// Dependencies is what the workbook is written from.
type Dependencies struct {
	Focus    string
	Issues   []backend.Issue
	External []issuerepo.ExternalIssue
	Links    []issuerepo.DependencyLink
	// Image is the graph as a PNG; nil leaves the Graph sheet out.
	Image []byte
}

// SelectDependencies narrows the profile's issues and its dependency graph
// to the scope, keeping their order.
func SelectDependencies(all []backend.Issue, g issuerepo.DependencyGraph, s DependencyScope) Dependencies {
	d := Dependencies{Focus: s.Focus, Issues: []backend.Issue{}, External: []issuerepo.ExternalIssue{}, Links: []issuerepo.DependencyLink{}}
	if s.Focus == "" {
		d.Issues = append(d.Issues, all...)
		d.External = append(d.External, g.External...)
		d.Links = append(d.Links, g.Links...)
		return d
	}
	keys := map[string]bool{s.Focus: true}
	for _, k := range s.Keys {
		keys[k] = true
	}
	types := map[string]bool{}
	for _, t := range s.Types {
		types[strings.ToLower(t)] = true
	}
	for _, iss := range all {
		if keys[iss.Key] {
			d.Issues = append(d.Issues, iss)
		}
	}
	for _, e := range g.External {
		if keys[e.Key] {
			d.External = append(d.External, e)
		}
	}
	for _, l := range g.Links {
		if keys[l.Outward] && keys[l.Inward] && (len(types) == 0 || types[strings.ToLower(l.Type)]) {
			d.Links = append(d.Links, l)
		}
	}
	return d
}

// DecodePNG reads the picture the view sends as a data URL. It crosses the
// IPC boundary, so anything that is not a PNG is refused rather than written
// into a workbook Excel then cannot open. An empty string is no picture.
func DecodePNG(dataURL string) ([]byte, error) {
	if dataURL == "" {
		return nil, nil
	}
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(dataURL, prefix) {
		return nil, errors.New("the graph picture is not a PNG")
	}
	b, err := base64.StdEncoding.DecodeString(dataURL[len(prefix):])
	if err != nil {
		return nil, fmt.Errorf("read the graph picture: %w", err)
	}
	if _, err := png.DecodeConfig(bytes.NewReader(b)); err != nil {
		return nil, fmt.Errorf("read the graph picture: %w", err)
	}
	return b, nil
}

const (
	issuesSheet = "Issues"
	linksSheet  = "Links"
	graphSheet  = "Graph"
)

var (
	depIssueColumns = []column{
		{head: "Key", width: 14},
		{head: "Type", width: 14},
		{head: "Summary", width: 52},
		{head: "Status", width: 18},
		{head: "Assignee", width: 18},
		{head: "Sprint", width: 20},
		{head: "Blocked by", width: 11, numeric: true},
		{head: "Blocks", width: 9, numeric: true},
		{head: "Other links", width: 11, numeric: true},
		{head: "Linked issues", width: 60},
	}
	chainColumn    = column{head: "In the chain", width: 14}
	depLinkColumns = []column{
		{head: "From", width: 14},
		{head: "From summary", width: 44},
		{head: "Type", width: 16},
		{head: "To", width: 14},
		{head: "To summary", width: 44},
		{head: "State", width: 16},
	}
)

// notCached is the status of a far end the profile does not cache.
const notCached = "Not in this profile"

// DependencyWorkbook writes the export.
func DependencyWorkbook(d Dependencies) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", issuesSheet); err != nil {
		return nil, fmt.Errorf("make the sheet: %w", err)
	}
	s, err := newStyles(f)
	if err != nil {
		return nil, err
	}
	cols := depIssueColumns
	if d.Focus != "" {
		cols = append(append([]column{}, depIssueColumns...), chainColumn)
	}
	if err := writeTable(f, s, issuesSheet, cols, issueRows(d)); err != nil {
		return nil, err
	}
	if _, err := f.NewSheet(linksSheet); err != nil {
		return nil, fmt.Errorf("make the sheet: %w", err)
	}
	if err := writeTable(f, s, linksSheet, depLinkColumns, linkRows(d)); err != nil {
		return nil, err
	}
	if d.Image != nil {
		if _, err := f.NewSheet(graphSheet); err != nil {
			return nil, fmt.Errorf("make the sheet: %w", err)
		}
		if err := f.AddPictureFromBytes(graphSheet, "A1", &excelize.Picture{
			Extension: ".png",
			File:      d.Image,
			Format:    &excelize.GraphicOptions{AltText: "Dependency graph", LockAspectRatio: true},
		}); err != nil {
			return nil, fmt.Errorf("add the graph: %w", err)
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("write the workbook: %w", err)
	}
	return buf.Bytes(), nil
}

// issueRows is a row per issue, the far ends last. A count says how many
// links an issue has of each kind; Linked issues names them, an arrow
// pointing the way the link reads, so "Blocks → B" is "this blocks B".
func issueRows(d Dependencies) [][]any {
	role := chainRoles(d)
	row := func(key, typ, summary, status, assignee, sprint string) []any {
		var blockedBy, blocks, other int
		var named []string
		for _, l := range d.Links {
			if l.Outward != key && l.Inward != key {
				continue
			}
			out := l.Outward == key
			switch {
			case !strings.EqualFold(l.Type, "blocks"):
				other++
			case out:
				blocks++
			default:
				blockedBy++
			}
			arrow, far := "→", l.Inward
			if !out {
				arrow, far = "←", l.Outward
			}
			named = append(named, fmt.Sprintf("%s %s %s%s", l.Type, arrow, far, stateNote(l)))
		}
		linked := strings.Join(named, "; ")
		if linked == "" {
			linked = "No dependencies"
		}
		r := []any{key, typ, summary, status, assignee, sprint, blockedBy, blocks, other, linked}
		if d.Focus != "" {
			r = append(r, role[key])
		}
		return r
	}
	rows := make([][]any, 0, len(d.Issues)+len(d.External))
	for _, i := range d.Issues {
		rows = append(rows, row(i.Key, i.Type, i.Summary, i.Status, i.Assignee, i.SprintName))
	}
	for _, e := range d.External {
		rows = append(rows, row(e.Key, e.IssueType, e.Summary, notCached, "", ""))
	}
	return rows
}

// chainRoles places each issue around the focus the way the view's chain
// does: upstream is what it waits on, downstream what waits on it.
func chainRoles(d Dependencies) map[string]string {
	roles := map[string]string{}
	if d.Focus == "" {
		return roles
	}
	walk := func(from, to func(issuerepo.DependencyLink) string, name string) {
		todo := []string{d.Focus}
		for len(todo) > 0 {
			at := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			for _, l := range d.Links {
				if next := to(l); from(l) == at && next != d.Focus && roles[next] == "" {
					roles[next] = name
					todo = append(todo, next)
				}
			}
		}
	}
	walk(func(l issuerepo.DependencyLink) string { return l.Inward }, func(l issuerepo.DependencyLink) string { return l.Outward }, "Upstream")
	walk(func(l issuerepo.DependencyLink) string { return l.Outward }, func(l issuerepo.DependencyLink) string { return l.Inward }, "Downstream")
	roles[d.Focus] = "Selected"
	return roles
}

func linkRows(d Dependencies) [][]any {
	summary := map[string]string{}
	for _, i := range d.Issues {
		summary[i.Key] = i.Summary
	}
	for _, e := range d.External {
		summary[e.Key] = e.Summary
	}
	rows := make([][]any, 0, len(d.Links))
	for _, l := range d.Links {
		rows = append(rows, []any{l.Outward, summary[l.Outward], l.Type, l.Inward, summary[l.Inward], linkState(l)})
	}
	return rows
}

func linkState(l issuerepo.DependencyLink) string {
	switch {
	case l.Removal:
		return "Removal pending"
	case l.Pending:
		return "Pending"
	}
	return "Committed"
}

func stateNote(l issuerepo.DependencyLink) string {
	if s := linkState(l); s != "Committed" {
		return " (" + strings.ToLower(s) + ")"
	}
	return ""
}

// writeTable writes a header and rows on one sheet and dresses it the way
// the backlog sheet is: widths, a frozen header and an autofilter.
func writeTable(f *excelize.File, s *styles, sheet string, cols []column, rows [][]any) error {
	for c, col := range cols {
		cell, _ := excelize.CoordinatesToCellName(c+1, 1)
		if err := f.SetCellStr(sheet, cell, col.head); err != nil {
			return fmt.Errorf("write the header: %w", err)
		}
		if err := f.SetCellStyle(sheet, cell, cell, s.head); err != nil {
			return fmt.Errorf("style the header: %w", err)
		}
		name, _ := excelize.ColumnNumberToName(c + 1)
		if err := f.SetColWidth(sheet, name, name, col.width); err != nil {
			return fmt.Errorf("width of %s: %w", col.head, err)
		}
	}
	for r, values := range rows {
		for c, v := range values {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				return fmt.Errorf("write %s: %w", cell, err)
			}
			style := s.text
			if cols[c].numeric {
				style = s.number
			}
			if err := f.SetCellStyle(sheet, cell, cell, style); err != nil {
				return fmt.Errorf("style %s: %w", cell, err)
			}
		}
	}
	if err := f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return fmt.Errorf("freeze the header: %w", err)
	}
	last, _ := excelize.ColumnNumberToName(len(cols))
	if err := f.AutoFilter(sheet, "A1:"+last+strconv.Itoa(len(rows)+1), nil); err != nil {
		return fmt.Errorf("add the filter: %w", err)
	}
	return nil
}
