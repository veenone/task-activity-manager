package jira

import (
	"encoding/json"
	"strings"
	"testing"
)

// The write path for datasets is not built yet (#277 W1). These tests pin the
// two things about Xray's write protocol that a client gets wrong by writing
// the obvious code, both captured from a live instance during the W0 spike.

// A successful write answers Content-Type: application/json with a body that
// is not JSON. A client that unmarshals the response of its own write, which
// is the natural thing to write, fails on success and only on success.
func TestDatasetWriteSuccessIsNotJSONDespiteItsContentType(t *testing.T) {
	body := readFixture(t, "dataset_write_success.txt")

	if json.Valid(body) {
		t.Fatal("the success body parses as JSON now; the client may unmarshal it")
	}
	if got := strings.TrimSpace(string(body)); got != "Dataset Updated with success." {
		t.Errorf("success body = %q, want the plain sentence Xray sends", got)
	}

	// The failure shape is JSON, so a client cannot simply always treat the
	// body as text either. Status is what separates them.
	var fail struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(readFixture(t, "dataset_write_unassociated_param.json"), &fail); err != nil {
		t.Fatalf("the failure body is no longer JSON: %v", err)
	}
	if fail.Error == "" {
		t.Error("failure body carries no error message")
	}
}

// Xray refuses a write that names a parameter it does not already own. A new
// parameter is asked for with a negative id; omitting the id, or sending 0,
// is refused with this message. Nothing documents that, and a client that
// leaves the id off writes nothing and sees 400.
func TestDatasetWriteRefusesAParameterWithNoServerID(t *testing.T) {
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(readFixture(t, "dataset_write_unassociated_param.json"), &body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body.Error, "not associated with the dataset") {
		t.Errorf("error = %q, want the unassociated-parameter refusal", body.Error)
	}
}

// Multipart writes to raven/1.0 are refused without Atlassian's XSRF opt-out
// header, with a plain-text 403 that is not JSON. The JSON write on the same
// prefix does not need the header, so this is a property of the content type
// rather than of the API.
func TestMultipartDatasetWriteNeedsTheXSRFOptOut(t *testing.T) {
	body := readFixture(t, "dataset_write_xsrf_refused.txt")

	if got := strings.TrimSpace(string(body)); got != "XSRF check failed" {
		t.Errorf("refusal body = %q, want Atlassian's XSRF message", got)
	}
	if json.Valid(body) {
		t.Error("the refusal is JSON now; a client cannot read it as text")
	}
}
