package zephyr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// measuredStatuses is the vocabulary the probed instance returned from both
// /util/testExecutionStatus and /util/teststepExecutionStatus. PASS WARNING is
// in it deliberately: it is not one of the five SmartBear documents, which is
// the whole reason this vocabulary is read rather than compiled in.
const measuredStatuses = `[
  {"id":-1,"name":"UNEXECUTED","description":"The test has not been executed yet.","color":"#A0A0A0"},
  {"id":1,"name":"PASS","description":"Test was executed and passed successfully.","color":"#75B000"},
  {"id":2,"name":"FAIL","description":"Test was executed and failed.","color":"#CC3300"},
  {"id":3,"name":"WIP","description":"Test execution is a work in progress.","color":"#F2B000"},
  {"id":4,"name":"BLOCKED","description":"The test execution of this test was blocked.","color":"#6693B0"},
  {"id":5,"name":"PASS WARNING","description":"Passed with a warning.","color":"#B3BAC5"}
]`

func statusServer(t *testing.T, exec, step string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case apiPath + "/util/testExecutionStatus":
			_, _ = w.Write([]byte(exec))
		case apiPath + "/util/teststepExecutionStatus":
			_, _ = w.Write([]byte(step))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestExecutionStatusesComeFromTheInstance is the constraint the design
// document calls the most important one in the bridge: status ids are
// configured per instance, so a status this instance defines and SmartBear
// does not must still resolve. A compiled-in map of the documented five would
// write the wrong numeric id silently.
func TestExecutionStatusesComeFromTheInstance(t *testing.T) {
	srv := statusServer(t, measuredStatuses, measuredStatuses)
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	v, err := c.ExecutionStatuses(context.Background())
	if err != nil {
		t.Fatalf("ExecutionStatuses: %v", err)
	}

	id, err := v.IDFor("PASS WARNING")
	if err != nil {
		t.Fatalf("IDFor(PASS WARNING): %v", err)
	}
	if id != 5 {
		t.Errorf("IDFor(PASS WARNING) = %d, want 5 as this instance defines it", id)
	}
	if got := v.NameFor(-1); got != "UNEXECUTED" {
		t.Errorf("NameFor(-1) = %q, want UNEXECUTED", got)
	}
	if got := v.NameFor(2); got != "FAIL" {
		t.Errorf("NameFor(2) = %q, want FAIL", got)
	}
}

// TestIDForIsCaseInsensitive exists because the status a user picks in XTM is
// a display string and the instance's own casing is not guaranteed to match
// it.
func TestIDForIsCaseInsensitive(t *testing.T) {
	srv := statusServer(t, measuredStatuses, measuredStatuses)
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	v, err := c.ExecutionStatuses(context.Background())
	if err != nil {
		t.Fatalf("ExecutionStatuses: %v", err)
	}
	id, err := v.IDFor("pass warning")
	if err != nil || id != 5 {
		t.Errorf("IDFor(pass warning) = %d, %v; want 5 with no error", id, err)
	}
}

// TestIDForRejectsANameTheInstanceDoesNotDefine keeps a status the instance
// never declared from being written as some default. Failing the write is the
// only honest answer: there is no id to send.
func TestIDForRejectsANameTheInstanceDoesNotDefine(t *testing.T) {
	srv := statusServer(t, measuredStatuses, measuredStatuses)
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	v, err := c.ExecutionStatuses(context.Background())
	if err != nil {
		t.Fatalf("ExecutionStatuses: %v", err)
	}
	if _, err := v.IDFor("PASSED"); err == nil {
		t.Fatal("IDFor on an undefined status name returned no error")
	} else if !strings.Contains(err.Error(), "PASSED") {
		t.Errorf("error does not name the status asked for: %v", err)
	}
}

// TestStepStatusesAreReadFromTheirOwnEndpoint guards against reading one
// vocabulary and using it for both. The two endpoints answered the same thing
// on the probed instance, which is exactly the coincidence that would hide a
// single-endpoint read.
func TestStepStatusesAreReadFromTheirOwnEndpoint(t *testing.T) {
	stepOnly := `[{"id":1,"name":"PASS"},{"id":7,"name":"NOT APPLICABLE"}]`
	srv := statusServer(t, measuredStatuses, stepOnly)
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	v, err := c.StepStatuses(context.Background())
	if err != nil {
		t.Fatalf("StepStatuses: %v", err)
	}
	if id, err := v.IDFor("NOT APPLICABLE"); err != nil || id != 7 {
		t.Errorf("IDFor(NOT APPLICABLE) = %d, %v; want 7 from the step endpoint", id, err)
	}
	if _, err := v.IDFor("PASS WARNING"); err == nil {
		t.Error("the step vocabulary resolved a name only the execution endpoint defines")
	}
}

// TestAnEmptyVocabularyIsAnError is here because the status endpoints'
// response envelope is the one shape in Phase 1 taken from SmartBear's
// documentation rather than measured. If an instance answers some other
// shape, this fails the connection test with a readable message instead of
// leaving every status lookup to fail later one at a time.
func TestAnEmptyVocabularyIsAnError(t *testing.T) {
	srv := statusServer(t, `[]`, measuredStatuses)
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	if _, err := c.ExecutionStatuses(context.Background()); err == nil {
		t.Fatal("an empty status vocabulary was accepted")
	}
}

// TestTheVocabularyIsReadOncePerClient keeps every status lookup from costing
// a round trip. The cache is per client, which is per operation here, so this
// asserts the cheap guarantee rather than a profile-wide one.
func TestTheVocabularyIsReadOncePerClient(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(measuredStatuses))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	for i := 0; i < 3; i++ {
		if _, err := c.ExecutionStatuses(context.Background()); err != nil {
			t.Fatalf("ExecutionStatuses: %v", err)
		}
	}
	if calls != 1 {
		t.Errorf("the execution status endpoint was read %d times, want 1", calls)
	}
}
