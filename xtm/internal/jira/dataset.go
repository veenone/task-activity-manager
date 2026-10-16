package jira

import (
	"bytes"
	"context"
	"encoding/csv"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// DatasetParameter is one column of a test's dataset: a parameter a
// ${placeholder} in the test's steps or definition refers to.
//
// Xray distinguishes text from list parameters, but the CSV export carries
// values rather than list definitions, so a list parameter is
// indistinguishable from a text one here. The type is therefore not modelled.
// The raven/1.0 JSON API does carry it, with the allowed values and the list's
// scope and id, if a caller ever needs more than the table.
type DatasetParameter struct {
	Name string `json:"name"`
	// Combinatorial parameters are marked with an asterisk suffix on the CSV
	// column name. Xray expands them into extra iterations at execution time,
	// so the stored rows are fewer than the iterations that will run.
	Combinatorial bool `json:"combinatorial"`
}

// Dataset is a test's parameter table: the parameters as columns and one row
// per iteration. Rows are rectangular, each the length of Parameters.
//
// An empty Dataset means the test has no dataset, which is the common case.
type Dataset struct {
	Parameters []DatasetParameter `json:"parameters"`
	Rows       [][]string         `json:"rows"`
}

// datasetExportPath is Xray's CSV dataset export, added in Xray 5.1.0
// (XRAY-7868). contextKey names a Test Plan or Test Execution whose dataset
// overrides the test's own; empty asks for the test level.
const datasetExportPath = "/rest/raven/2.0/api/dataset/export"

// GetDataset returns the dataset attached to a test, or an empty Dataset when
// it has none (FR, issue #267 Phase 1).
//
// An instance below Xray 5.1 has no export endpoint and answers 404, the same
// status a test with no dataset gets. Reading both as "no dataset" is right
// for a read-only path, and a write path must not: the #277 W0 spike found the
// two are distinguishable by body, so no version check is needed. A missing
// dataset answers application/json {"error":"Dataset not found"}; a missing
// route answers Jersey's application/xml <status-code>404</status-code>. The
// fixtures in testdata hold one of each.
//
// The spike settled what #267 Phase 0 could only guess: the response is raw
// CSV with Content-Type application/octet-stream, not text/csv and not a JSON
// wrapper, and a list parameter's allowed values do not survive the export.
// They are not lost, only absent here: /rest/raven/1.0/dataset returns them,
// along with the level a dataset resolves from. See the Xray datasets section
// of agents/project/domain-context.md.
func (c *Client) GetDataset(ctx context.Context, testKey, contextKey string) (Dataset, error) {
	if isDemoURL(c.baseURL) {
		return demoDataset(testKey), nil
	}
	q := url.Values{}
	q.Set("testIssueKey", testKey)
	if contextKey != "" {
		q.Set("contextIssueKey", contextKey)
	}
	body, status, err := c.getBytesAnyType(ctx, datasetExportPath+"?"+q.Encode())
	if status == http.StatusNotFound {
		return Dataset{}, nil
	}
	if err != nil {
		return Dataset{}, err
	}
	return parseDatasetCSV(body), nil
}

// parseDatasetCSV reads Xray's dataset export. The header row names the
// parameters, each later row is one iteration, and a name ending in an
// asterisk is combinatorial.
//
// A malformed body yields an empty dataset rather than an error: the panel
// showing nothing is a better answer than a test that cannot be opened.
func parseDatasetCSV(body []byte) Dataset {
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
			Name:          strings.TrimSuffix(trimmed, "*"),
			Combinatorial: strings.HasSuffix(trimmed, "*"),
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

// fitRow makes a record exactly n cells long, padding a short row and
// truncating a long one, so every row lines up under its parameters.
func fitRow(rec []string, n int) []string {
	out := make([]string, n)
	copy(out, rec)
	return out
}
