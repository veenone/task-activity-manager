package zephyr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// stepVocabulary is deliberately NOT measuredStatuses. Zephyr keeps the step
// statuses on their own endpoint and the probed instance answered the same six
// on both, which is exactly the coincidence that would hide a step result
// resolved against the execution vocabulary. These names exist on neither
// Xray nor the execution endpoint, so a lookup against the wrong vocabulary
// fails instead of passing by accident.
const stepVocabulary = `[
  {"id":-1,"name":"NOT RUN"},
  {"id":1,"name":"STEP PASSED"},
  {"id":2,"name":"STEP FAILED"},
  {"id":3,"name":"STEP RUNNING"}
]`

// stepResultServer answers the reads a step-result call makes: traceability to
// find the execution, the step definitions for their text and order, the step
// results themselves, and both status vocabularies. It records every request.
func stepResultServer(t *testing.T, handler map[string]func(http.ResponseWriter, *http.Request)) (*httptest.Server, *[]recorded) {
	t.Helper()
	srv, seen := writeServer(t, mergeHandlers(map[string]func(http.ResponseWriter, *http.Request){
		apiPath + "/util/teststepExecutionStatus": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(stepVocabulary))
		},
		"/rest/api/2/issue/PROJ-101": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"id":"7001","key":"PROJ-101"}`))
		},
		apiPath + "/teststep/7001": func(w http.ResponseWriter, _ *http.Request) {
			// Out of orderId sequence on purpose: the rank has to come from
			// the definition, not from the order the results arrive in.
			_, _ = w.Write([]byte(`{"stepBeanCollection":[
			  {"id":3002,"orderId":2,"step":"Submit the form","data":"valid payload","result":"202 Accepted"},
			  {"id":3001,"orderId":1,"step":"Open the gateway","data":"","result":"the console loads"}
			]}`))
		},
		apiPath + "/stepResult": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`[
			  {"id":4002,"stepId":3002,"executionId":2001,"status":"1","comment":""},
			  {"id":4001,"stepId":3001,"executionId":2001,"status":"2","comment":"gateway returned no response"}
			]`))
		},
	}, handler))
	return srv, seen
}

func mergeHandlers(base, over map[string]func(http.ResponseWriter, *http.Request)) map[string]func(http.ResponseWriter, *http.Request) {
	merged := make(map[string]func(http.ResponseWriter, *http.Request), len(base)+len(over))
	for path, h := range base {
		merged[path] = h
	}
	for path, h := range over {
		merged[path] = h
	}
	return merged
}

// TestRunStepResultsCarryTheirOwnIDsAndTheStepText is the whole point of Phase
// 6: on Zephyr a step result is addressable on an ordinary execution, with no
// dataset and no iteration anywhere in it.
func TestRunStepResultsCarryTheirOwnIDsAndTheStepText(t *testing.T) {
	srv, seen := stepResultServer(t, nil)
	defer srv.Close()
	a := newTestAdapter(srv)

	steps, err := a.GetRunStepResults(context.Background(), "501", "PROJ-101")
	if err != nil {
		t.Fatalf("GetRunStepResults: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("got %d step results, want 2", len(steps))
	}

	// Ordered by the definition's orderId, so the step the results listed
	// second comes first.
	if steps[0].ID != "4001" || steps[0].Rank != 1 {
		t.Errorf("first step result is %s at rank %d, want 4001 at rank 1", steps[0].ID, steps[0].Rank)
	}
	if steps[0].Action != "Open the gateway" || steps[0].Expected != "the console loads" {
		t.Errorf("first step result carries action %q expected %q, want the definition's text",
			steps[0].Action, steps[0].Expected)
	}
	// Resolved through the STEP vocabulary. Against the execution one, status
	// 2 is FAIL and this assertion fails.
	if steps[0].Status != "STEP FAILED" {
		t.Errorf("status is %q, want STEP FAILED from this instance's step vocabulary", steps[0].Status)
	}
	if steps[0].Comment != "gateway returned no response" {
		t.Errorf("comment is %q, want the one the instance recorded", steps[0].Comment)
	}
	if steps[0].DefinitionID != "3001" {
		t.Errorf("definition id is %q, want 3001", steps[0].DefinitionID)
	}
	if steps[1].ID != "4002" || steps[1].Status != "STEP PASSED" || steps[1].Rank != 2 {
		t.Errorf("second step result is %+v, want 4002 STEP PASSED at rank 2", steps[1])
	}

	// The results are asked for by execution id, which traceability resolved
	// from the test and the cycle.
	req, ok := findRequest(*seen, "GET", apiPath+"/stepResult")
	if !ok {
		t.Fatal("no step results were requested")
	}
	if !strings.Contains(req.Query, "executionId=2001") {
		t.Errorf("step results were asked for with %q, want executionId=2001", req.Query)
	}
}

// TestRunStepResultsNameTheTestAndCycleWhenTheTestIsNotInIt covers the lookup
// failing rather than the write: a test with no execution in that cycle has no
// step results to read, and saying only "not found" would leave the user
// guessing which of the two is wrong.
func TestRunStepResultsNameTheTestAndCycleWhenTheTestIsNotInIt(t *testing.T) {
	srv, _ := stepResultServer(t, map[string]func(http.ResponseWriter, *http.Request){
		apiPath + "/traceability/executionsByTest": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"executions":[],"totalCount":0}`))
		},
	})
	defer srv.Close()
	a := newTestAdapter(srv)

	_, err := a.GetRunStepResults(context.Background(), "501", "PROJ-101")
	if err == nil {
		t.Fatal("reading the step results of a test that is not in the cycle succeeded")
	}
	if !strings.Contains(err.Error(), "PROJ-101") || !strings.Contains(err.Error(), "501") {
		t.Errorf("error is %q, want both the test and the cycle named", err)
	}
}

