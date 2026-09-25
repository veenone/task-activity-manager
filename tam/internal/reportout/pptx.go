package reportout

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// deckTemplate is the look of the export: the theme, the slide master, the
// two layouts a report uses and the table style, authored in PowerPoint
// rather than in Go literals. Restyling the deck is an edit to this file.
//
// The layouts are found by the name in their own XML rather than by part
// number, so adding or removing a layout in PowerPoint does not silently
// point the renderer at the wrong one.
//
//go:embed deck.potx
var deckTemplate []byte

// The names the template's two layouts carry, and the table style they are
// authored alongside.
const (
	titleLayoutName   = "Report title"
	sectionLayoutName = "Report section"
	tableStyleID      = "{6E25E649-3F16-4E02-A733-19D2CEDC3F1C}"
)

// Where a table sits on the Report section layout, in EMU, which is the
// unit OOXML measures in: 914400 to the inch. A table is not a placeholder,
// so this is the one piece of the section slide's geometry the template
// cannot hand over: it is the gap the layout leaves between the sentences
// and the caveats. Move a placeholder in the template and these move with
// it. cellSize is here for the same reason: a table style sets a cell's
// face, colour and weight but never its size.
const (
	tableLeft   = 685800
	tableTop    = 2286000
	tableWidth  = 10820400
	tableBottom = 5486400
	rowHeight   = 274320
	cellSize    = 1200
)

const xmlHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"

const relType = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/"

const nsDecls = `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
	`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
	`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`

// emptyTree is the group shape every spTree opens with, even one that holds
// nothing else.
const emptyTree = `<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
	`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/>` +
	`<a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`

// PPTX is a report as a deck: a title slide, then a slide per section
// carrying its sentences, its figures as a real table, and the caveats on
// them.
//
// The template is opened and filled rather than a deck being assembled from
// scratch, so every colour, font and cell border is in a file a designer can
// open. Go writes the slides and three bookkeeping parts, and copies the
// rest of the template through untouched.
//
// Text and tables only: a chart part would be a second way of saying what
// the table already says, and the decision on this report is that a chart
// travels as its numbers.
//
// A table too long for one slide is continued on the next, under the same
// heading and with the same caveats, rather than running off the bottom
// where nobody sees it.
func PPTX(d Document) ([]byte, error) {
	if err := d.Check(); err != nil {
		return nil, err
	}
	parts, order, err := unpack(deckTemplate)
	if err != nil {
		return nil, err
	}
	titleLayout, err := layoutNamed(parts, titleLayoutName)
	if err != nil {
		return nil, err
	}
	sectionLayout, err := layoutNamed(parts, sectionLayoutName)
	if err != nil {
		return nil, err
	}

	type slide struct{ body, layout string }
	slides := []slide{{titleSlide(d.Title), titleLayout}}
	for _, s := range d.Sections {
		for _, body := range sectionSlides(s) {
			slides = append(slides, slide{body, sectionLayout})
		}
	}

	const relsPart = "ppt/_rels/presentation.xml.rels"
	firstRel := nextRelID(parts[relsPart])
	parts["[Content_Types].xml"] = withSlideTypes(parts["[Content_Types].xml"], len(slides))
	parts["ppt/presentation.xml"], err = withSlideIDs(parts["ppt/presentation.xml"], len(slides), firstRel)
	if err != nil {
		return nil, err
	}
	parts[relsPart], err = withSlideRels(parts[relsPart], len(slides), firstRel)
	if err != nil {
		return nil, err
	}
	for i, s := range slides {
		order = append(order, fmt.Sprintf("ppt/slides/slide%d.xml", i+1))
		parts[order[len(order)-1]] = s.body
		order = append(order, fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", i+1))
		parts[order[len(order)-1]] = slideRels(s.layout)
	}

	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for _, name := range order {
		w, err := z.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(parts[name])); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// unpack reads the template into memory, keeping the order its parts were
// written in so the export is byte-comparable part for part.
func unpack(data []byte) (map[string]string, []string, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, nil, err
	}
	parts := make(map[string]string, len(r.File))
	order := make([]string, 0, len(r.File))
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return nil, nil, err
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, nil, err
		}
		parts[f.Name] = string(body)
		order = append(order, f.Name)
	}
	return parts, order, nil
}

var layoutName = regexp.MustCompile(`<p:cSld[^>]*name="([^"]*)"`)

// layoutNamed is the file name of the layout the template calls name.
func layoutNamed(parts map[string]string, name string) (string, error) {
	for part, body := range parts {
		if !strings.HasPrefix(part, "ppt/slideLayouts/slideLayout") {
			continue
		}
		if m := layoutName.FindStringSubmatch(body); m != nil && m[1] == name {
			return part[len("ppt/slideLayouts/"):], nil
		}
	}
	return "", fmt.Errorf("the deck template has no %q layout, so the report cannot be laid out on it", name)
}

var relID = regexp.MustCompile(`Id="rId(\d+)"`)

// nextRelID is the first relationship id the slides may take without
// colliding with one the template already spent on a master or a theme.
func nextRelID(rels string) int {
	next := 1
	for _, m := range relID.FindAllStringSubmatch(rels, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil && n >= next {
			next = n + 1
		}
	}
	return next
}

func withSlideTypes(types string, slides int) string {
	// A deck still typed as a template opens in PowerPoint as a new unsaved
	// copy rather than as the file the user asked for.
	types = strings.ReplaceAll(types, "presentationml.template.main", "presentationml.presentation.main")
	var b strings.Builder
	for i := 1; i <= slides; i++ {
		fmt.Fprintf(&b, `<Override PartName="/ppt/slides/slide%d.xml" `+
			`ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`, i)
	}
	return strings.Replace(types, "</Types>", b.String()+"</Types>", 1)
}

