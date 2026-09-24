package reportout

import (
	"bytes"
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

func mustXLSX(t *testing.T, d Document) []byte {
	t.Helper()
	data, err := XLSX(d)
	if err != nil {
		t.Fatalf("xlsx: %v", err)
	}
	return data
}
