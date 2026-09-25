package reportout

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func deck(t *testing.T, d Document) map[string]string {
	t.Helper()
	data, err := PPTX(d)
	if err != nil {
		t.Fatalf("pptx: %v", err)
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("the deck is not a readable zip: %v", err)
	}
	parts := map[string]string{}
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		parts[f.Name] = string(body)
	}
	return parts
}

func TestPPTXHasThePartsPowerPointLooksFor(t *testing.T) {
	parts := deck(t, sample())
	for _, want := range []string{
		"[Content_Types].xml",
		"_rels/.rels",
		"ppt/presentation.xml",
		"ppt/_rels/presentation.xml.rels",
		"ppt/slideMasters/slideMaster1.xml",
		"ppt/slideMasters/_rels/slideMaster1.xml.rels",
		"ppt/slideLayouts/slideLayout1.xml",
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels",
		"ppt/theme/theme1.xml",
		"ppt/slides/slide1.xml",
		"ppt/slides/_rels/slide1.xml.rels",
	} {
		if _, ok := parts[want]; !ok {
			t.Errorf("the deck has no %s", want)
		}
	}
	// Every slide is declared, related and typed, or PowerPoint repairs the
	// file rather than opening it.
	for name := range parts {
		if !strings.HasPrefix(name, "ppt/slides/slide") {
			continue
		}
		if !strings.Contains(parts["[Content_Types].xml"], "/"+name) {
			t.Errorf("%s has no content type", name)
		}
		if !strings.Contains(parts["ppt/_rels/presentation.xml.rels"], name[len("ppt/"):]) {
			t.Errorf("%s is not related to the presentation", name)
		}
	}
}

func TestPPTXOpensOnTheTitleThenOneSlidePerSection(t *testing.T) {
	d := sample()
	d.Sections = append(d.Sections, Section{Heading: "Velocity, oldest sprint first", Lines: []string{"nothing yet"}})
	parts := deck(t, d)
	if !strings.Contains(parts["ppt/slides/slide1.xml"], "Sprint 11 · Report") {
		t.Errorf("the first slide is not the title:\n%s", parts["ppt/slides/slide1.xml"])
	}
	if !strings.Contains(parts["ppt/slides/slide2.xml"], "Sprint outcome") {
		t.Error("the second slide is not the first section")
	}
	if !strings.Contains(parts["ppt/slides/slide3.xml"], "Velocity, oldest sprint first") {
		t.Error("the third slide is not the second section")
	}
	if _, ok := parts["ppt/slides/slide4.xml"]; ok {
		t.Error("a fourth slide was written for a two section report")
	}
}

func TestPPTXPutsTheFiguresInATableAndTheCaveatsOnTheSameSlide(t *testing.T) {
	slide := deck(t, sample())["ppt/slides/slide2.xml"]
	if !strings.Contains(slide, "<a:tbl>") {
		t.Errorf("the figures are not a table:\n%s", slide)
	}
	for _, want := range []string{"Figure", "Committed", "34 points", "Committed is a minimum estimate."} {
		if !strings.Contains(slide, want) {
			t.Errorf("the slide is missing %q", want)
		}
	}
}

func TestPPTXSplitsALongTableAndRepeatsTheCaveatsOnEverySlide(t *testing.T) {
	d := sample()
	rows := [][]string{}
	for i := 0; i < 40; i++ {
		rows = append(rows, []string{"2 Mar", "34 points"})
	}
	d.Sections[0].Table.Rows = rows
	parts := deck(t, d)
	var slides []string
	for name, body := range parts {
		if strings.HasPrefix(name, "ppt/slides/slide") && strings.Contains(body, "Sprint outcome") {
			slides = append(slides, body)
		}
	}
	if len(slides) < 2 {
		t.Fatalf("40 rows fit on %d slide(s); they would run off the bottom", len(slides))
	}
	for i, body := range slides {
		if !strings.Contains(body, "Committed is a minimum estimate.") {
			t.Errorf("continuation slide %d dropped the caveat", i)
		}
		if !strings.Contains(body, "<a:t>Figure</a:t>") {
			t.Errorf("continuation slide %d dropped the table's column names", i)
		}
	}
}

func TestPPTXIsBuiltOnTheTemplate(t *testing.T) {
	parts := deck(t, sample())

	// The template's design travels with the deck: its master, its theme,
	// its two named layouts and its table style.
	if !strings.Contains(parts["ppt/slideLayouts/slideLayout1.xml"], `name="`+titleLayoutName+`"`) {
		t.Errorf("the first layout is not the template's %s layout", titleLayoutName)
	}
	sectionLayout := ""
	for name, body := range parts {
		if strings.HasPrefix(name, "ppt/slideLayouts/slideLayout") && strings.Contains(body, `name="`+sectionLayoutName+`"`) {
			sectionLayout = name
		}
	}
	if sectionLayout == "" {
		t.Fatalf("the deck carries no %s layout", sectionLayoutName)
	}
	if !strings.Contains(parts["ppt/tableStyles.xml"], tableStyleID) {
		t.Errorf("the deck carries no table style:\n%s", parts["ppt/tableStyles.xml"])
	}

	// A template's own main part type would make PowerPoint open the export
	// as a new unsaved deck rather than as the file the user asked for.
	types := parts["[Content_Types].xml"]
	if strings.Contains(types, "presentationml.template.main") {
		t.Error("the export is still typed as a template")
	}
	if !strings.Contains(types, "presentationml.presentation.main") {
		t.Error("the export has no presentation content type")
	}

	// Every slide sits on a layout from the template, which is what gives
	// it the theme's fonts and colours rather than PowerPoint's defaults.
	if !strings.Contains(parts["ppt/slides/_rels/slide1.xml.rels"], "slideLayout1.xml") {
		t.Errorf("the title slide is not on the %s layout", titleLayoutName)
	}
	want := sectionLayout[len("ppt/slideLayouts/"):]
	if !strings.Contains(parts["ppt/slides/_rels/slide2.xml.rels"], want) {
		t.Errorf("a section slide is not on the %s layout", sectionLayoutName)
	}

	// Heading, sentences and caveats are the layout's placeholders, so the
	// template decides where they sit and how they read.
	section := parts["ppt/slides/slide2.xml"]
	for _, ph := range []string{`<p:ph type="title"/>`, `<p:ph idx="1"/>`, `<p:ph idx="2"/>`} {
		if !strings.Contains(section, ph) {
			t.Errorf("the section slide does not use the placeholder %s:\n%s", ph, section)
		}
	}
	if !strings.Contains(parts["ppt/slides/slide1.xml"], `<p:ph type="ctrTitle"/>`) {
		t.Error("the title slide does not use the layout's title placeholder")
	}
	if !strings.Contains(section, tableStyleID) {
		t.Error("the table is a bare grid; it names no table style")
	}
}

func TestPPTXEscapesWhatWouldBeMarkup(t *testing.T) {
	d := sample()
	d.Sections[0].Lines = []string{`a <b> & "quoted" sprint`}
	slide := deck(t, d)["ppt/slides/slide2.xml"]
	if strings.Contains(slide, "<b>") {
		t.Errorf("an angle bracket reached the slide as markup:\n%s", slide)
	}
}

func TestPPTXRefusesADocumentWithNothingInIt(t *testing.T) {
	if _, err := PPTX(Document{}); err == nil {
		t.Fatal("want a refusal, got nil")
	}
}
