package export

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestWorkbookWritesEachSheetWithNumbersAsNumbers(t *testing.T) {
	raw, err := Workbook([]Sheet{
		{Name: "Links", Header: []string{"From", "To", "Count"}, Rows: [][]string{{"ALPHA", "BRAVO", "3"}, {"ALPHA", "CHARLIE", "12"}}},
		{Name: "Cases", Header: []string{"Case", "Reason"}, Rows: [][]string{{"ACME-1", "007 is a build number"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if got := f.GetSheetList(); len(got) != 2 || got[0] != "Links" || got[1] != "Cases" {
		t.Fatalf("sheets %v", got)
	}
	if v, _ := f.GetCellValue("Links", "C3"); v != "12" {
		t.Fatalf("C3 = %q", v)
	}
	if typ, _ := f.GetCellType("Links", "C2"); typ != excelize.CellTypeUnset && typ != excelize.CellTypeNumber {
		t.Fatalf("a count was stored as %v", typ)
	}
	if v, _ := f.GetCellValue("Cases", "B2"); v != "007 is a build number" {
		t.Fatalf("B2 = %q", v)
	}
	panes, _ := f.GetPanes("Links")
	if !panes.Freeze || panes.YSplit != 1 {
		t.Fatalf("the header row is not frozen: %+v", panes)
	}
}

func TestWorkbookKeepsLeadingZerosAndTrimsSheetNames(t *testing.T) {
	raw, err := Workbook([]Sheet{{Name: "Verdicts that changed, by case and project", Header: []string{"Key"}, Rows: [][]string{{"0042"}}}})
	if err != nil {
		t.Fatal(err)
	}
	f, _ := excelize.OpenReader(bytes.NewReader(raw))
	name := f.GetSheetList()[0]
	if len([]rune(name)) > 31 {
		t.Fatalf("sheet name %q is longer than Excel allows", name)
	}
	if v, _ := f.GetCellValue(name, "A2"); v != "0042" {
		t.Fatalf("A2 = %q", v)
	}
}

func TestWorkbookRefusesNoSheets(t *testing.T) {
	if _, err := Workbook(nil); err == nil {
		t.Fatal("an empty workbook was written")
	}
}
