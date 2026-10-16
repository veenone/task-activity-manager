package syncer_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"agile-suite/xtm/internal/store"
	"agile-suite/xtm/internal/testrepo"
)

// datasetInstance is a stub Xray: it resolves a key to an issue id, serves the
// raven/1.0 dataset model the commit reads before writing, and records the
// write. columns is the model's params array, empty for a test with no
// dataset.
type datasetInstance struct {
	columns []string
	read    int
	wrote   string
	writes  int
}

// textColumn and listColumn build one entry of the model's params array, in
// the shape the instance returns: a plain parameter and one whose values come
// from a list.
func textColumn(id int, name string, values ...string) string {
	return `{"param":{"id":` + strconv.Itoa(id) + `,"name":"` + name + `","rank":0,` +
		`"combinatorial":false,"info":{"type":"TEXT","options":[]}},"values":` +
		modelValues(values) + `}`
}

func listColumn(id int, name string, listID int, options []string, values ...string) string {
	quoted := make([]string, len(options))
	for i, o := range options {
		quoted[i] = `"` + o + `"`
	}
	return `{"param":{"id":` + strconv.Itoa(id) + `,"name":"` + name + `","rank":0,` +
		`"combinatorial":false,"info":{"type":"LIST","options":[` + strings.Join(quoted, ",") +
		`],"paramTestListScope":"PARAMETER_AD_HOC","listId":` + strconv.Itoa(listID) + `}},` +
		`"values":` + modelValues(values) + `}`
}

func modelValues(values []string) string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = `{"value":"` + v + `","rank":` + strconv.Itoa(i) + `}`
	}
	return "[" + strings.Join(out, ",") + "]"
}

func (d *datasetInstance) server(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"4242","key":"QA-1","fields":{"updated":""}}`))
		case r.URL.Path == "/rest/raven/1.0/dataset" && r.Method == http.MethodGet:
			d.read++
			w.Header().Set("Content-Type", "application/json;charset=UTF-8")
			_, _ = w.Write([]byte(`{"entityDataset":{"params":[` +
				strings.Join(d.columns, ",") + `]},"resolvedDataset":{"params":[]},` +
				`"resolutionMessage":"This is the resolved dataset, defined in the Test."}`))
		case r.URL.Path == "/rest/raven/1.0/dataset":
			d.writes++
			b, _ := io.ReadAll(r.Body)
			d.wrote = string(b)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("Dataset Updated with success."))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
}

func datasetRepo(t *testing.T) *testrepo.Repository {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return testrepo.NewRepository(st)
}

func pendingDatasets(t *testing.T, repo *testrepo.Repository) []testrepo.PendingChange {
	t.Helper()
	all, err := repo.ListPendingChanges("p1")
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	out := []testrepo.PendingChange{}
	for _, c := range all {
		if c.EntityType == "test_dataset" {
			out = append(out, c)
		}
	}
	return out
}
