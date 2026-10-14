package jira

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// xrayFields is /rest/api/2/field for an instance with the Xray fields a
// precondition and a test create carry, each identified by its plugin key.
var xrayFields = []map[string]any{
	{"id": "customfield_13988", "name": "Pre-Condition Type", "custom": true,
		"schema": map[string]any{"custom": xrayPreconditionTypeType}},
	{"id": "customfield_13989", "name": "Conditions", "custom": true,
		"schema": map[string]any{"custom": xrayPreconditionEditorType}},
	{"id": "customfield_10100", "name": "Test Type", "custom": true,
		"schema": map[string]any{"custom": xrayTestTypeType}},
}

// createmeta answers a createmeta request with every field in fields marked
// required, and records the query it was asked.
func createmeta(t *testing.T, gotQuery *string, fields map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/issuetype":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "77", "name": "Pre-Condition"},
				{"id": "5", "name": "Test"},
			})
		case "/rest/api/2/field":
			_ = json.NewEncoder(w).Encode(xrayFields)
		case "/rest/api/2/issue/createmeta":
			*gotQuery = r.URL.RawQuery
			_ = json.NewEncoder(w).Encode(map[string]any{
				"projects": []any{map[string]any{
					"issuetypes": []any{map[string]any{"fields": fields}},
				}},
			})
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func required(name string) map[string]any {
	return map[string]any{"name": name, "required": true, "schema": map[string]any{"type": "string"}}
}

func ids(defs []FieldDef) []string {
	out := []string{}
	for _, d := range defs {
		out = append(out, d.ID)
	}
	sort.Strings(out)
	return out
}

func TestPreconditionCreateFieldsAskOnlyForWhatTheDialogLacks(t *testing.T) {
	var q string
	srv := httptest.NewServer(createmeta(t, &q, map[string]any{
		"project":           required("Project"),
		"issuetype":         required("Issue Type"),
		"summary":           required("Summary"),
		"description":       required("Description"),
		"customfield_13988": required("Pre-Condition Type"),
		"customfield_13989": required("Conditions"),
		"customfield_20000": required("Team"),
	}))
	defer srv.Close()

	defs, err := newTestClient(srv).GetPreconditionCreateFields(context.Background(), "QA")
	if err != nil {
		t.Fatalf("fields: %v", err)
	}
	if got := ids(defs); len(got) != 1 || got[0] != "customfield_20000" {
		t.Fatalf("fields %v, want only the Team field", got)
	}
	if q != "projectKeys=QA&issuetypeIds=77&expand=projects.issuetypes.fields" {
		t.Fatalf("createmeta query %q", q)
	}
}

func TestTestCreateFieldsAskOnlyForWhatTheDialogLacks(t *testing.T) {
	var q string
	srv := httptest.NewServer(createmeta(t, &q, map[string]any{
		"summary":           required("Summary"),
		"priority":          required("Priority"),
		"labels":            required("Labels"),
		"components":        required("Component/s"),
		"customfield_10100": required("Test Type"),
		"customfield_20000": required("Team"),
	}))
	defer srv.Close()

	defs, err := newTestClient(srv).GetTestCreateFields(context.Background(), "QA")
	if err != nil {
		t.Fatalf("fields: %v", err)
	}
	if got := ids(defs); len(got) != 1 || got[0] != "customfield_20000" {
		t.Fatalf("fields %v, want only the Team field", got)
	}
	if q != "projectKeys=QA&issuetypeIds=5&expand=projects.issuetypes.fields" {
		t.Fatalf("createmeta query %q", q)
	}
}

// posted captures the fields of the create POST.
func posted(t *testing.T, key string, got *map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/issuetype":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "77", "name": "Pre-Condition"},
				{"id": "5", "name": "Test"},
			})
		case "/rest/api/2/field":
			_ = json.NewEncoder(w).Encode(xrayFields)
		case "/rest/api/2/issue":
			body, _ := io.ReadAll(r.Body)
			var payload struct {
				Fields map[string]any `json:"fields"`
			}
			_ = json.Unmarshal(body, &payload)
			*got = payload.Fields
			_ = json.NewEncoder(w).Encode(map[string]string{"key": key})
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestCreatePreconditionSendsItsTypeAndExtraFields(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(posted(t, "PC-9", &got))
	defer srv.Close()

	_, err := newTestClient(srv).CreatePrecondition(context.Background(), "QA", "Net up", "Generic", "",
		map[string]any{"customfield_20000": map[string]string{"id": "3"}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	ptype, _ := got["customfield_13988"].(map[string]any)
	if ptype["value"] != "Generic" {
		t.Fatalf("Pre-Condition Type = %v, want Generic", got["customfield_13988"])
	}
	team, _ := got["customfield_20000"].(map[string]any)
	if team["id"] != "3" {
		t.Fatalf("extra field = %v", got["customfield_20000"])
	}
}

func TestCreateTestSendsExtraFields(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(posted(t, "QA-1", &got))
	defer srv.Close()

	_, err := newTestClient(srv).CreateTest(context.Background(), "QA", "Login", "", "", nil, nil,
		map[string]any{"customfield_20000": "Payments"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got["customfield_20000"] != "Payments" || got["summary"] != "Login" {
		t.Fatalf("fields %v", got)
	}
}

// A field offered on two issue types comes back from createmeta once per type.
// Without a guard the dialog renders it twice and the second copy overwrites
// the first value the user typed.
func TestCreateScreenFieldsReturnsAFieldOfferedTwiceOnlyOnce(t *testing.T) {
	team := required("Team")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/issue/createmeta" {
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"projects": []any{map[string]any{"issuetypes": []any{
				map[string]any{"fields": map[string]any{"customfield_20000": team}},
				map[string]any{"fields": map[string]any{"customfield_20000": team}},
			}}},
		})
	}))
	defer srv.Close()

	got, err := newTestClient(srv).GetCreateScreenFields(
		context.Background(), "QA", "", "Test", nil)
	if err != nil {
		t.Fatalf("fields: %v", err)
	}
	if want := []string{"customfield_20000"}; !reflect.DeepEqual(ids(got), want) {
		t.Fatalf("ids = %v, want %v", ids(got), want)
	}
}

// An instance whose field catalogue cannot be read must still create the
// precondition. Losing the type is worth a log line; losing the ability to
// create preconditions at all is not.
func TestCreatePreconditionSurvivesAnUnreadableFieldCatalogue(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/issuetype":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "77", "name": "Pre-Condition"}})
		case "/rest/api/2/field":
			w.WriteHeader(http.StatusInternalServerError)
		case "/rest/api/2/issue":
			body, _ := io.ReadAll(r.Body)
			var payload struct {
				Fields map[string]any `json:"fields"`
			}
			_ = json.Unmarshal(body, &payload)
			got = payload.Fields
			_ = json.NewEncoder(w).Encode(map[string]string{"key": "PC-9"})
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	key, err := newTestClient(srv).CreatePrecondition(
		context.Background(), "QA", "Net up", "Generic", "", nil)
	if err != nil {
		t.Fatalf("create must survive an unreadable field catalogue, got %v", err)
	}
	if key != "PC-9" {
		t.Fatalf("key = %q, want PC-9", key)
	}
	for k := range got {
		if strings.HasPrefix(k, "customfield_") {
			t.Fatalf("no custom field should be sent when the catalogue is unreadable, got %s", k)
		}
	}
}
