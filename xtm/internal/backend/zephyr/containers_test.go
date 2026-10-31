package zephyr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The cycle list is the shape trap worth a test of its own: it is an object
// keyed by cycle id, not an array, and it carries a -1 pseudo-cycle for the
// executions that belong to no cycle. Both entries below are the probed
// shapes.
const (
	measuredCyclesForVersion = `{"501":{"name":"Regression cycle","environment":"staging",
	  "versionName":"Release 2.4","description":"the release regression pass","totalExecutions":12}}`

	measuredCyclesUnscheduled = `{"-1":{"versionName":"Unscheduled","projectKey":"PROJ","versionId":-1,
	  "environment":"","totalExecutions":3,"totalCycleExecutions":3}}`

	measuredExecutionsInCycle = `{"executions":[
	  {"id":2001,"issueKey":"PROJ-101","status":{"id":2,"name":"FAIL"},"executedBy":"qa.user",
	   "comment":"the gateway returned no response","defects":[{"key":"PROJ-900"}]},
	  {"id":2002,"issueKey":"PROJ-102","status":2,"executedBy":"qa.user","defects":[]}],
	 "recordsCount":2}`

	measuredTraceability = `{"executions":[{"execution":{"id":"2001","issueKey":"PROJ-101",
	  "status":"WIP","statusId":"3","testCycle":"Regression cycle","versionName":"Release 2.4"}}],
	 "totalCount":1}`

	measuredOneExecution = `{"execution":{"id":2001,"issueKey":"PROJ-101","cycleId":501,
	  "cycleName":"Regression cycle","versionId":10200,"projectId":10100,"comment":""}}`
)

