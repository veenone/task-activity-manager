package zephyr

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/jira"
)

// zephyrServer answers the three endpoints a connection test touches: Jira's
// own /myself, Zephyr's Test issue type probe, and both status vocabularies.
// handler lets one test override a single path.
func zephyrServer(t *testing.T, override map[string]func(http.ResponseWriter)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h, ok := override[r.URL.Path]; ok {
			h(w)
			return
		}
		switch r.URL.Path {
		case "/rest/api/2/myself":
			_, _ = w.Write([]byte(`{"name":"qa.user","displayName":"QA User","emailAddress":"qa.user@example.internal"}`))
		case apiPath + "/util/zephyrTestIssueType":
			_, _ = w.Write([]byte(`{"testcaseIssueTypeId":"9"}`))
		case apiPath + "/util/testExecutionStatus", apiPath + "/util/teststepExecutionStatus":
			_, _ = w.Write([]byte(measuredStatuses))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func newTestAdapter(srv *httptest.Server) *Adapter {
	return New(jira.NewClient(srv.URL, "tok"), NewClientWithHTTP(srv.URL, "tok", srv.Client()))
}

// TestConnectionProvesZephyrIsInstalledAndReadsTheVocabularies covers the
// two-part check Phase 1 specifies. Jira answering /myself only proves the
// token works; it says nothing about whether the Zephyr plugin is there, and a
// Zephyr profile pointed at a plain Jira would otherwise connect and then fail
// on every call.
func TestConnectionProvesZephyrIsInstalledAndReadsTheVocabularies(t *testing.T) {
	srv := zephyrServer(t, nil)
	defer srv.Close()
	a := newTestAdapter(srv)

	u, err := a.TestConnection(context.Background())
	if err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
	if u == nil || u.Name != "qa.user" {
		t.Fatalf("TestConnection returned %+v, want the authenticated user", u)
	}

	id, err := a.z.TestIssueTypeID(context.Background())
	if err != nil {
		t.Fatalf("TestIssueTypeID after a connection test: %v", err)
	}
	if id != "9" {
		t.Errorf("Test issue type id = %q, want the id this instance reported", id)
	}
	v, err := a.z.ExecutionStatuses(context.Background())
	if err != nil {
		t.Fatalf("ExecutionStatuses after a connection test: %v", err)
	}
	if _, err := v.IDFor("PASS WARNING"); err != nil {
		t.Errorf("the connection test did not leave this instance's vocabulary available: %v", err)
	}
}

// TestConnectionFailsWhenZephyrIsAbsent is the case that matters in practice:
// a working Jira token on an instance with no Zephyr. The error has to name
// Zephyr, because "404" on its own sends the user looking at their token.
func TestConnectionFailsWhenZephyrIsAbsent(t *testing.T) {
	srv := zephyrServer(t, map[string]func(http.ResponseWriter){
		apiPath + "/util/zephyrTestIssueType": func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorMessages":["null for uri: ..."]}`))
		},
	})
	defer srv.Close()
	a := newTestAdapter(srv)

	_, err := a.TestConnection(context.Background())
	if err == nil {
		t.Fatal("TestConnection succeeded against an instance with no Zephyr")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "zephyr") {
		t.Errorf("error does not name Zephyr, so it reads as a credential problem: %v", err)
	}
}

// TestConnectionFailsWhenTheTokenIsRejectedWithA200 ties the client's body
// check to the connection test, which is where a wrong token surfaces first.
func TestConnectionFailsWhenTheTokenIsRejectedWithA200(t *testing.T) {
	srv := zephyrServer(t, map[string]func(http.ResponseWriter){
		apiPath + "/util/zephyrTestIssueType": func(w http.ResponseWriter) {
			_, _ = w.Write([]byte(`{"errorId":"ERROR","errorDesc":"Authentication failed."}`))
		},
	})
	defer srv.Close()
	a := newTestAdapter(srv)

	if _, err := a.TestConnection(context.Background()); err == nil {
		t.Fatal("a 200 carrying an errorId passed the connection test")
	}
}

// TestCapabilitiesDeclareWhatEssentialDoesNotHave pins the declaration to the
// four features the probes confirmed absent by 404. These flags are what make
// the repository, precondition, dataset and test plan views go dark rather
// than call endpoints that are not there.
func TestCapabilitiesDeclareWhatEssentialDoesNotHave(t *testing.T) {
	caps := (&Adapter{}).Capabilities()

	if caps.Name != "zephyr-essential-dc" {
		t.Errorf("Name = %q, want zephyr-essential-dc so a diagnostic names the product and the deployment", caps.Name)
	}
	absent := map[string]bool{
		"SupportsFolders":                    caps.SupportsFolders,
		"SupportsFolderWrites":               caps.SupportsFolderWrites,
		"SupportsPreconditionObjects":        caps.SupportsPreconditionObjects,
		"SupportsDatasets":                   caps.SupportsDatasets,
		"SupportsDatasetWrites":              caps.SupportsDatasetWrites,
		"SupportsParameterizedRunDetail":     caps.SupportsParameterizedRunDetail,
		"SupportsParameterizedRunWrites":     caps.SupportsParameterizedRunWrites,
		"SupportsParameterizedRunStepWrites": caps.SupportsParameterizedRunStepWrites,
		"SupportsTestTypes":                  caps.SupportsTestTypes,
		"SupportsEnvironments":               caps.SupportsEnvironments,
		"SupportsComponentAdmin":             caps.SupportsComponentAdmin,
	}
	for name, on := range absent {
		if on {
			t.Errorf("%s is declared, but Essential has no such feature", name)
		}
	}

	present := map[string]bool{
		"SupportsJQLScope":            caps.SupportsJQLScope,
		"SupportsRequirementObjects":  caps.SupportsRequirementObjects,
		"SupportsIssueLinkTypes":      caps.SupportsIssueLinkTypes,
		"SupportsContainers":          caps.SupportsContainers,
		"SupportsTestRuns":            caps.SupportsTestRuns,
		"SupportsWorkflowTransitions": caps.SupportsWorkflowTransitions,
		"SupportsBugCreation":         caps.SupportsBugCreation,
		"SupportsBugLinks":            caps.SupportsBugLinks,
		"SupportsTags":                caps.SupportsTags,
	}
	for name, on := range present {
		if !on {
			t.Errorf("%s is not declared, but a Zephyr test is a Jira issue so it holds", name)
		}
	}

	if len(caps.ContainerKinds) != 1 || caps.ContainerKinds[0] != backend.KindTestExec {
		t.Errorf("ContainerKinds = %v, want only the execution kind: Essential has no test sets or plans", caps.ContainerKinds)
	}
}

// TestAnUnsupportedMethodSaysSoRatherThanPanicking checks the stub contract
// the later phases replace one method at a time. A nil return with no error
// would read as "this project has none of those".
func TestAnUnsupportedMethodSaysSoRatherThanPanicking(t *testing.T) {
	a := &Adapter{}
	if _, err := a.GetDataset(context.Background(), "PROJ-101"); !errors.Is(err, backend.ErrUnsupported) {
		t.Errorf("GetDataset error = %v, want backend.ErrUnsupported", err)
	}
	if _, _, err := a.ListPreconditions(context.Background(), "PROJ", nil); !errors.Is(err, backend.ErrUnsupported) {
		t.Errorf("ListPreconditions error = %v, want backend.ErrUnsupported", err)
	}
}
