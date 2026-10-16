package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// DatasetParameter is one column of a test's dataset: a parameter a
// ${placeholder} in the test's steps or definition refers to.
//
// Everything past Name and Combinatorial is Xray's own bookkeeping, read from
// the raven/1.0 model and handed straight back on a write. It is here because
// a write replaces the whole table: a parameter sent without its id and its
// list binding is rebuilt from scratch, which is how #283 turned a list
// parameter into free text and lost its allowed values.
type DatasetParameter struct {
	Name string `json:"name"`
	// Combinatorial parameters are marked with an asterisk suffix on the CSV
	// column name. Xray expands them into extra iterations at execution time,
	// so the stored rows are fewer than the iterations that will run.
	Combinatorial bool `json:"combinatorial"`
	// ID is the parameter's id in Xray, zero for one that does not exist
	// there yet. Sending it back keeps the parameter, so a rename stays a
	// rename instead of a removal and an addition.
	ID int `json:"id,omitempty"`
	// Type is TEXT or LIST. A LIST parameter's allowed values are in Options,
	// and ListScope and ListID say which list they come from. XTM neither
	// creates nor edits lists (#277 scope); it carries them.
	Type      string   `json:"type,omitempty"`
	Options   []string `json:"options,omitempty"`
	ListScope string   `json:"listScope,omitempty"`
	ListID    int      `json:"listId,omitempty"`
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
// it has none (#267 Phase 1, #283).
//
// A test's own dataset is read from the raven/1.0 JSON model, because that is
// the only route that carries each parameter's id and, for a list parameter,
// its allowed values and the list it belongs to. The CSV export carries
// values alone, and a write that only knows what the export said rebuilds
// every parameter from scratch: new ids, and a list flattened into free text
// (#283). It costs one extra request, since the model is addressed by numeric
// issue id and publishes no key parameter.
//
// A container-level read still uses the export. The model addresses a Test
// Run rather than a Test Execution, and resolving that is #277 W3's job.
//
// Both routes answer 404 on an instance below Xray 5.1, which has neither.
// That reads as "no dataset", which is what the panel should show there. On
// the model route a test that simply has no dataset answers 200 with an empty
// params array instead, so nothing has to interpret a 404 as data. See the
// Xray datasets section of agents/project/domain-context.md.
func (c *Client) GetDataset(ctx context.Context, testKey, contextKey string) (Dataset, error) {
	if isDemoURL(c.baseURL) {
		return demoDataset(testKey), nil
	}
	if contextKey == "" {
		return c.datasetModel(ctx, testKey)
	}
	q := url.Values{}
	q.Set("testIssueKey", testKey)
	q.Set("contextIssueKey", contextKey)
	body, status, err := c.getBytesAnyType(ctx, datasetExportPath+"?"+q.Encode())
	if status == http.StatusNotFound {
		return Dataset{}, nil
	}
	if err != nil {
		return Dataset{}, err
	}
	return ParseDatasetCSV(body), nil
}

// datasetModel reads a test's own dataset from the JSON resource.
func (c *Client) datasetModel(ctx context.Context, testKey string) (Dataset, error) {
	id, err := c.issueID(ctx, testKey)
	if err != nil {
		return Dataset{}, err
	}
	q := url.Values{}
	q.Set("entityId", id)
	q.Set("type", datasetEntityTest)
	body, status, err := c.getBytesAnyType(ctx, datasetWritePath+"?"+q.Encode())
	if status == http.StatusNotFound {
		return Dataset{}, nil
	}
	if err != nil {
		return Dataset{}, err
	}
	var model struct {
		EntityDataset struct {
			Params []datasetColumn `json:"params"`
		} `json:"entityDataset"`
	}
	if err := json.Unmarshal(body, &model); err != nil {
		return Dataset{}, fmt.Errorf("decode dataset of %s: %w", testKey, err)
	}
	return datasetRows(model.EntityDataset.Params), nil
}

// datasetRows turns Xray's column-major model into the row-major table every
// caller above holds. A column shorter than the longest contributes an empty
// cell, so every row is as wide as the parameter list.
func datasetRows(cols []datasetColumn) Dataset {
	ds := Dataset{Parameters: make([]DatasetParameter, 0, len(cols))}
	height := 0
	for _, col := range cols {
		if len(col.Values) > height {
			height = len(col.Values)
		}
		ds.Parameters = append(ds.Parameters, DatasetParameter{
			Name:          col.Param.Name,
			Combinatorial: col.Param.Combinatorial,
			ID:            col.Param.ID,
			Type:          col.Param.Info.Type,
			Options:       col.Param.Info.Options,
			ListScope:     col.Param.Info.ListScope,
			ListID:        col.Param.Info.ListID,
		})
	}
	for r := 0; r < height; r++ {
		row := make([]string, len(cols))
		for i, col := range cols {
			if r < len(col.Values) {
				row[i] = col.Values[r].Value
			}
		}
		ds.Rows = append(ds.Rows, row)
	}
	return ds
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

// datasetParamInfo carries the parameter's type, and for a list parameter the
// allowed values and the list they come from. All four are read and written:
// a write that drops them rebuilds the list, which both gives it a new id and,
// without the type, turns it into free text (#283).
type datasetParamInfo struct {
	Type      string   `json:"type"`
	Options   []string `json:"options"`
	ListScope string   `json:"paramTestListScope,omitempty"`
	ListID    int      `json:"listId,omitempty"`
}

type datasetValue struct {
	Value string `json:"value"`
	Rank  int    `json:"rank"`
}

// SetDataset replaces the dataset on a test with d, creating one when the test
// has none and clearing it when d is empty (#277 W1).
//
// A parameter carrying an id is sent back under it, so Xray updates that
// parameter rather than replacing it: the id survives and a rename stays a
// rename. A parameter with no id is announced as new. Whatever the read found
// about a list parameter goes back untouched, or the list is rebuilt as free
// text (#283).
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
	q.Set("type", datasetEntityTest)
	return c.post(ctx, datasetWritePath+"?"+q.Encode(), datasetPayload{
		Params: datasetColumns(d),
	})
}

// datasetEntityTest is the only entity type a test's own dataset is read or
// written under. Xray also accepts TEST_RUN, addressed by run id, which #277
// leaves to W3; everything else answers "Entity Type is not valid."
const datasetEntityTest = "TEST"

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
				ID:            parameterID(p),
				Name:          p.Name,
				Rank:          i,
				Combinatorial: p.Combinatorial,
				Info:          parameterInfo(p),
			},
			Values: values,
		})
	}
	return cols
}

// parameterID is the id to send for a parameter: its own where Xray already
// has one, and the negative marker where it does not.
func parameterID(p DatasetParameter) int {
	if p.ID > 0 {
		return p.ID
	}
	return newParameterID
}

// parameterInfo hands back what the read found. A parameter the editor just
// added has none, and is written as text; nothing in XTM creates a list.
func parameterInfo(p DatasetParameter) datasetParamInfo {
	info := datasetParamInfo{
		Type:      p.Type,
		Options:   p.Options,
		ListScope: p.ListScope,
		ListID:    p.ListID,
	}
	if info.Type == "" {
		info.Type = datasetParamText
	}
	if info.Options == nil {
		info.Options = []string{}
	}
	return info
}

// datasetParamText is the type of an ordinary parameter, and the only type
// XTM writes for one it has just been given.
const datasetParamText = "TEXT"