// cycleServer answers everything the container reads need: the Jira project
// with its versions, the cycle map per version, the executions in a cycle,
// traceability for one test and one execution by id. override replaces a
// single path, which is how the degradation cases are set up.
func cycleServer(t *testing.T, override map[string]func(http.ResponseWriter)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h, ok := override[r.URL.Path]; ok {
			h(w)
			return
		}
		switch r.URL.Path {
		case "/rest/api/2/project/PROJ":
			_, _ = w.Write([]byte(`{"id":"10100","key":"PROJ",
			  "versions":[{"id":"10200","name":"Release 2.4"},{"id":"10201","name":"Release 2.5"}]}`))
		case apiPath + "/cycle":
			switch r.URL.Query().Get("versionId") {
			case "10200":
				_, _ = w.Write([]byte(measuredCyclesForVersion))
			case "-1":
				_, _ = w.Write([]byte(measuredCyclesUnscheduled))
			default:
				_, _ = w.Write([]byte(`{}`))
			}
		case apiPath + "/execution":
			if r.URL.Query().Get("cycleId") == "501" {
				_, _ = w.Write([]byte(measuredExecutionsInCycle))
				return
			}
			_, _ = w.Write([]byte(`{"executions":[]}`))
		case apiPath + "/execution/2001":
			_, _ = w.Write([]byte(measuredOneExecution))
		case apiPath + "/traceability/executionsByTest":
			_, _ = w.Write([]byte(measuredTraceability))
		case apiPath + "/util/testExecutionStatus", apiPath + "/util/teststepExecutionStatus":
			_, _ = w.Write([]byte(measuredStatuses))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestListContainersReadsTheCycleMapAcrossVersions covers the whole walk: a
// cycle belongs to a Jira version, so the project's versions decide how many
// times /cycle is asked, and the unscheduled bucket is one of them.
func TestListContainersReadsTheCycleMapAcrossVersions(t *testing.T) {
	srv := cycleServer(t, nil)
	defer srv.Close()
	a := newTestAdapter(srv)

	var lastDone, lastTotal int
	containers, links, err := a.ListContainers(context.Background(), "PROJ", func(done, total int) {
		lastDone, lastTotal = done, total
	})
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	byKey := map[string]string{}
	for _, c := range containers {
		byKey[c.Key] = c.Summary
		if c.Kind != "testexec" {
			t.Errorf("container %s has kind %q, want the execution kind: a cycle is the only kind here", c.Key, c.Kind)
		}
	}
	if byKey["501"] != "Regression cycle" {
		t.Errorf("cycle 501 read as %q, want its name from the map value", byKey["501"])
	}
	if _, ok := byKey["-1"]; !ok {
		t.Errorf("the -1 pseudo-cycle was dropped, so ad-hoc executions would have no container: %v", byKey)
	}
	if lastTotal == 0 || lastDone != lastTotal {
		t.Errorf("progress ended at %d/%d, want it to finish at the version count", lastDone, lastTotal)
	}

	// The cycle list says nothing about membership, so the links come from the
	// executions inside each cycle, which is also where the run status is.
	if len(links) != 2 {
		t.Fatalf("read %d container links, want one per execution in the cycle", len(links))
	}
	for _, l := range links {
		if l.ContainerKey != "501" {
			t.Errorf("link for %s points at container %q, want the cycle the execution is in", l.TestKey, l.ContainerKey)
		}
		if l.RunStatus != "FAIL" {
			t.Errorf("link for %s has run status %q, want the status the execution carries", l.TestKey, l.RunStatus)
		}
	}
}

// TestListContainersStillListsCyclesWhenTheExecutionsCannotBeRead is the
// degradation that matters. The execution list endpoint is documented but was
// never probed, so a cycle list that fails whole because of it would take the
// Containers view down over a response shape nobody has confirmed.
func TestListContainersStillListsCyclesWhenTheExecutionsCannotBeRead(t *testing.T) {
	srv := cycleServer(t, map[string]func(http.ResponseWriter){
		apiPath + "/execution": func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errorDesc":"boom"}`))
		},
	})
	defer srv.Close()
	a := newTestAdapter(srv)

	containers, links, err := a.ListContainers(context.Background(), "PROJ", nil)
	if err != nil {
		t.Fatalf("ListContainers failed whole because the executions could not be read: %v", err)
	}
	if len(containers) == 0 {
		t.Error("no cycles were reported, although the cycle list itself answered")
	}
	if len(links) != 0 {
		t.Errorf("reported %d links from a failed execution read", len(links))
	}
}

// TestGetTestRunsReadsAStatusInEitherShape is the view's own call, so unlike
// the list walk it reports its failures. The status field is the unproven part
// of this response: Zephyr answers it as an object on some builds and as the
// bare numeric id on others, and the id only means something through this
// instance's own vocabulary.
func TestGetTestRunsReadsAStatusInEitherShape(t *testing.T) {
	srv := cycleServer(t, nil)
	defer srv.Close()
	a := newTestAdapter(srv)

	runs, err := a.GetTestRuns(context.Background(), "501")
	if err != nil {
		t.Fatalf("GetTestRuns: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("read %d runs, want one per execution", len(runs))
	}
	if runs[0].TestKey != "PROJ-101" || runs[0].Status != "FAIL" {
		t.Errorf("first run = %s/%s, want the status from the object form", runs[0].TestKey, runs[0].Status)
	}
	if runs[1].Status != "FAIL" {
		t.Errorf("second run status = %q, want id 2 resolved through this instance's vocabulary", runs[1].Status)
	}
	if runs[0].Comment != "the gateway returned no response" {
		t.Errorf("run comment = %q, want the execution's own comment", runs[0].Comment)
	}
	if len(runs[0].Defects) != 1 || runs[0].Defects[0] != "PROJ-900" {
		t.Errorf("run defects = %v, want the linked issue keys", runs[0].Defects)
	}
}

// TestGetTestRunsReportsAFailureRatherThanAnEmptyCycle separates the two
// callers: the list walk degrades, a view does not. An empty list here would
// read as a cycle nobody has run.
func TestGetTestRunsReportsAFailureRatherThanAnEmptyCycle(t *testing.T) {
	srv := cycleServer(t, map[string]func(http.ResponseWriter){
		apiPath + "/execution": func(w http.ResponseWriter) {
			_, _ = w.Write([]byte(`{"errorId":"ERROR","errorDesc":"Authentication failed."}`))
		},
	})
	defer srv.Close()
	a := newTestAdapter(srv)

	if _, err := a.GetTestRuns(context.Background(), "501"); err == nil {
		t.Fatal("GetTestRuns reported a cycle with no runs for a rejected request")
	}
}

// TestTestExecutionsForTestResolvesTheCycleEachExecutionBelongsTo is the
// cross-project discovery path. Traceability names the cycle but the
// containers it produces are keyed by cycle id, so that id has to come from
// somewhere: the execution read, which was probed, supplies it.
func TestTestExecutionsForTestResolvesTheCycleEachExecutionBelongsTo(t *testing.T) {
	srv := cycleServer(t, nil)
	defer srv.Close()
	a := newTestAdapter(srv)

	containers, links, err := a.TestExecutionsForTest(context.Background(), "PROJ-101")
	if err != nil {
		t.Fatalf("TestExecutionsForTest: %v", err)
	}
	// The syncer upserts these two lists in step, so a mismatch drops the
	// whole test rather than one row (SyncBugRunData).
	if len(containers) != len(links) {
		t.Fatalf("%d containers and %d links, want them index aligned", len(containers), len(links))
	}
	if len(containers) != 1 {
		t.Fatalf("read %d executions, want the one traceability reported", len(containers))
	}
	if containers[0].Key != "501" {
		t.Errorf("container key = %q, want the cycle id so it matches the cycle list", containers[0].Key)
	}
	if containers[0].Summary != "Regression cycle" {
		t.Errorf("container summary = %q, want the cycle name", containers[0].Summary)
	}
	if links[0].TestKey != "PROJ-101" || links[0].RunStatus != "WIP" {
		t.Errorf("link = %s/%s, want the test and the status traceability carries", links[0].TestKey, links[0].RunStatus)
	}
}

// TestTestExecutionsForTestDegradesWhenTraceabilityFails mirrors the Xray
// adapter's own contract for this method: the syncer calls it per affected
// test, so one failing test logs and moves on rather than ending the pass.
func TestTestExecutionsForTestDegradesWhenTraceabilityFails(t *testing.T) {
	srv := cycleServer(t, map[string]func(http.ResponseWriter){
		apiPath + "/traceability/executionsByTest": func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errorDesc":"no such test"}`))
		},
	})
	defer srv.Close()
	a := newTestAdapter(srv)

	containers, links, err := a.TestExecutionsForTest(context.Background(), "PROJ-101")
	if err != nil {
		t.Errorf("TestExecutionsForTest reported an error the syncer treats as fatal per test: %v", err)
	}
	if len(containers) != 0 || len(links) != 0 {
		t.Errorf("read %d containers and %d links from a failed traceability call", len(containers), len(links))
	}
}

// TestACycleMapWithStrayFieldsIsStillRead keeps the map decode honest. The
// response is keyed by cycle id, and anything whose key is not a number is not
// a cycle: reading one as a cycle would put a container named after a counter
// in the view.
func TestACycleMapWithStrayFieldsIsStillRead(t *testing.T) {
	srv := cycleServer(t, map[string]func(http.ResponseWriter){
		apiPath + "/cycle": func(w http.ResponseWriter) {
			_, _ = w.Write([]byte(`{"recordsCount":1,"502":{"name":"Smoke cycle","versionName":"Release 2.4"}}`))
		},
	})
	defer srv.Close()
	a := newTestAdapter(srv)

	containers, _, err := a.ListContainers(context.Background(), "PROJ", nil)
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	for _, c := range containers {
		if strings.Contains(strings.ToLower(c.Summary), "recordscount") || c.Key == "recordsCount" {
			t.Errorf("a counter field was read as a cycle: %+v", c)
		}
	}
	if len(containers) == 0 {
		t.Error("the real cycle beside the counter field was dropped too")
	}
}
