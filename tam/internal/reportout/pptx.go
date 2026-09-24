package reportout

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
)

// The deck's geometry, in EMU, which is the unit OOXML measures in: 914400
// to the inch. The slide is the 16:9 one PowerPoint opens on.
const (
	slideWidth  = 12192000
	slideHeight = 6858000
	margin      = 685800
	bodyWidth   = slideWidth - 2*margin
	titleTop    = 457200
	titleHeight = 685800
	linesTop    = 1219200
	linesHeight = 1066800
	notesTop    = 5410200
	notesHeight = 1143000
	rowHeight   = 274320
	gap         = 152400
)

// PPTX is a report as a deck: a title slide, then a slide per section
// carrying its sentences, its figures as a real table, and the caveats on
// them.
//
// It is written straight into a zip of XML with the standard library,
// because that is all a pptx is. Text and tables only: a chart part would
// be a second way of saying what the table already says, and the decision
// on this report is that a chart travels as its numbers.
//
// A table too long for one slide is continued on the next, under the same
// heading and with the same caveats, rather than running off the bottom
// where nobody sees it.
func PPTX(d Document) ([]byte, error) {
	if err := d.Check(); err != nil {
		return nil, err
	}
	slides := []string{titleSlide(d.Title)}
	for _, s := range d.Sections {
		slides = append(slides, sectionSlides(s)...)
	}

	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	parts := [][2]string{
		{"[Content_Types].xml", contentTypes(len(slides))},
		{"_rels/.rels", rootRels()},
		{"ppt/presentation.xml", presentation(len(slides))},
		{"ppt/_rels/presentation.xml.rels", presentationRels(len(slides))},
		{"ppt/slideMasters/slideMaster1.xml", slideMaster()},
		{"ppt/slideMasters/_rels/slideMaster1.xml.rels", slideMasterRels()},
		{"ppt/slideLayouts/slideLayout1.xml", slideLayout()},
		{"ppt/slideLayouts/_rels/slideLayout1.xml.rels", slideLayoutRels()},
		{"ppt/theme/theme1.xml", theme()},
	}
	for i, body := range slides {
		parts = append(parts,
			[2]string{fmt.Sprintf("ppt/slides/slide%d.xml", i+1), body},
			[2]string{fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", i+1), slideRels()})
	}
	for _, p := range parts {
		w, err := z.Create(p[0])
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(p[1])); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// sectionSlides is one section, over as many slides as its table needs.
func sectionSlides(s Section) []string {
	top := linesTop
	if len(s.Lines) > 0 {
		top = linesTop + linesHeight + gap
	}
	rows := s.Table.Rows
	perSlide := (notesTop - top - gap) / rowHeight
	// One row of the budget is the column names, which every slide repeats.
	if perSlide < 2 {
		perSlide = 2
	}
	perSlide--

	var out []string
	for first := true; first || len(rows) > 0; first = false {
		take := rows
		if len(take) > perSlide {
			take = take[:perSlide]
		}
		rows = rows[len(take):]
		out = append(out, slide(s, Table{Columns: s.Table.Columns, Rows: take}, top))
		if len(rows) == 0 {
			break
		}
	}
	return out
}

func slide(s Section, t Table, tableTop int) string {
	var b strings.Builder
	b.WriteString(xmlHeader + `<p:sld ` + nsDecls + `><p:cSld><p:spTree>` + emptyTree)
	id := 2
	b.WriteString(textBox(&id, s.Heading, margin, titleTop, bodyWidth, titleHeight, 2400, true))
	if len(s.Lines) > 0 {
		b.WriteString(textBox(&id, strings.Join(s.Lines, "\n"), margin, linesTop, bodyWidth, linesHeight, 1400, false))
	}
	if len(t.Columns) > 0 {
		b.WriteString(tableFrame(&id, t, tableTop))
	}
	if len(s.Notes) > 0 {
		b.WriteString(textBox(&id, strings.Join(s.Notes, "\n"), margin, notesTop, bodyWidth, notesHeight, 1000, false))
	}
	b.WriteString(`</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`)
	return b.String()
}

func titleSlide(title string) string {
	id := 2
	return xmlHeader + `<p:sld ` + nsDecls + `><p:cSld><p:spTree>` + emptyTree +
		textBox(&id, title, margin, (slideHeight-titleHeight)/2, bodyWidth, titleHeight, 3600, true) +
		`</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`
}

// textBox is a shape holding one paragraph per line of text. normAutofit is
// what keeps a caveat longer than its box on the slide instead of over the
// edge of it, which for these boxes is the caveats' whole point.
func textBox(id *int, text string, x, y, cx, cy, size int, bold bool) string {
	var body strings.Builder
	for _, line := range strings.Split(text, "\n") {
		body.WriteString(paragraph(line, size, bold))
	}
	shape := fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="Text %d"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>`+
		`<p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>`+
		`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/></p:spPr>`+
		`<p:txBody><a:bodyPr wrap="square"><a:normAutofit/></a:bodyPr><a:lstStyle/>%s</p:txBody></p:sp>`,
		*id, *id, x, y, cx, cy, body.String())
	*id++
	return shape
}

func paragraph(text string, size int, bold bool) string {
	b := ""
	if bold {
		b = ` b="1"`
	}
	if text == "" {
		return fmt.Sprintf(`<a:p><a:endParaRPr lang="en-US" sz="%d"/></a:p>`, size)
	}
	return fmt.Sprintf(`<a:p><a:r><a:rPr lang="en-US" sz="%d"%s/><a:t>%s</a:t></a:r></a:p>`, size, b, esc(text))
}

func tableFrame(id *int, t Table, y int) string {
	width := bodyWidth / len(t.Columns)
	var grid strings.Builder
	for range t.Columns {
		grid.WriteString(fmt.Sprintf(`<a:gridCol w="%d"/>`, width))
	}
	var body strings.Builder
	body.WriteString(tableRow(t.Columns, true))
	for _, r := range t.Rows {
		body.WriteString(tableRow(r, false))
	}
	height := rowHeight * (len(t.Rows) + 1)
	frame := fmt.Sprintf(`<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="%d" name="Table %d"/>`+
		`<p:cNvGraphicFramePr><a:graphicFrameLocks noGrp="1"/></p:cNvGraphicFramePr><p:nvPr/></p:nvGraphicFramePr>`+
		`<p:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></p:xfrm>`+
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table">`+
		`<a:tbl><a:tblPr firstRow="1" bandRow="1"/><a:tblGrid>%s</a:tblGrid>%s</a:tbl>`+
		`</a:graphicData></a:graphic></p:graphicFrame>`,
		*id, *id, margin, y, width*len(t.Columns), height, grid.String(), body.String())
	*id++
	return frame
}

func tableRow(cells []string, heading bool) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<a:tr h="%d">`, rowHeight))
	for _, c := range cells {
		b.WriteString(`<a:tc><a:txBody><a:bodyPr/><a:lstStyle/>` + paragraph(c, 1200, heading) +
			`</a:txBody><a:tcPr/></a:tc>`)
	}
	b.WriteString(`</a:tr>`)
	return b.String()
}