// TestRunStepResultsKeepAResultWhoseDefinitionIsGone: a step deleted from the
// test leaves its results behind on past executions. Dropping the row would
// hide a recorded result, so it is kept with no text and sorted last (I1).
func TestRunStepResultsKeepAResultWhoseDefinitionIsGone(t *testing.T) {
	srv, _ := stepResultServer(t, map[string]func(http.ResponseWriter, *http.Request){
		apiPath + "/teststep/7001": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"stepBeanCollection":[
			  {"id":3001,"orderId":1,"step":"Open the gateway","data":"","result":"the console loads"}
			]}`))
		},
	})
	defer srv.Close()
	a := newTestAdapter(srv)

	steps, err := a.GetRunStepResults(context.Background(), "501", "PROJ-101")
	if err != nil {
		t.Fatalf("GetRunStepResults: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("got %d step results, want both kept", len(steps))
	}
	if steps[1].ID != "4002" || steps[1].Action != "" {
		t.Errorf("the orphaned result is %+v, want 4002 last with no step text", steps[1])
	}
	if steps[1].Status != "STEP PASSED" {
		t.Errorf("the orphaned result lost its status %q", steps[1].Status)
	}
}

// TestSetRunStepStatusWritesTheStepVocabularysID is the write. The status
// arrives as a name and Zephyr takes an id, and the id has to come from the
// step vocabulary.
func TestSetRunStepStatusWritesTheStepVocabularysID(t *testing.T) {
	srv, seen := stepResultServer(t, map[string]func(http.ResponseWriter, *http.Request){
		apiPath + "/stepResult/4001": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"id":4001,"status":"2"}`))
		},
	})
	defer srv.Close()
	a := newTestAdapter(srv)

	if err := a.SetRunStepStatus(context.Background(), "501", "PROJ-101", "4001", "STEP FAILED"); err != nil {
		t.Fatalf("SetRunStepStatus: %v", err)
	}
	req, ok := findRequest(*seen, "PUT", apiPath+"/stepResult/4001")
	if !ok {
		t.Fatal("no step result was written")
	}
	// 2 in the step vocabulary, which is STEP FAILED. The execution
	// vocabulary also has a 2, so a body carrying the name instead of the id,
	// or the id from the wrong vocabulary, is what this catches.
	if !strings.Contains(req.Body, `"status":2`) {
		t.Errorf("wrote %q, want the step vocabulary's numeric id 2", req.Body)
	}
}

// TestSetRunStepStatusRefusesAStatusThisInstanceDoesNotDefine: there is no id
// to send, and defaulting would record a result the user did not pick.
func TestSetRunStepStatusRefusesAStatusThisInstanceDoesNotDefine(t *testing.T) {
	srv, seen := stepResultServer(t, nil)
	defer srv.Close()
	a := newTestAdapter(srv)

	// A real Xray step status, and one this instance has no id for.
	err := a.SetRunStepStatus(context.Background(), "501", "PROJ-101", "4001", "EXECUTING")
	if err == nil {
		t.Fatal("writing a status the instance does not define succeeded")
	}
	if !strings.Contains(err.Error(), "EXECUTING") {
		t.Errorf("error is %q, want the refused status named", err)
	}
	if _, ok := findRequest(*seen, "PUT", apiPath+"/stepResult/4001"); ok {
		t.Error("the step result was written anyway")
	}
}

// TestSetRunStepStatusReportsAnErrorEnvelopeCarriedBy200: Zephyr answers an
// invalid authentication with 200 OK and an errorId. A write that trusted the
// status code would clear the pending change for a result the instance never
// accepted (I1).
func TestSetRunStepStatusReportsAnErrorEnvelopeCarriedBy200(t *testing.T) {
	srv, _ := stepResultServer(t, map[string]func(http.ResponseWriter, *http.Request){
		apiPath + "/stepResult/4001": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"errorId":"zephyr.common.invalid.authentication"}`))
		},
	})
	defer srv.Close()
	a := newTestAdapter(srv)

	err := a.SetRunStepStatus(context.Background(), "501", "PROJ-101", "4001", "STEP FAILED")
	if err == nil {
		t.Fatal("a 200 carrying an errorId was read as a successful write")
	}
	if !strings.Contains(err.Error(), "authentication") {
		t.Errorf("error is %q, want the envelope's reason", err)
	}
}

// TestCapabilitiesReportStepResultsOnAPlainExecution: the member row only
// offers the expansion when the backend declares it, and Zephyr is the only
// backend that can.
func TestCapabilitiesReportStepResultsOnAPlainExecution(t *testing.T) {
	a := New(nil, nil)
	if !a.Capabilities().SupportsRunStepResults {
		t.Error("a Zephyr profile does not declare step results on a plain execution")
	}
	// The parameterized pair stays off: there is no dataset here, so there is
	// no iteration to hang a step result from.
	if a.Capabilities().SupportsParameterizedRunDetail {
		t.Error("a Zephyr profile claims parameterized run detail it does not have")
	}
}
