package zephyr

import (
	"context"
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
