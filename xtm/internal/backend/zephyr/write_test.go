package zephyr

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// recorded is one request the write server saw.
type recorded struct {
	Method string
	Path   string
	Query  string
	Body   string
}

// writeServer answers the reads a write has to make first and records every
// request. handler lets a test replace one path's response.
//
// The reads are the measured ones: the Jira project for its id and versions,
// traceability for a test's executions, and both status vocabularies.
func writeServer(t *testing.T, handler map[string]func(http.ResponseWriter, *http.Request)) (*httptest.Server, *[]recorded) {
	t.Helper()
	var (
		mu   sync.Mutex
		seen []recorded
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, recorded{r.Method, r.URL.Path, r.URL.RawQuery, string(body)})
		mu.Unlock()

		if h, ok := handler[r.URL.Path]; ok {
			h(w, r)
			return
		}
		switch r.URL.Path {
		case "/rest/api/2/project/PROJ":
			_, _ = w.Write([]byte(`{"id":"10100","versions":[{"id":"10200","name":"Release 2.4"}]}`))
		case apiPath + "/util/testExecutionStatus", apiPath + "/util/teststepExecutionStatus":
			_, _ = w.Write([]byte(measuredStatuses))
		case apiPath + "/traceability/executionsByTest":
			_, _ = w.Write([]byte(`{"executions":[{"execution":{"id":"2001","issueKey":"PROJ-101",` +
				`"status":"WIP","cycleId":"501","testCycle":"Regression cycle"}}],"totalCount":1}`))
		case apiPath + "/cycle":
			// Keyed by cycle id, with a counter mixed in, as measured.
			_, _ = w.Write([]byte(`{"501":{"name":"Regression cycle","versionId":10200},"recordsCount":1}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return srv, &seen
}

func findRequest(seen []recorded, method, path string) (recorded, bool) {
	for _, r := range seen {
		if r.Method == method && r.Path == path {
			return r, true
		}
	}
	return recorded{}, false
}

// TestSetTestRunStatusResolvesTheExecutionAndTheStatusID is the two-step write
// the design document calls out.
//
// XTM hands over a cycle key, a test key and a status name. Zephyr needs a
// numeric execution id and this instance's numeric status id, and neither is
// in the call, so both are resolved first. Sending the name, or the cycle id
// in place of the execution id, would write to the wrong place or not at all.
func TestSetTestRunStatusResolvesTheExecutionAndTheStatusID(t *testing.T) {
	srv, seen := writeServer(t, map[string]func(http.ResponseWriter, *http.Request){
		apiPath + "/execution/2001/execute": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"id":2001,"status":2}`))
		},
	})
	defer srv.Close()
	a := newTestAdapter(srv)

	if err := a.SetTestRunStatus(context.Background(), "501", "PROJ-101", "FAIL"); err != nil {
		t.Fatalf("SetTestRunStatus: %v", err)
	}
	req, ok := findRequest(*seen, http.MethodPut, apiPath+"/execution/2001/execute")
	if !ok {
		t.Fatalf("no write to the execution; requests were %+v", *seen)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("write body %q: %v", req.Body, err)
	}
	// FAIL is id 2 on this instance, read from it rather than compiled in.
	if got, want := body["status"], float64(2); got != want {
		t.Errorf("wrote status %v, want this instance's numeric id %v", got, want)
	}
}

// TestSetTestRunStatusRefusesAStatusThisInstanceDoesNotDefine keeps a result
// the instance has no id for out of the write. Picking a default would record
// a result the user did not choose.
func TestSetTestRunStatusRefusesAStatusThisInstanceDoesNotDefine(t *testing.T) {
	srv, seen := writeServer(t, nil)
	defer srv.Close()

	err := newTestAdapter(srv).SetTestRunStatus(context.Background(), "501", "PROJ-101", "ABORTED")
	if err == nil {
		t.Fatal("a status this instance does not define was accepted")
	}
	if !strings.Contains(err.Error(), "PASS WARNING") {
		t.Errorf("error %q does not say what this instance does define", err)
	}
	if _, wrote := findRequest(*seen, http.MethodPut, apiPath+"/execution/2001/execute"); wrote {
		t.Error("the write went out anyway")
	}
}

// TestSetTestRunStatusSaysWhenTheTestIsNotInThatCycle is the other refusal.
// Traceability answers for the test, so a test with executions elsewhere but
// none in this cycle has to be told apart from a test with none at all.
func TestSetTestRunStatusSaysWhenTheTestIsNotInThatCycle(t *testing.T) {
	srv, seen := writeServer(t, nil)
	defer srv.Close()

	err := newTestAdapter(srv).SetTestRunStatus(context.Background(), "999", "PROJ-101", "PASS")
	if err == nil {
		t.Fatal("writing a result to a cycle the test is not in was accepted")
	}
	if !strings.Contains(err.Error(), "999") || !strings.Contains(err.Error(), "PROJ-101") {
		t.Errorf("error %q names neither the cycle nor the test", err)
	}
	for _, r := range *seen {
		if r.Method == http.MethodPut {
			t.Errorf("a write went out: %+v", r)
		}
	}
}

