package reportout

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func sheetRows(t *testing.T, data []byte) [][]string {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("the spreadsheet would not open: %v", err)
	}
	defer f.Close()
	names := f.GetSheetList()
	if len(names) != 1 {
		t.Fatalf("sheets = %v, want one", names)
	}
	rows, err := f.GetRows(names[0])
	if err != nil {
		t.Fatalf("read the rows: %v", err)
	}
	return rows
}

func flat(rows [][]string) string {
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(strings.Join(r, "\t") + "\n")
	}
	return b.String()
}

func TestXLSXCarriesTheTitleTheFiguresAndTheCaveats(t *testing.T) {
	data, err := XLSX(sample())
	if err != nil {
		t.Fatalf("xlsx: %v", err)
	}
	rows := sheetRows(t, data)
	text := flat(rows)
	for _, want := range []string{
		"Sprint 11 · Report",
		"Sprint outcome",
		"Closed sprint · final results",
		"Figure\tAmount",
		"Committed\t34 points",
		"Committed is a minimum estimate.",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the spreadsheet is missing %q:\n%s", want, text)
		}
	}
	if i, j := strings.Index(text, "34 points"), strings.Index(text, "Committed is a minimum"); i > j {
		t.Error("the caveat is above the figures it qualifies; it belongs under them")
	}
}

func TestXLSXKeepsAFigureInItsOwnCell(t *testing.T) {
	rows := sheetRows(t, mustXLSX(t, sample()))
	var found bool
	for _, r := range rows {
		if len(r) == 2 && r[0] == "Completed" && r[1] == "29 points" {
			found = true
		}
	}
	if !found {
		t.Errorf("no row holds Completed and its amount in two cells:\n%s", flat(rows))
	}
}

func TestXLSXRefusesADocumentWithNothingInIt(t *testing.T) {
	if _, err := XLSX(Document{}); err == nil {
		t.Fatal("want a refusal, got nil")
	}
}

// sheetStyle is the style excelize resolved for a cell, so a test can ask
// what a reader will see rather than what the writer meant.
func sheetStyle(t *testing.T, f *excelize.File, cell string) *excelize.Style {
	t.Helper()
	idx, err := f.GetCellStyle(sheetName, cell)
	if err != nil {
		t.Fatalf("style of %s: %v", cell, err)
	}
	style, err := f.GetStyle(idx)
	if err != nil {
		t.Fatalf("style %d: %v", idx, err)
	}
	return style
}

func TestXLSXTakesItsLookFromTheTemplate(t *testing.T) {
	f, err := excelize.OpenReader(bytes.NewReader(mustXLSX(t, sample())))
	if err != nil {
		t.Fatalf("the spreadsheet would not open: %v", err)
	}
	defer f.Close()

	rows, err := f.GetRows(sheetName)
	if err != nil {
		t.Fatalf("read the rows: %v", err)
	}
	// The template's style key is consumed, not shipped: a reader opening
	// the export must not find the words that named its styles.
	for _, r := range rows {
		if len(r) > 0 && (r[0] == "Table header" || r[0] == "Caveat") {
			t.Fatalf("the template's style key reached the export:\n%s", flat(rows))
		}
	}

	title := sheetStyle(t, f, "A1")
	if title.Font == nil || !title.Font.Bold || title.Font.Size < 14 {
		t.Errorf("the title is not the template's title style: %+v", title.Font)
	}

	// The header row is the styled difference a reader notices first.
	head := ""
	for i, r := range rows {
		if len(r) > 1 && r[0] == "Figure" && r[1] == "Amount" {
			head = fmt.Sprintf("A%d", i+1)
		}
	}
	if head == "" {
		t.Fatalf("no header row in:\n%s", flat(rows))
	}
	hs := sheetStyle(t, f, head)
	if hs.Font == nil || !hs.Font.Bold {
		t.Errorf("the header row is not bold: %+v", hs.Font)
	}
	if hs.Fill.Type != "pattern" || len(hs.Fill.Color) == 0 {
		t.Errorf("the header row carries no fill: %+v", hs.Fill)
	}
	if len(hs.Border) == 0 {
		t.Error("the header row carries no border")
	}

	width, err := f.GetColWidth(sheetName, "A")
	if err != nil {
		t.Fatalf("column width: %v", err)
	}
	if width < 44 {
		t.Errorf("column A is %v wide; a caveat needs the template's width", width)
	}
}

func mustXLSX(t *testing.T, d Document) []byte {
	t.Helper()
	data, err := XLSX(d)
	if err != nil {
		t.Fatalf("xlsx: %v", err)
	}
	return data
}
