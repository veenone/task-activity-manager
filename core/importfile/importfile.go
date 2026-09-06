// Package importfile turns an uploaded CSV or XLSX file into rows of cells,
// the first step of every spreadsheet import in the suite. It was lifted
// from XTM's testrepo importer, which now delegates here; mapping columns
// to fields and validating rows stays with each app.
package importfile

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Preview is what a freshly parsed file looks like before mapping: its
// column headers, the number of data rows, and a sample row so a mapping UI
// can show what each column actually holds.
type Preview struct {
	Headers  []string `json:"headers"`
	RowCount int      `json:"rowCount"`
	Sample   []string `json:"sample"`
}

// ParsePreview reads the header row, counts the data rows, and keeps the
// first data row as a sample; Sample is an empty slice when there is none.
func ParsePreview(records [][]string) (Preview, error) {
	if len(records) == 0 {
		return Preview{}, fmt.Errorf("the file is empty")
	}
	sample := []string{}
	if len(records) > 1 {
		sample = records[1]
	}
	return Preview{Headers: records[0], RowCount: len(records) - 1, Sample: sample}, nil
}

// ParseRecords parses raw file bytes into rows, CSV or XLSX. For XLSX the
// first worksheet is used.
func ParseRecords(data []byte, isXlsx bool) ([][]string, error) {
	if isXlsx {
		return ParseXLSX(data)
	}
	return ReadCSV(string(StripUTF8BOM(data)))
}

// utf8BOM is the UTF-8 byte-order mark (EF BB BF) that Excel and Windows
// editors prepend to saved CSVs. Left in place it fuses onto the first
// header cell, so column auto-mapping no longer recognizes "Summary".
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// StripUTF8BOM removes a leading UTF-8 BOM so the first column header maps
// cleanly.
func StripUTF8BOM(data []byte) []byte {
	return bytes.TrimPrefix(data, utf8BOM)
}

// ReadCSV parses CSV content leniently (variable field counts allowed).
func ReadCSV(content string) ([][]string, error) {
	reader := csv.NewReader(strings.NewReader(content))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse CSV: %w", err)
	}
	return records, nil
}

// ParseXLSX reads the first worksheet of an XLSX file into rows.
func ParseXLSX(data []byte) ([][]string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer func() { _ = f.Close() }()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("the workbook has no sheets")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("read xlsx rows: %w", err)
	}
	return rows, nil
}
