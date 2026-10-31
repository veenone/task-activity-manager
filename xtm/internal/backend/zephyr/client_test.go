package zephyr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestGetJSONRejectsA200CarryingAnErrorId is the first test in this package
// because it guards the trap Zephyr's own Data Center documentation names: an
// invalid authentication can arrive as 200 OK with an errorId in the body. A
// client that trusts the status code reads a failed auth as success, and the
// user is then told the project has no tests rather than that the token is
// wrong (I1, #371 Phase 1).
func TestGetJSONRejectsA200CarryingAnErrorId(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"errorId":"ERROR","errorDesc":"You do not have the permission to make this request."}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	var out map[string]any
	err := c.getJSON(context.Background(), "/util/zephyrTestIssueType", &out)
	if err == nil {
		t.Fatal("getJSON on a 200 carrying an errorId returned no error")
	}
	if !strings.Contains(err.Error(), "do not have the permission") {
		t.Errorf("error does not carry Zephyr's own words: %v", err)
	}
}

// TestGetJSONDecodesAPlainObject is the control: the errorId check must not
// reject an ordinary successful response.
func TestGetJSONDecodesAPlainObject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != apiPath+"/util/zephyrTestIssueType" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization = %q, want the Jira personal access token as a bearer", got)
		}
		_, _ = w.Write([]byte(`{"testcaseIssueTypeId":"9"}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	var out struct {
		ID string `json:"testcaseIssueTypeId"`
	}
	if err := c.getJSON(context.Background(), "/util/zephyrTestIssueType", &out); err != nil {
		t.Fatalf("getJSON: %v", err)
	}
	if out.ID != "9" {
		t.Errorf("testcaseIssueTypeId = %q, want 9", out.ID)
	}
}

// TestGetJSONDecodesABareArray exists because several Zephyr endpoints answer
// with a bare JSON array (/stepResult is one). The errorId check looks for an
// object, so it must leave an array alone rather than refusing to decode it.
func TestGetJSONDecodesABareArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":4001},{"id":4002}]`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	var out []struct {
		ID int `json:"id"`
	}
	if err := c.getJSON(context.Background(), "/stepResult?executionId=2001", &out); err != nil {
		t.Fatalf("getJSON: %v", err)
	}
	if len(out) != 2 || out[0].ID != 4001 {
		t.Errorf("decoded %+v, want the two step results", out)
	}
}

// TestTestIssueTypeIDAcceptsANumericId guards the one field shape that was
// measured as a quoted string. Reading it as a plain number would be a decode
// failure on an instance that answers unquoted, and the id is only ever used
// as a string in JQL.
func TestTestIssueTypeIDAcceptsANumericId(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"testcaseIssueTypeId":9}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	id, err := c.TestIssueTypeID(context.Background())
	if err != nil {
		t.Fatalf("TestIssueTypeID: %v", err)
	}
	if id != "9" {
		t.Errorf("TestIssueTypeID = %q, want 9", id)
	}
}
