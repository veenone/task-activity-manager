package reportout

import (
	"html"
	"strings"
)

// Storage is a document as Confluence storage format: a heading, its
// sentences, its table, its caveats and its charts, in that order, for each
// section.
//
// The figures stay a table as well as becoming a picture. A table on the page
// is searchable and diffable and an image is neither, so the chart is an
// addition to the rows rather than a replacement for them.
//
// The notes follow the table because a caveat above a figure is read before
// there is anything to qualify, and the charts follow the notes, where the
// spreadsheet and the deck put them too.
func Storage(d Document) string {
	var b strings.Builder
	for _, s := range d.Sections {
		b.WriteString("<h2>" + esc(s.Heading) + "</h2>")
		for _, line := range s.Lines {
			para(&b, line)
		}
		storageTable(&b, s.Table)
		for _, note := range s.Notes {
			para(&b, note)
		}
		for _, im := range s.Images {
			storageImage(&b, im)
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

// storageImage references a chart that Publish attaches to the page. Storage
// format cannot hold the bytes of a picture, so the file goes on the page and
// the body names it; ac:alt is what a reader who cannot see it is given.
//
// Both strings land in an XML attribute, so both are escaped like every other
// string here (I1). The file name has already been through Image.PNG, and it
// is escaped anyway rather than trusted twice.
func storageImage(b *strings.Builder, im Image) {
	b.WriteString(`<p><ac:image ac:alt="` + esc(im.Alt) + `"><ri:attachment ri:filename="` + esc(im.Name) + `"/></ac:image></p>`)
}

func storageTable(b *strings.Builder, t Table) {
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
