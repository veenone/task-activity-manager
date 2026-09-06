package importfile_test

import (
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"agile-suite/core/importfile"
)

func TestParseRecordsCSVStripsTheBOMAndAllowsRaggedRows(t *testing.T) {
	data := append([]byte{0xEF, 0xBB, 0xBF}, []byte("Summary,Type\nFirst,Task\nSecond\n")...)
	recs, err := importfile.ParseRecords(data, false)
	if err != nil {
		t.Fatalf("ParseRecords: %v", err)
	}
	if len(recs) != 3 || recs[0][0] != "Summary" || len(recs[2]) != 1 || recs[2][0] != "Second" {
		t.Errorf("records = %v", recs)
	}
}

func TestParseRecordsXLSXReadsTheFirstSheet(t *testing.T) {
	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "Summary")
	_ = f.SetCellValue("Sheet1", "B1", "Points")
	_ = f.SetCellValue("Sheet1", "A2", "Add a retry")
	_ = f.SetCellValue("Sheet1", "B2", 3)
	_, _ = f.NewSheet("Other")
	_ = f.SetCellValue("Other", "A1", "ignored")
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	recs, err := importfile.ParseRecords(buf.Bytes(), true)
	if err != nil {
		t.Fatalf("ParseRecords xlsx: %v", err)
	}
	if len(recs) != 2 || recs[0][1] != "Points" || recs[1][0] != "Add a retry" || recs[1][1] != "3" {
		t.Errorf("records = %v", recs)
	}
	if _, err := importfile.ParseRecords([]byte("not a workbook"), true); err == nil || !strings.Contains(err.Error(), "open xlsx") {
		t.Errorf("bad xlsx: %v", err)
	}
}

func TestParsePreview(t *testing.T) {
	pv, err := importfile.ParsePreview([][]string{{"A", "B"}, {"1", "2"}, {"3"}})
	if err != nil || len(pv.Headers) != 2 || pv.RowCount != 2 {
		t.Errorf("preview = %+v %v", pv, err)
	}
	if _, err := importfile.ParsePreview(nil); err == nil || err.Error() != "the file is empty" {
		t.Errorf("empty: %v", err)
	}
}