// TestSetTestRunStatusRejectsAnErrorEnvelopeOnA200 is the Zephyr-specific trap
// a write must not fall into: an invalid authentication can answer 200 with an
// errorId, so a write that trusts the status code reports a result as pushed
// that was never accepted, and the pending change is cleared.
func TestSetTestRunStatusRejectsAnErrorEnvelopeOnA200(t *testing.T) {
	srv, _ := writeServer(t, map[string]func(http.ResponseWriter, *http.Request){
		apiPath + "/execution/2001/execute": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"errorId":"ERROR","errorDesc":"You do not have permission to execute this test"}`))
		},
	})
	defer srv.Close()

	err := newTestAdapter(srv).SetTestRunStatus(context.Background(), "501", "PROJ-101", "PASS")
	if err == nil {
		t.Fatal("a 200 carrying an error envelope was read as a successful write")
	}
	if !strings.Contains(err.Error(), "permission") {
		t.Errorf("error %q drops what the instance said", err)
	}
}

// TestCreateContainerCreatesACycle covers the one container kind Essential
// has. A cycle belongs to a Jira version and the call carries none, so it is
// created in the unscheduled bucket, which is where Zephyr itself puts a cycle
// with no release.
func TestCreateContainerCreatesACycle(t *testing.T) {
	srv, seen := writeServer(t, map[string]func(http.ResponseWriter, *http.Request){
		apiPath + "/cycle": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				_, _ = w.Write([]byte(`{"id":"777","responseMessage":"Cycle Regression created successfully."}`))
				return
			}
			_, _ = w.Write([]byte(`{}`))
		},
	})
	defer srv.Close()

	key, err := newTestAdapter(srv).CreateContainer(context.Background(), "PROJ", "testexec", "Regression")
	if err != nil {
		t.Fatalf("CreateContainer: %v", err)
	}
	if key != "777" {
		t.Errorf("new container key = %q, want the cycle id the instance returned", key)
	}
	req, ok := findRequest(*seen, http.MethodPost, apiPath+"/cycle")
	if !ok {
		t.Fatalf("no cycle was created; requests were %+v", *seen)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("create body %q: %v", req.Body, err)
	}
	if body["name"] != "Regression" {
		t.Errorf("created a cycle named %v, want the summary the caller gave", body["name"])
	}
	if body["projectId"] != "10100" {
		t.Errorf("created it in project %v, want the id of the project key", body["projectId"])
	}
	if body["versionId"] != "-1" {
		t.Errorf("created it in version %v, want the unscheduled bucket", body["versionId"])
	}
}

// TestCreateContainerRefusesAKindEssentialDoesNotHave stops a Test Set or a
// Test Plan being created as a cycle, which would silently make the wrong
// thing rather than reporting that this backend has no such kind.
func TestCreateContainerRefusesAKindEssentialDoesNotHave(t *testing.T) {
	srv, seen := writeServer(t, nil)
	defer srv.Close()

	if _, err := newTestAdapter(srv).CreateContainer(context.Background(), "PROJ", "testplan", "Q3 plan"); err == nil {
		t.Fatal("a test plan was accepted on a backend that has none")
	}
	if _, wrote := findRequest(*seen, http.MethodPost, apiPath+"/cycle"); wrote {
		t.Error("it created a cycle anyway")
	}
}

// TestAddTestsToContainerWaitsForTheJob covers the asynchronous write. Zephyr
// answers the add with a job token and the tests are not in the cycle when the
// call returns, while the commit engine expects a write that has landed, so
// the token is polled rather than ignored.
func TestAddTestsToContainerWaitsForTheJob(t *testing.T) {
	polls := 0
	srv, seen := writeServer(t, map[string]func(http.ResponseWriter, *http.Request){
		apiPath + "/execution/addTestsToCycle": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"jobProgressToken":"job-1"}`))
		},
		apiPath + "/execution/jobProgress/job-1": func(w http.ResponseWriter, r *http.Request) {
			polls++
			if polls < 2 {
				_, _ = w.Write([]byte(`{"progress":0.5}`))
				return
			}
			_, _ = w.Write([]byte(`{"progress":1.0}`))
		},
	})
	defer srv.Close()

	err := newTestAdapter(srv).AddTestsToContainer(context.Background(), "testexec", "501", []string{"PROJ-101"})
	if err != nil {
		t.Fatalf("AddTestsToContainer: %v", err)
	}
	if polls < 2 {
		t.Errorf("polled the job %d times, want it followed until it finished", polls)
	}
	req, ok := findRequest(*seen, http.MethodPost, apiPath+"/execution/addTestsToCycle")
	if !ok {
		t.Fatalf("no add went out; requests were %+v", *seen)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("add body %q: %v", req.Body, err)
	}
	if body["cycleId"] != "501" || body["projectId"] != "10100" || body["versionId"] != "10200" {
		t.Errorf("add body = %v, want the cycle with the project and version it belongs to", body)
	}
	if body["method"] != "1" {
		t.Errorf("method = %v, want the documented 1 for adding by issue key", body["method"])
	}
	issues, _ := body["issues"].([]any)
	if len(issues) != 1 || issues[0] != "PROJ-101" {
		t.Errorf("issues = %v, want the test keys the caller gave", body["issues"])
	}
}

