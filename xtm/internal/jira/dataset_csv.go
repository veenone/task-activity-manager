// Xray's dataset CSV: the format the 2.0 export serves and the import takes,
// read and written in one place so a round trip through a spreadsheet lands
// back where it started.
//
// The format carries names, the combinatorial asterisk and values. A
// parameter's id and the list behind it are not in it, which is why an import
// has to recover them from the table it replaces.

package jira

import (
	"bytes"
	"encoding/csv"
	"io"
	"strings"
)

// ParseDatasetCSV reads Xray's dataset export. The header row names the
// parameters, each later row is one iteration, and a name ending in an
// asterisk is combinatorial.
//
// A malformed body yields an empty dataset rather than an error: the panel
// showing nothing is a better answer than a test that cannot be opened.
func ParseDatasetCSV(body []byte) Dataset {
	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))))
	// Xray's own export is comma-delimited. Ragged rows are tolerated and
	// padded below, so the parser must not reject them on field count.
	r.FieldsPerRecord = -1

	header, err := r.Read()
	if err != nil {
		return Dataset{}
	}
	params := make([]DatasetParameter, 0, len(header))
	for _, name := range header {
		trimmed := strings.TrimSpace(name)
		params = append(params, DatasetParameter{
			Name:          strings.TrimSuffix(trimmed, combinatorialMark),
			Combinatorial: strings.HasSuffix(trimmed, combinatorialMark),
		})
	}

	var rows [][]string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Stop at the first unreadable row and keep what parsed. A
			// truncated table is more useful than none.
			break
		}
		rows = append(rows, fitRow(rec, len(params)))
	}
	return Dataset{Parameters: params, Rows: rows}
}

// DatasetCSV writes a table in the layout Xray's own export uses, so a user
// can take one out, edit it in a spreadsheet and bring it back (#277 W2).
//
// It carries names, the combinatorial asterisk and values, which is all the
// format has room for. A parameter's id and its list binding are not in it
// and cannot be: an import matches those back by name from the table it is
// replacing.
func DatasetCSV(d Dataset) []byte {
	if len(d.Parameters) == 0 {
		return nil
	}
	var out bytes.Buffer
	w := csv.NewWriter(&out)
	header := make([]string, len(d.Parameters))
	for i, p := range d.Parameters {
		header[i] = p.Name
		if p.Combinatorial {
			header[i] += combinatorialMark
		}
	}
	// Writing to a bytes.Buffer cannot fail, so the error csv reports would be
	// about the buffer rather than the data.
	_ = w.Write(header)
	for _, row := range d.Rows {
		_ = w.Write(fitRow(row, len(d.Parameters)))
	}
	w.Flush()
	return out.Bytes()
}

// combinatorialMark is the suffix Xray puts on a combinatorial parameter's
// column name in the export, and reads back off it on import.
const combinatorialMark = "*"

// fitRow makes a record exactly n cells long, padding a short row and
// truncating a long one, so every row lines up under its parameters.
func fitRow(rec []string, n int) []string {
	out := make([]string, n)
	copy(out, rec)
	return out
}
