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
// Xray also distinguishes text from list parameters, but the CSV export
// carries values rather than list definitions, so a list parameter is
// indistinguishable from a text one here. The type is therefore not modelled.
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
// An instance below Xray 5.1 has no export endpoint and answers 404. XTM does
// not version-gate the feature, because nothing in the Xray REST API reports
// its version: Settings exposes only teststatuses and teststepstatuses. A 404
// is therefore read as "no dataset", which renders identically to a test that
// genuinely has none. That equivalence holds only while this path is
// read-only; a write phase has to tell the two apart before it sends anything.
//
// NOTE(xtm): #267 Phase 0 left these unverified against a live instance,
// because the public docs do not state them: whether the export answers with
// raw text/csv or a JSON wrapper, and whether a list parameter's allowed
// values survive the round trip. The parser reads the body as CSV either way
// and would return an empty dataset from a JSON wrapper rather than fail.
func (c *Client) GetDataset(ctx context.Context, testKey, contextKey string) (Dataset, error) {
	if isDemoURL(c.baseURL) {
		return demoDataset(testKey), nil
	}
	q := url.Values{}
	q.Set("testIssueKey", testKey)
	if contextKey != "" {
		q.Set("contextIssueKey", contextKey)
	}
	body, status, err := c.getBytesStatus(ctx, datasetExportPath+"?"+q.Encode())
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