// TestAddTestsToContainerReportsAFailedJob is why the token is polled at all.
// A job that was accepted and then failed would otherwise clear the pending
// change and leave the cycle without the tests.
func TestAddTestsToContainerReportsAFailedJob(t *testing.T) {
	srv, _ := writeServer(t, map[string]func(http.ResponseWriter, *http.Request){
		apiPath + "/execution/addTestsToCycle": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"jobProgressToken":"job-1"}`))
		},
		apiPath + "/execution/jobProgress/job-1": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"progress":0.4,"errorMessage":"Tests could not be added"}`))
		},
	})
	defer srv.Close()

	err := newTestAdapter(srv).AddTestsToContainer(context.Background(), "testexec", "501", []string{"PROJ-101"})
	if err == nil {
		t.Fatal("a failed job was reported as a successful add")
	}
	if !strings.Contains(err.Error(), "could not be added") {
		t.Errorf("error %q drops what the job said", err)
	}
}

// TestAddTestsToContainerAcceptsAnAddWithNoJobToken covers the shape the
// documentation leaves open: whether a single-test add is asynchronous at all.
// An add that answered without a token has already happened, so waiting for a
// job that does not exist would fail a write that worked.
func TestAddTestsToContainerAcceptsAnAddWithNoJobToken(t *testing.T) {
	srv, seen := writeServer(t, map[string]func(http.ResponseWriter, *http.Request){
		apiPath + "/execution/addTestsToCycle": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"responseMessage":"1 test added"}`))
		},
	})
	defer srv.Close()

	if err := newTestAdapter(srv).AddTestsToContainer(context.Background(), "testexec", "501", []string{"PROJ-101"}); err != nil {
		t.Fatalf("AddTestsToContainer: %v", err)
	}
	for _, r := range *seen {
		if strings.Contains(r.Path, "/jobProgress/") {
			t.Errorf("polled for a job the instance never started: %+v", r)
		}
	}
}

// TestAddTestsToContainerSaysWhenItCannotFindTheCycle keeps the add off an
// unknown cycle. The project and version a cycle belongs to are not in the
// call, so they are looked up, and a cycle id that is in no version's list
// would otherwise be added to with the wrong version.
func TestAddTestsToContainerSaysWhenItCannotFindTheCycle(t *testing.T) {
	srv, seen := writeServer(t, nil)
	defer srv.Close()

	err := newTestAdapter(srv).AddTestsToContainer(context.Background(), "testexec", "4242", []string{"PROJ-101"})
	if err == nil {
		t.Fatal("an add to an unknown cycle was accepted")
	}
	if !strings.Contains(err.Error(), "4242") {
		t.Errorf("error %q does not name the cycle", err)
	}
	if _, wrote := findRequest(*seen, http.MethodPost, apiPath+"/execution/addTestsToCycle"); wrote {
		t.Error("the add went out anyway")
	}
}

// TestAddTestsToContainerAcceptsAJobItCouldNotSeeFinish pins a deliberate
// choice rather than an oversight.
//
// The add was accepted. A job still running when the wait runs out is reported
// as done, because the alternative is leaving the row pending and adding every
// test a second time on the next commit. The next sync shows what landed.
func TestAddTestsToContainerAcceptsAJobItCouldNotSeeFinish(t *testing.T) {
	srv, _ := writeServer(t, map[string]func(http.ResponseWriter, *http.Request){
		apiPath + "/execution/addTestsToCycle": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"jobProgressToken":"job-1"}`))
		},
		apiPath + "/execution/jobProgress/job-1": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"progress":0.5}`))
		},
	})
	defer srv.Close()

	attempts, interval := jobPollAttempts, jobPollInterval
	jobPollAttempts, jobPollInterval = 2, time.Millisecond
	defer func() { jobPollAttempts, jobPollInterval = attempts, interval }()

	if err := newTestAdapter(srv).AddTestsToContainer(context.Background(), "testexec", "501", []string{"PROJ-101"}); err != nil {
		t.Fatalf("an accepted add that had not finished was reported as a failure: %v", err)
	}
}

// TestCreateContainerRefusesACreateThatNamesNoID is not a nicety. The caller
// keys the new container by the id and journals pending work against it, so a
// create that cannot say what it made has to fail rather than hand back an
// empty key that every later write would then miss.
func TestCreateContainerRefusesACreateThatNamesNoID(t *testing.T) {
	srv, _ := writeServer(t, map[string]func(http.ResponseWriter, *http.Request){
		apiPath + "/cycle": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"responseMessage":"Cycle Regression created successfully."}`))
		},
	})
	defer srv.Close()

	key, err := newTestAdapter(srv).CreateContainer(context.Background(), "PROJ", "testexec", "Regression")
	if err == nil {
		t.Fatalf("a create that named no id returned key %q as a success", key)
	}
	if key != "" {
		t.Errorf("returned key %q alongside the error", key)
	}
}
