package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// perTypeStory is Data Center 8.4's per-type answer for a Story, in two
// pages, and it does not list customfield_10253: a field absent from this
// answer is not on the create screen.
const perTypeStoryPage1 = `{"startAt":0,"maxResults":2,"total":3,"isLast":false,"values":[
	{"fieldId":"summary","name":"Summary","required":true,"schema":{"type":"string","system":"summary"},"operations":["set"]},
	{"fieldId":"customfield_10050","name":"Severity","required":true,"schema":{"type":"option","custom":"com.atlassian.jira.plugin.system.customfieldtypes:select"},"allowedValues":[{"id":"1","value":"Minor"},{"id":"3","value":"Critical"}]}
]}`

const perTypeStoryPage2 = `{"startAt":2,"maxResults":2,"total":3,"isLast":true,"values":[
	{"fieldId":"customfield_10300","name":"Acceptance criteria","required":false,"schema":{"type":"string","custom":"com.atlassian.jira.plugin.system.customfieldtypes:textarea"}}
]}`

const classicStory = `{"projects":[{"key":"TKT","issuetypes":[{"id":"10001","name":"Story","fields":{
	"summary":{"required":true,"name":"Summary","schema":{"type":"string","system":"summary"}},
	"customfield_10253":{"required":false,"name":"Team","schema":{"type":"string"}}
}}]}]}`

func TestCreateMetaReadsEveryPageOfThePerTypeEndpoint(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path+"?"+r.URL.RawQuery)
		if r.URL.Path != "/rest/api/2/issue/createmeta/TKT/issuetypes/10001" {
			t.Errorf("path = %s", r.URL.Path)
		}
		switch r.URL.Query().Get("startAt") {
		case "0":
			_, _ = w.Write([]byte(perTypeStoryPage1))
		case "2":
			_, _ = w.Write([]byte(perTypeStoryPage2))
		default:
			t.Errorf("startAt = %q", r.URL.Query().Get("startAt"))
		}
	}))
	defer srv.Close()

	meta, err := NewClientWithHTTP(srv.URL, "tok", srv.Client()).CreateMeta(context.Background(), "TKT", "10001", "Story")
	if err != nil {
		t.Fatalf("CreateMeta: %v", err)
	}
	if meta.Source != MetaPerType || len(meta.Fields) != 3 || len(paths) != 2 {
		t.Fatalf("meta = %+v after %v", meta, paths)
	}
	if _, ok := meta.Field("customfield_10253"); ok {
		t.Error("a field the per-type answer does not list is not on the screen")
	}
	sev, ok := meta.Field("customfield_10050")
	if !ok || !sev.Required || sev.Kind() != KindOption || sev.AllowedValues[1].Label() != "Critical" {
		t.Errorf("severity = %+v", sev)
	}
	if ac, _ := meta.Field("customfield_10300"); ac.Kind() != KindTextarea || ac.Required {
		t.Errorf("acceptance criteria = %+v", ac)
	}
}

func TestCreateMetaFallsBackToTheClassicCallOnlyOnA404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/createmeta/") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<html>Not Found</html>`))
			return
		}
		q := r.URL.Query()
		if r.URL.Path != "/rest/api/2/issue/createmeta" || q.Get("projectKeys") != "TKT" ||
			q.Get("issuetypeIds") != "10001" || q.Get("expand") != "projects.issuetypes.fields" {
			t.Errorf("classic request = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(classicStory))
	}))
	defer srv.Close()

	meta, err := NewClientWithHTTP(srv.URL, "tok", srv.Client()).CreateMeta(context.Background(), "TKT", "10001", "Story")
	if err != nil {
		t.Fatalf("CreateMeta: %v", err)
	}
	if meta.Source != MetaClassic {
		t.Errorf("source = %q", meta.Source)
	}
	team, ok := meta.Field("customfield_10253")
	if !ok || team.ID != "customfield_10253" || team.Name != "Team" {
		t.Errorf("the classic answer carries its ids as map keys: %+v", meta.Fields)
	}
}

