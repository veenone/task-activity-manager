package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// editMetaSixFields is what a real Data Center answered for every issue type
// of one project, story and sub-task alike: six fields, and neither Story
// Points (customfield_10253) nor the Epic Link among them, although Story
// Points reads fine on the issue and exists in the global field list.
const editMetaSixFields = `{"fields":{
	"summary":{"required":true,"name":"Summary","operations":["set"],"schema":{"type":"string","system":"summary"}},
	"priority":{"required":false,"name":"Priority","operations":["set"],"schema":{"type":"priority","system":"priority"},"allowedValues":[{"id":"3","name":"Medium"}]},
	"reporter":{"required":true,"name":"Reporter","operations":["set"],"schema":{"type":"user","system":"reporter"}},
	"description":{"required":false,"name":"Description","operations":["set"],"schema":{"type":"string","system":"description"}},
	"labels":{"required":false,"name":"Labels","operations":["add","set","remove"],"schema":{"type":"array","items":"string","system":"labels"}},
	"assignee":{"required":false,"name":"Assignee","operations":["set"],"schema":{"type":"user","system":"assignee"}}
}}`

func TestEditMetaReadsTheEditScreenByFieldID(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(editMetaSixFields))
	}))
	defer srv.Close()

	meta, err := NewClientWithHTTP(srv.URL, "tok", srv.Client()).EditMeta(context.Background(), "RND-4")
	if err != nil {
		t.Fatalf("EditMeta: %v", err)
	}
	if path != "/rest/api/2/issue/RND-4/editmeta" {
		t.Errorf("path = %s", path)
	}
	if len(meta) != 6 {
		t.Fatalf("fields = %+v", meta)
	}
	if _, ok := meta.Field("customfield_10253"); ok {
		t.Error("a field the edit screen does not carry must not be reported as editable")
	}
	desc, ok := meta.Field("description")
	if !ok || desc.Name != "Description" || desc.Kind() != KindTextarea {
		t.Errorf("description = %+v (ok %v)", desc, ok)
	}
	prio, ok := meta.Field("priority")
	if !ok || prio.Kind() != KindOption || len(prio.AllowedValues) != 1 || prio.AllowedValues[0].Label() != "Medium" {
		t.Errorf("priority = %+v (ok %v)", prio, ok)
	}
	// The map key is the only place the id lives in this payload, so a
	// reader that forgets to copy it leaves every field unnamed.
	for _, f := range meta {
		if f.ID == "" {
			t.Fatalf("field %q came back with no id", f.Name)
		}
	}
}

func TestEditMetaReportsAFailedRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errorMessages":["no permission"]}`))
	}))
	defer srv.Close()

	if _, err := NewClientWithHTTP(srv.URL, "tok", srv.Client()).EditMeta(context.Background(), "RND-4"); err == nil {
		t.Fatal("a 403 must be an error, not an empty edit screen: an empty one would refuse every field")
	}
}
