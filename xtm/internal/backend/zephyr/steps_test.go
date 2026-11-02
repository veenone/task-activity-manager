package zephyr

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// measuredSteps is the shape the probed instance answered, abridged to the
// fields that matter plus the HTML ones that must be ignored.
const measuredSteps = `{"stepBeanCollection":[
  {"id":3001,"orderId":1,"step":"Sign in","data":"a valid account","result":"the dashboard opens",
   "htmlStep":"<p>Sign in</p>","htmlData":"<p>a valid account</p>","customFieldValuesMap":{},"totalStepCount":2},
  {"id":3002,"orderId":2,"step":"Sign out","data":"","result":"the login page returns"}]}`

// stepServer answers the issue id lookup and the steps endpoint, and records
// the step path so a test can prove which identifier was sent.
func stepServer(t *testing.T, steps string) (*httptest.Server, *string) {
	t.Helper()
	var stepPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/2/issue/PROJ-101":
			_, _ = w.Write([]byte(`{"id":"10501","key":"PROJ-101","fields":{}}`))
		case r.URL.Path == apiPath+"/teststep/10501":
			stepPath = r.URL.Path
			_, _ = w.Write([]byte(steps))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return srv, &stepPath
}

// stepCreateServer answers the issue id lookup and one POST to the step
// endpoint with answer, recording every request so a test can prove what was
// sent and where.
func stepCreateServer(t *testing.T, answer string) (*httptest.Server, *[]recorded) {
	t.Helper()
	var seen []recorded
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen = append(seen, recorded{r.Method, r.URL.Path, r.URL.RawQuery, string(body)})
		switch {
		case r.URL.Path == "/rest/api/2/issue/PROJ-101":
			_, _ = w.Write([]byte(`{"id":"10501","key":"PROJ-101","fields":{}}`))
		case r.URL.Path == apiPath+"/teststep/10501":
			_, _ = w.Write([]byte(answer))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return srv, &seen
}

// A step create has to clear the same two adaptations the read does: the
// endpoint is keyed by the Jira issue id rather than its key, and Zephyr names
// the three fields step, data and result. Sending XTM's own names would create
// an empty step, and sending the key answers 400 (ids.go).
func TestCreateTestStepSendsZephyrsNamesToTheIssueID(t *testing.T) {
	srv, seen := stepCreateServer(t, `{"id":3003,"orderId":3,"step":"Sign in again"}`)
	defer srv.Close()
	a := newTestAdapter(srv)

	id, err := a.CreateTestStep(context.Background(), "PROJ-101", "Sign in again", "a valid account", "the dashboard opens")
	if err != nil {
		t.Fatalf("CreateTestStep: %v", err)
	}
	if id != "3003" {
		t.Errorf("new step id = %q, want 3003 from the answer", id)
	}

	req, ok := findRequest(*seen, "POST", apiPath+"/teststep/10501")
	if !ok {
		t.Fatalf("no POST to the step endpoint keyed by the issue id: %+v", *seen)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("request body is not JSON: %v (%s)", err, req.Body)
	}
	for field, want := range map[string]string{
		"step":   "Sign in again",
		"data":   "a valid account",
		"result": "the dashboard opens",
	} {
		if got, _ := body[field].(string); got != want {
			t.Errorf("body[%q] = %q, want %q", field, got, want)
		}
	}
	// XTM's own names would be silently ignored by the instance, so a step
	// would be created with no text at all.
	for _, absent := range []string{"action", "expected"} {
		if _, ok := body[absent]; ok {
			t.Errorf("body carries XTM's field name %q rather than Zephyr's", absent)
		}
	}
}

// The answer's shape is not measured, only its path and its field names, so a
// create that lands without a readable id must still count as a success.
//
// backend.Backend's contract already allows it: syncer's createStep is
// documented as returning the id "when Jira reports one" and its caller guards
// on a non-empty string, reconciling the placeholder on the next refresh.
// Failing instead would be worse than useless here, because publishSteps stops
// at the first error, so one unreadable answer would truncate the rest of a
// published test's steps even though every write had landed.
//
// This is the opposite choice from CreateContainer, which does fail without an
// id: a caller keys journalled work by a cycle id, and nothing keys anything by
// a step id.
func TestCreateTestStepWithNoReadableIDStillSucceeds(t *testing.T) {
	for _, answer := range []string{`{}`, `{"stepBeanCollection":[{"id":3003}]}`, ``} {
		srv, _ := stepCreateServer(t, answer)
		a := newTestAdapter(srv)

		id, err := a.CreateTestStep(context.Background(), "PROJ-101", "Sign in", "", "")
		srv.Close()
		if err != nil {
			t.Errorf("answer %q: CreateTestStep failed although the write landed: %v", answer, err)
		}
		if id != "" {
			t.Errorf("answer %q: id = %q, want empty rather than a guess", answer, id)
		}
	}
}

// Zephyr answers an auth failure with 200 OK and an errorId in the body, so a
// status check alone reads a refusal as a success and the commit path clears a
// change that never landed (I1). writeJSON holds this; the test pins it for
// this method, which is the one a bridge publish calls per step.
func TestCreateTestStepReportsAnErrorIDAnsweredWith200(t *testing.T) {
	srv, _ := stepCreateServer(t, `{"errorId":"zephyr.auth.failed","errorDesc":"not authorised"}`)
	defer srv.Close()
	a := newTestAdapter(srv)

	if _, err := a.CreateTestStep(context.Background(), "PROJ-101", "Sign in", "", ""); err == nil {
		t.Fatal("a 200 answer carrying an errorId was read as a successful create")
	}
}

// TestGetTestStepsReadsThePlainFieldsFromTheCollection covers both shape traps
// in one go: the array is wrapped in stepBeanCollection, and every field
// arrives twice, once as text and once as HTML. Reading the HTML copy puts
// markup in the editor and in every diff a commit produces.
func TestGetTestStepsReadsThePlainFieldsFromTheCollection(t *testing.T) {
	srv, stepPath := stepServer(t, measuredSteps)
	defer srv.Close()
	a := newTestAdapter(srv)

	steps, err := a.GetTestSteps(context.Background(), "PROJ-101")
	if err != nil {
		t.Fatalf("GetTestSteps: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("read %d steps, want the 2 in the collection", len(steps))
	}
	if steps[0].Action != "Sign in" {
		t.Errorf("action = %q, want the plain step text rather than its HTML copy", steps[0].Action)
	}
	if steps[0].Data != "a valid account" || steps[0].Expected != "the dashboard opens" {
		t.Errorf("data/expected = %q/%q, want Zephyr's data and result fields", steps[0].Data, steps[0].Expected)
	}
	if steps[0].ID != "3001" {
		t.Errorf("step id = %q, want the numeric id as a string", steps[0].ID)
	}
	if steps[0].Index != 1 || steps[1].Index != 2 {
		t.Errorf("indexes = %d, %d, want Zephyr's 1-based orderId", steps[0].Index, steps[1].Index)
	}
	// The endpoint takes the Jira issue id. Sending the key answers 400, and
	// the key is what every caller holds, so the resolver is not optional.
	if *stepPath != apiPath+"/teststep/10501" {
		t.Errorf("steps were asked for at %q, want the issue id path", *stepPath)
	}
}

// TestGetTestStepsOnATestWithNoSteps is the ordinary empty case. It must not
// be an error: a test with no steps is normal, and reporting one would make
// the detail panel show a failure instead of an empty step list.
func TestGetTestStepsOnATestWithNoSteps(t *testing.T) {
	srv, _ := stepServer(t, `{"stepBeanCollection":[]}`)
	defer srv.Close()
	a := newTestAdapter(srv)

	steps, err := a.GetTestSteps(context.Background(), "PROJ-101")
	if err != nil {
		t.Fatalf("GetTestSteps on a test with no steps: %v", err)
	}
	if len(steps) != 0 {
		t.Errorf("read %d steps from an empty collection", len(steps))
	}
}

// TestGetTestStepsRejectsA200CarryingAnErrorID ties the step read to the
// client's body check. This is the endpoint a browsing user hits most, so a
// rejected token has to surface here as a credential error rather than as a
// test that appears to have lost its steps.
func TestGetTestStepsRejectsA200CarryingAnErrorID(t *testing.T) {
	srv, _ := stepServer(t, `{"errorId":"ERROR","errorDesc":"Authentication failed."}`)
	defer srv.Close()
	a := newTestAdapter(srv)

	if _, err := a.GetTestSteps(context.Background(), "PROJ-101"); err == nil {
		t.Fatal("a 200 carrying an errorId read as a test with no steps")
	}
}
