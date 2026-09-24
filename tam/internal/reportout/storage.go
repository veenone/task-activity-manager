package reportout

import (
	"html"
	"strings"
)

// Storage is a document as Confluence storage format: a heading, its
// sentences, its table and its caveats, in that order, for each section.
//
// The charts are tables and not pictures. Confluence charts a table itself
// with its own macro when a reader wants a picture, and a table stays
// searchable, diffable and never goes stale against the page it sits on,
// which an attached image does.
//
// The notes follow the table because a caveat above a figure is read before
// there is anything to qualify.
func Storage(d Document) string {
	var b strings.Builder
	for _, s := range d.Sections {
		b.WriteString("<h2>" + esc(s.Heading) + "</h2>")
		for _, line := range s.Lines {
			para(&b, line)
		}
		table(&b, s.Table)
		for _, note := range s.Notes {
			para(&b, note)
		}
	}
	return b.String()
}

func esc(s string) string { return html.EscapeString(s) }

func para(b *strings.Builder, text string) {
	if text == "" {
		return
	}
	b.WriteString("<p>" + esc(text) + "</p>")
}

func table(b *strings.Builder, t Table) {
	if len(t.Columns) == 0 {
		return
	}
	b.WriteString("<table><tbody><tr>")
	for _, c := range t.Columns {
		b.WriteString("<th>" + esc(c) + "</th>")
	}
	b.WriteString("</tr>")
	for _, row := range t.Rows {
		b.WriteString("<tr>")
		for _, cell := range row {
			b.WriteString("<td>" + esc(cell) + "</td>")
		}
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody></table>")
}