func withSlideIDs(pres string, slides, firstRel int) (string, error) {
	if !strings.Contains(pres, "<p:sldIdLst/>") {
		return "", errors.New("the deck template already lists slides of its own, so the report's slides have nowhere to go")
	}
	var b strings.Builder
	b.WriteString("<p:sldIdLst>")
	for i := 0; i < slides; i++ {
		fmt.Fprintf(&b, `<p:sldId id="%d" r:id="rId%d"/>`, 256+i, firstRel+i)
	}
	b.WriteString("</p:sldIdLst>")
	return strings.Replace(pres, "<p:sldIdLst/>", b.String(), 1), nil
}

func withSlideRels(rels string, slides, firstRel int) (string, error) {
	if !strings.Contains(rels, "</Relationships>") {
		return "", errors.New("the deck template's presentation relationships could not be read")
	}
	var b strings.Builder
	for i := 0; i < slides; i++ {
		fmt.Fprintf(&b, `<Relationship Id="rId%d" Type="%sslide" Target="slides/slide%d.xml"/>`,
			firstRel+i, relType, i+1)
	}
	return strings.Replace(rels, "</Relationships>", b.String()+"</Relationships>", 1), nil
}

func slideRels(layout string) string {
	return xmlHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="` + relType + `slideLayout" Target="../slideLayouts/` + layout + `"/>` +
		`</Relationships>`
}

func titleSlide(title string) string {
	return xmlHeader + `<p:sld ` + nsDecls + `><p:cSld><p:spTree>` + emptyTree +
		placeholder(2, "Title", `type="ctrTitle"`, []string{title}) +
		`</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`
}

// sectionSlides is one section, over as many slides as its table needs.
func sectionSlides(s Section) []string {
	// One row of the budget is the column names, which every slide repeats.
	perSlide := (tableBottom-tableTop)/rowHeight - 1
	if perSlide < 1 {
		perSlide = 1
	}
	rows := s.Table.Rows
	var out []string
	for first := true; first || len(rows) > 0; first = false {
		take := rows
		if len(take) > perSlide {
			take = take[:perSlide]
		}
		rows = rows[len(take):]
		out = append(out, sectionSlide(s, Table{Columns: s.Table.Columns, Rows: take}))
		if len(rows) == 0 {
			break
		}
	}
	return out
}

func sectionSlide(s Section, t Table) string {
	var b strings.Builder
	b.WriteString(xmlHeader + `<p:sld ` + nsDecls + `><p:cSld><p:spTree>` + emptyTree)
	b.WriteString(placeholder(2, "Heading", `type="title"`, []string{s.Heading}))
	if len(s.Lines) > 0 {
		b.WriteString(placeholder(3, "Lines", `idx="1"`, s.Lines))
	}
	if len(s.Notes) > 0 {
		b.WriteString(placeholder(4, "Caveats", `idx="2"`, s.Notes))
	}
	if len(t.Columns) > 0 {
		b.WriteString(tableFrame(5, t))
	}
	b.WriteString(`</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`)
	return b.String()
}

// placeholder fills one of the layout's placeholders, one paragraph per
// line. It carries no position and no run properties of its own: every one
// of those is inherited from the layout, which is the point of the template.
func placeholder(id int, name, ph string, lines []string) string {
	var body strings.Builder
	for _, line := range lines {
		body.WriteString(paragraph(line, 0))
	}
	return fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="%s"/>`+
		`<p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph %s/></p:nvPr></p:nvSpPr>`+
		`<p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/>%s</p:txBody></p:sp>`,
		id, name, ph, body.String())
}

// paragraph is one line of text. A size of zero leaves the size to whatever
// the shape inherits, which for a placeholder is the layout.
func paragraph(text string, size int) string {
	sz := ""
	if size > 0 {
		sz = fmt.Sprintf(` sz="%d"`, size)
	}
	if text == "" {
		return fmt.Sprintf(`<a:p><a:endParaRPr lang="en-US"%s/></a:p>`, sz)
	}
	return fmt.Sprintf(`<a:p><a:r><a:rPr lang="en-US"%s/><a:t>%s</a:t></a:r></a:p>`, sz, esc(text))
}

func tableFrame(id int, t Table) string {
	width := tableWidth / len(t.Columns)
	var grid strings.Builder
	for range t.Columns {
		fmt.Fprintf(&grid, `<a:gridCol w="%d"/>`, width)
	}
	var body strings.Builder
	body.WriteString(tableRow(t.Columns))
	for _, r := range t.Rows {
		body.WriteString(tableRow(r))
	}
	height := rowHeight * (len(t.Rows) + 1)
	return fmt.Sprintf(`<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="%d" name="Table %d"/>`+
		`<p:cNvGraphicFramePr><a:graphicFrameLocks noGrp="1"/></p:cNvGraphicFramePr><p:nvPr/></p:nvGraphicFramePr>`+
		`<p:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></p:xfrm>`+
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table">`+
		`<a:tbl><a:tblPr firstRow="1" bandRow="1"><a:tableStyleId>%s</a:tableStyleId></a:tblPr>`+
		`<a:tblGrid>%s</a:tblGrid>%s</a:tbl>`+
		`</a:graphicData></a:graphic></p:graphicFrame>`,
		id, id, tableLeft, tableTop, width*len(t.Columns), height, tableStyleID, grid.String(), body.String())
}

func tableRow(cells []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<a:tr h="%d">`, rowHeight)
	for _, c := range cells {
		b.WriteString(`<a:tc><a:txBody><a:bodyPr/><a:lstStyle/>` + paragraph(c, cellSize) +
			`</a:txBody><a:tcPr marT="45720" marB="45720" anchor="ctr"/></a:tc>`)
	}
	b.WriteString(`</a:tr>`)
	return b.String()
}
