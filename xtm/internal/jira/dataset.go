package jira

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
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

// datasetWritePath is Xray's JSON dataset resource. It is not the documented
// 2.0 CSV import: that one merges and takes no flag saying otherwise, while
// this one replaces the whole table, which is what an editor needs. The
// #277 W0 spike confirmed every operation against a live instance; see the
// Xray datasets section of agents/project/domain-context.md.
const datasetWritePath = "/rest/raven/1.0/dataset"

// newParameterID is the id a parameter Xray does not yet own must carry. Any
// negative value works; omitting the id or sending zero is refused with
// "Parameters (0) not associated with the dataset", which is the single
// easiest thing to get wrong about this API.
const newParameterID = -1

// datasetPayload is the body the JSON resource accepts. It is the bare params
// object: the wrapper the matching GET returns is rejected with a 500 on
// getParams(), so this type deliberately does not mirror the read shape.
type datasetPayload struct {
	Params []datasetColumn `json:"params"`
}

// datasetColumn is one parameter and the values beneath it. Xray stores a
// dataset by column, while everything above this layer holds it by row.
type datasetColumn struct {
	Param  datasetParamDef `json:"param"`
	Values []datasetValue  `json:"values"`
}

type datasetParamDef struct {
	ID            int              `json:"id"`
	Name          string           `json:"name"`
	Rank          int              `json:"rank"`
	Combinatorial bool             `json:"combinatorial"`
	Info          datasetParamInfo `json:"info"`
}

// datasetParamInfo carries the parameter's type. Only TEXT is written: a LIST
// parameter needs allowed values, which the table this layer receives does not
// model.
type datasetParamInfo struct {
	Type    string   `json:"type"`
	Options []string `json:"options"`
}

type datasetValue struct {
	Value string `json:"value"`
	Rank  int    `json:"rank"`
}

// SetDataset replaces the dataset on a test with d, creating one when the test
// has none and clearing it when d is empty (#277 W1).
//
// Every parameter is announced as new, which is correct while this is the
// create path; editing an existing dataset has to send each parameter's own id
// so a rename stays a rename, and that is W2's problem.
//
// A successful write answers with a plain sentence under a JSON content type,
// so the response is deliberately not decoded.
func (c *Client) SetDataset(ctx context.Context, testKey string, d Dataset) error {
	if isDemoURL(c.baseURL) {
		return fmt.Errorf("dataset writes are not available on a demo profile")
	}
	id, err := c.issueID(ctx, testKey)
	if err != nil {
		return err
	}
	q := url.Values{}
	q.Set("entityId", id)
	q.Set("type", "TEST")
	return c.post(ctx, datasetWritePath+"?"+q.Encode(), datasetPayload{
		Params: datasetColumns(d),
	})
}

// issueIDFields is the field list of the issue GET that resolves a key to an
// id. Jira answers with the id at the top level whatever is asked for, and
// asking for nothing is not a documented option, so this asks for the one
// field every issue has.
const issueIDFields = "summary"

// issueID resolves an issue key to the numeric id Jira assigned it. The
// dataset write needs it because /rest/raven/1.0/dataset addresses an entity
// by id and accepts no key; the CSV export on raven/2.0 takes the key, which
// is why the read path never had to do this.
func (c *Client) issueID(ctx context.Context, key string) (string, error) {
	var resp struct {
		ID string `json:"id"`
	}
	if err := c.get(ctx, "/rest/api/2/issue/"+url.PathEscape(key)+"?fields="+issueIDFields, &resp); err != nil {
		return "", fmt.Errorf("resolve %s: %w", key, err)
	}
	if resp.ID == "" {
		return "", fmt.Errorf("jira returned no issue id for %s", key)
	}
	return resp.ID, nil
}

// datasetColumns transposes the row-major table every caller holds into the
// column-major one Xray stores. A row shorter than the parameter list
// contributes an empty cell rather than shortening the column, so every
// parameter ends up with the same number of values.
func datasetColumns(d Dataset) []datasetColumn {
	cols := make([]datasetColumn, 0, len(d.Parameters))
	for i, p := range d.Parameters {
		values := make([]datasetValue, 0, len(d.Rows))
		for r, row := range d.Rows {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			values = append(values, datasetValue{Value: cell, Rank: r})
		}
		cols = append(cols, datasetColumn{
			Param: datasetParamDef{
				ID:            newParameterID,
				Name:          p.Name,
				Rank:          i,
				Combinatorial: p.Combinatorial,
				Info:          datasetParamInfo{Type: "TEXT", Options: []string{}},
			},
			Values: values,
		})
	}
	return cols
}