func TestCreateMetaAsksTheClassicCallByNameWhenTheTypeHasNoId(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/rest/api/2/issue/createmeta/") {
			t.Errorf("no type id, so no per-type request: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("issuetypeNames"); got != "Bug" {
			t.Errorf("issuetypeNames = %q", got)
		}
		_, _ = w.Write([]byte(`{"projects":[{"issuetypes":[{"name":"Bug","fields":{}}]}]}`))
	}))
	defer srv.Close()

	if _, err := NewClientWithHTTP(srv.URL, "tok", srv.Client()).CreateMeta(context.Background(), "PLAT", "", "Bug"); err != nil {
		t.Fatalf("CreateMeta: %v", err)
	}
}

func TestCreateMetaDoesNotFallBackOnAnythingButA404(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errorMessages":["no permission"]}`))
	}))
	defer srv.Close()

	_, err := NewClientWithHTTP(srv.URL, "tok", srv.Client()).CreateMeta(context.Background(), "TKT", "10001", "Story")
	if err == nil || calls != 1 {
		t.Fatalf("err = %v after %d calls, want the 403 and no classic retry", err, calls)
	}
}

func TestShapeValueForEachKind(t *testing.T) {
	opts := []MetaOption{{ID: "100", Name: "Checkout"}, {ID: "101", Name: "Payments"}}
	for _, tc := range []struct {
		name  string
		field MetaField
		in    string
		want  string
	}{
		{"option with values sends the id", MetaField{Schema: MetaSchema{Type: "option"}, AllowedValues: opts}, "100", `{"id":"100"}`},
		{"option without values sends the text", MetaField{Schema: MetaSchema{Type: "option"}}, "Needs docs", `{"value":"Needs docs"}`},
		{"array with values sends every id", MetaField{Schema: MetaSchema{Type: "array", Items: "component"}, AllowedValues: opts}, "100, 101", `[{"id":"100"},{"id":"101"}]`},
		{"array of strings is a plain list", MetaField{Schema: MetaSchema{Type: "array", Items: "string"}}, "alpha, beta", `["alpha","beta"]`},
		{"array of options without values sends values", MetaField{Schema: MetaSchema{Type: "array", Items: "option"}}, "a,b", `[{"value":"a"},{"value":"b"}]`},
		{"array of users sends names", MetaField{Schema: MetaSchema{Type: "array", Items: "user"}}, "jdoe, ranand", `[{"name":"jdoe"},{"name":"ranand"}]`},
		{"user sends a name", MetaField{Schema: MetaSchema{Type: "user"}}, " jdoe ", `{"name":"jdoe"}`},
		{"number is a number", MetaField{Schema: MetaSchema{Type: "number"}}, "2.5", `2.5`},
		{"a number that is not one goes as typed", MetaField{Schema: MetaSchema{Type: "number"}}, "soon", `"soon"`},
		{"date is the ISO day", MetaField{Schema: MetaSchema{Type: "date"}}, " 2026-09-16 ", `"2026-09-16"`},
		{"datetime is the day at midnight", MetaField{Schema: MetaSchema{Type: "datetime"}}, "2026-09-16", `"2026-09-16T00:00:00.000+0000"`},
		{"text is text", MetaField{Schema: MetaSchema{Type: "string"}}, "free text", `"free text"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(ShapeValue(tc.field, tc.in))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("ShapeValue = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestKindNamesLongTextAndTheFieldsNoFormCanFill(t *testing.T) {
	for _, tc := range []struct {
		schema MetaSchema
		want   string
	}{
		{MetaSchema{Type: "string", Custom: "com.atlassian.jira.plugin.system.customfieldtypes:textarea"}, KindTextarea},
		{MetaSchema{Type: "string", System: "environment"}, KindTextarea},
		{MetaSchema{Type: "string"}, KindString},
		{MetaSchema{Type: "version"}, KindOption},
		{MetaSchema{Type: "attachment"}, KindOther},
		{MetaSchema{Type: "array", Items: "issuelinks"}, KindOther},
		{MetaSchema{Type: "timetracking"}, KindOther},
	} {
		if got := (MetaField{Schema: tc.schema}).Kind(); got != tc.want {
			t.Errorf("Kind(%+v) = %q, want %q", tc.schema, got, tc.want)
		}
	}
	if got := SplitList(" a, ,b ,"); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("SplitList = %v", got)
	}
}
