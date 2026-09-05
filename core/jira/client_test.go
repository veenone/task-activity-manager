package jira

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetSendsBearerAndDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization = %q, want Bearer tok", got)
		}
		if r.URL.Path != "/rest/api/2/myself" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"jdoe","displayName":"J. Doe","emailAddress":"j@x"}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL+"/", "tok", srv.Client())
	if c.BaseURL() != srv.URL {
		t.Fatalf("BaseURL = %q, want trailing slash trimmed", c.BaseURL())
	}
	u, err := c.Myself(context.Background())
	if err != nil {
		t.Fatalf("Myself: %v", err)
	}
	if u.Name != "jdoe" || u.DisplayName != "J. Doe" {
		t.Errorf("user = %+v", u)
	}
}

func TestGetTurnsNon200IntoHTTPErrorWithJiraMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errorMessages":["Field 'foo' does not exist"],"errors":{"jql":"bad"}}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	var out map[string]any
	err := c.Get(context.Background(), "/rest/api/2/search", &out)
	var he *HTTPError
	if !errors.As(err, &he) {
		t.Fatalf("want *HTTPError, got %T: %v", err, err)
	}
	if he.Code != 400 || he.Method != "GET" || he.Path != "/rest/api/2/search" {
		t.Errorf("HTTPError = %+v", he)
	}
	if he.Message != "Field 'foo' does not exist; jql: bad" {
		t.Errorf("Message = %q", he.Message)
	}
	if he.Error() != "jira: 400 Bad Request: Field 'foo' does not exist; jql: bad" {
		t.Errorf("Error() = %q", he.Error())
	}
}

func TestHTTPErrorFallsBackToMethodAndPath(t *testing.T) {
	e := &HTTPError{Method: "GET", Path: "/x", Code: 500, Status: "500 Internal Server Error"}
	if got := e.Error(); got != "jira: GET /x -> 500 Internal Server Error" {
		t.Errorf("Error() = %q", got)
	}
}

func TestWriteJSONReturningDecodesAndReportsFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("method %s content-type %s", r.Method, r.Header.Get("Content-Type"))
		}
		if r.URL.Path == "/ok" {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"key":"PLAT-1"}`))
			return
		}
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`nope`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	var out struct {
		Key string `json:"key"`
	}
	if err := c.WriteJSONReturning(context.Background(), http.MethodPost, "/ok", map[string]string{"a": "b"}, &out); err != nil {
		t.Fatalf("post ok: %v", err)
	}
	if out.Key != "PLAT-1" {
		t.Errorf("decoded key = %q", out.Key)
	}
	err := c.Post(context.Background(), "/bad", map[string]string{})
	if err == nil || err.Error() != "jira: POST /bad -> 409 Conflict: nope" {
		t.Errorf("post bad err = %v", err)
	}
}

func TestDeleteAcceptsAny2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	if err := c.Delete(context.Background(), "/rest/api/2/issue/PLAT-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestGetBytesStatusReturnsBodyAndStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/raw" {
			_, _ = w.Write([]byte(`[1,2]`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`not a test`))
	}))
	defer srv.Close()
	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	body, code, err := c.GetBytesStatus(context.Background(), "/raw")
	if err != nil || code != 200 || string(body) != "[1,2]" {
		t.Fatalf("raw: %q %d %v", body, code, err)
	}
	_, code, err = c.GetBytesStatus(context.Background(), "/bad")
	if code != 400 || err == nil || err.Error() != "jira: GET /bad -> 400 Bad Request: not a test" {
		t.Errorf("bad: %d %v", code, err)
	}
}

func TestNilHTTPClientUsesDefaultTimeout(t *testing.T) {
	c := NewClientWithHTTP("https://jira.example", "t", nil)
	if c.http == nil || c.http.Timeout == 0 {
		t.Fatalf("expected a default http client with a timeout, got %+v", c.http)
	}
}
