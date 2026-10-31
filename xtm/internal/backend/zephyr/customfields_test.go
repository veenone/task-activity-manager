package zephyr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"agile-suite/xtm/internal/backend"
)

// The two Zephyr custom field namespaces. The entity endpoint was measured
// answering 200 on the probed instance, but its body was not captured, so the
// reader tolerates both shapes SmartBear's API uses elsewhere: a bare array
// and an array wrapped in an object. These fixtures cover one each.
const measuredExecutionFields = `[{"id":1,"name":"Build number","customFieldType":"TEXT"}]`

const measuredStepFields = `{"customFields":[{"id":"2","name":"Evidence","type":"LARGE_TEXT"}]}`

// customFieldServer answers the Jira half and both Zephyr namespaces. The
// Jira half serves createmeta as absent so the client takes its /field
// fallback, which is the path a locked-down instance takes anyway.
//
// entity lets a test replace the per-entityType response; a nil writer there
// means that namespace fails.
func customFieldServer(t *testing.T, entity map[string]func(http.ResponseWriter)) *httptest.Server {
	t.Helper()
	if entity == nil {
		entity = map[string]func(http.ResponseWriter){
			"EXECUTION": func(w http.ResponseWriter) { _, _ = w.Write([]byte(measuredExecutionFields)) },
			"TESTSTEP":  func(w http.ResponseWriter) { _, _ = w.Write([]byte(measuredStepFields)) },
		}
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/issue/createmeta":
			w.WriteHeader(http.StatusNotFound)
		case "/rest/api/2/field":
			_, _ = w.Write([]byte(`[{"id":"customfield_10102","name":"Automation","custom":true,"schema":{"type":"option"}},` +
				`{"id":"summary","name":"Summary","custom":false}]`))
		case apiPath + "/customfield/entity":
			h, ok := entity[r.URL.Query().Get("entityType")]
			if !ok || h == nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			h(w)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// scopeOf indexes definitions by id so a test can assert on one without
// depending on the order the namespaces are read in.
func scopeOf(defs []backend.CustomFieldDef) map[string]backend.CustomFieldDef {
	out := map[string]backend.CustomFieldDef{}
	for _, d := range defs {
		out[d.ID] = d
	}
	return out
}

// TestListCustomFieldsReadsBothNamespaces is the point of Phase 3. A Zephyr
// profile has Jira's fields on the test issue and Zephyr's own fields on an
// execution and on a step, and the three are not interchangeable: an edit to
// the first commits as a Jira issue update, while the other two have no write
// path at all. The scope is what carries that difference to every consumer.
func TestListCustomFieldsReadsBothNamespaces(t *testing.T) {
	srv := customFieldServer(t, nil)
	defer srv.Close()

	defs, err := newTestAdapter(srv).ListCustomFields(context.Background(), "PROJ")
	if err != nil {
		t.Fatalf("ListCustomFields: %v", err)
	}
	byID := scopeOf(defs)
	if len(byID) != 3 {
		t.Fatalf("read %d fields (%v), want the Jira field plus one per Zephyr namespace", len(byID), defs)
	}
	for _, want := range []backend.CustomFieldDef{
		{ID: "customfield_10102", Name: "Automation", Type: "option", Scope: backend.FieldScopeIssue},
		{ID: "1", Name: "Build number", Type: "TEXT", Scope: backend.FieldScopeExecution},
		{ID: "2", Name: "Evidence", Type: "LARGE_TEXT", Scope: backend.FieldScopeStep},
	} {
		if got := byID[want.ID]; got != want {
			t.Errorf("field %s = %+v, want %+v", want.ID, got, want)
		}
	}
}

// TestListCustomFieldsKeepsTheJiraFieldsWhenZephyrsNamespaceFails degrades
// rather than refusing, because this is the sync's custom field stage and that
// stage is a hard failure in the engine: losing the whole sync over a
// namespace whose response shape was never measured would be the wrong trade.
// The Jira fields are the ones a user can actually edit.
func TestListCustomFieldsKeepsTheJiraFieldsWhenZephyrsNamespaceFails(t *testing.T) {
	srv := customFieldServer(t, map[string]func(http.ResponseWriter){
		"EXECUTION": nil, // answers 500
		"TESTSTEP":  func(w http.ResponseWriter) { _, _ = w.Write([]byte(measuredStepFields)) },
	})
	defer srv.Close()

	defs, err := newTestAdapter(srv).ListCustomFields(context.Background(), "PROJ")
	if err != nil {
		t.Fatalf("ListCustomFields degraded into an error: %v", err)
	}
	byID := scopeOf(defs)
	if _, ok := byID["customfield_10102"]; !ok {
		t.Errorf("the Jira field was dropped when a Zephyr namespace failed: %v", defs)
	}
	if _, ok := byID["2"]; !ok {
		t.Errorf("the readable Zephyr namespace was dropped too: %v", defs)
	}
	if _, ok := byID["1"]; ok {
		t.Errorf("a field was reported from the namespace that failed: %v", defs)
	}
}

// TestListCustomFieldsRefusesWhenJiraRefuses is the other half of that split.
// The Jira fields are not optional: a sync that silently reported none would
// empty the detail panel's custom fields and stage no edit for any of them.
func TestListCustomFieldsRefusesWhenJiraRefuses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	if _, err := newTestAdapter(srv).ListCustomFields(context.Background(), "PROJ"); err == nil {
		t.Fatal("ListCustomFields reported success although Jira refused the field list")
	}
}

// TestListCustomFieldsAsksForEachNamespaceByName pins the request, because the
// entityType is the only thing that separates the two namespaces: one request
// without it would return whichever the instance defaults to, scoped wrongly.
func TestListCustomFieldsAsksForEachNamespaceByName(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/issue/createmeta":
			w.WriteHeader(http.StatusNotFound)
		case "/rest/api/2/field":
			_, _ = w.Write([]byte(`[]`))
		case apiPath + "/customfield/entity":
			asked = append(asked, r.URL.Query().Get("entityType"))
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	if _, err := newTestAdapter(srv).ListCustomFields(context.Background(), "PROJ"); err != nil {
		t.Fatalf("ListCustomFields: %v", err)
	}
	sort.Strings(asked)
	if strings.Join(asked, ",") != "EXECUTION,TESTSTEP" {
		t.Errorf("asked for %v, want one request per Zephyr entity type", asked)
	}
}

// TestListCustomFieldsSkipsADuplicateID guards the one thing the unmeasured
// body could do to the store: the definitions are keyed by (profile, field id)
// there, so two definitions sharing an id would overwrite each other and the
// surviving scope would depend on read order.
func TestListCustomFieldsSkipsADuplicateID(t *testing.T) {
	same := `[{"id":"7","name":"Same id","customFieldType":"TEXT"}]`
	srv := customFieldServer(t, map[string]func(http.ResponseWriter){
		"EXECUTION": func(w http.ResponseWriter) { _, _ = w.Write([]byte(same)) },
		"TESTSTEP":  func(w http.ResponseWriter) { _, _ = w.Write([]byte(same)) },
	})
	defer srv.Close()

	defs, err := newTestAdapter(srv).ListCustomFields(context.Background(), "PROJ")
	if err != nil {
		t.Fatalf("ListCustomFields: %v", err)
	}
	n := 0
	for _, d := range defs {
		if d.ID == "7" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("id 7 reported %d times, want once", n)
	}
}

// TestListCustomFieldsIgnoresANamelessField keeps a row the detail panel
// cannot label out of the list. An unnamed definition renders as an empty
// label beside an input, which reads as a broken panel.
func TestListCustomFieldsIgnoresANamelessField(t *testing.T) {
	srv := customFieldServer(t, map[string]func(http.ResponseWriter){
		"EXECUTION": func(w http.ResponseWriter) {
			_, _ = w.Write([]byte(`[{"id":"3","name":""},{"id":"","name":"No id"}]`))
		},
		"TESTSTEP": func(w http.ResponseWriter) { _, _ = w.Write([]byte(`[]`)) },
	})
	defer srv.Close()

	defs, err := newTestAdapter(srv).ListCustomFields(context.Background(), "PROJ")
	if err != nil {
		t.Fatalf("ListCustomFields: %v", err)
	}
	for _, d := range defs {
		if d.Scope != backend.FieldScopeIssue {
			t.Errorf("kept an unusable Zephyr definition %+v", d)
		}
	}
}
