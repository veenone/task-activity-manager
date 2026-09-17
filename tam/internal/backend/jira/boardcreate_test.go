package jira_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
	jirabackend "agile-suite/tam/internal/backend/jira"
)

// boardCreateServer answers the three calls a board create can make: the
// filter POST, the board POST, and the filter DELETE a failed board create
// rolls back with. The filter POST always succeeds, since every test here
// is about what happens after the filter already exists; boardStatus and
// deleteStatus let a test choose how the other two answer.
func boardCreateServer(t *testing.T, boardStatus, deleteStatus int) (*httptest.Server, *[]string) {
	t.Helper()
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/rest/api/2/filter":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"10050"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/rest/agile/1.0/board":
			w.WriteHeader(boardStatus)
			if boardStatus >= 300 {
				_, _ = w.Write([]byte(`{"errorMessages":["A board with this name already exists."]}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":42}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/rest/api/2/filter/10050":
			w.WriteHeader(deleteStatus)
			if deleteStatus >= 300 {
				_, _ = w.Write([]byte(`{"errorMessages":["You do not have permission to delete this filter."]}`))
			}
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func newBoardBackend(srv *httptest.Server) *jirabackend.Backend {
	c := corejira.NewClientWithHTTP(srv.URL, "tok", srv.Client())
	b := jirabackend.New(c, "")
	return b
}

var draft = backend.BoardDraft{
	Name: "PLAT Scrum", Type: "scrum", FilterName: "PLAT Scrum filter", JQL: `project = "PLAT"`,
}

// TestCreateBoardRollsBackTheFilterWhenTheBoardCreateFails is the task's
// most important behaviour: a board create that fails after its filter
// landed must delete that filter, and the error must name both what failed
// and that the filter was cleaned up.
func TestCreateBoardRollsBackTheFilterWhenTheBoardCreateFails(t *testing.T) {
	srv, calls := boardCreateServer(t, http.StatusBadRequest, http.StatusNoContent)
	b := newBoardBackend(srv)

	_, err := b.CreateBoard(context.Background(), "PLAT", draft)
	if err == nil {
		t.Fatal("want an error: the board create was refused")
	}
	if !strings.Contains(err.Error(), "A board with this name already exists.") {
		t.Errorf("err = %v, want Jira's own board-create failure kept", err)
	}
	if !strings.Contains(err.Error(), "10050") || !strings.Contains(strings.ToLower(err.Error()), "delet") {
		t.Errorf("err = %v, want it to say the filter was deleted", err)
	}
	want := []string{"POST /rest/api/2/filter", "POST /rest/agile/1.0/board", "DELETE /rest/api/2/filter/10050"}
	if strings.Join(*calls, ", ") != strings.Join(want, ", ") {
		t.Errorf("calls = %v, want %v", *calls, want)
	}
}

// TestCreateBoardNamesTheStrandedFilterWhenCleanupAlsoFails is the other
// half: a rollback that itself fails must still report the original
// failure and must name the filter id it left behind, since losing that id
// would strand a filter nobody could find again.
func TestCreateBoardNamesTheStrandedFilterWhenCleanupAlsoFails(t *testing.T) {
	srv, _ := boardCreateServer(t, http.StatusBadRequest, http.StatusInternalServerError)
	b := newBoardBackend(srv)

	_, err := b.CreateBoard(context.Background(), "PLAT", draft)
	if err == nil {
		t.Fatal("want an error: the board create was refused")
	}
	if !strings.Contains(err.Error(), "A board with this name already exists.") {
		t.Errorf("err = %v, want the original board-create failure kept", err)
	}
	if !strings.Contains(err.Error(), "10050") {
		t.Errorf("err = %v, want the stranded filter's id named", err)
	}
	if !strings.Contains(err.Error(), "You do not have permission to delete this filter.") {
		t.Errorf("err = %v, want the cleanup failure's own reason kept too", err)
	}
}

// TestCreateBoardReturnsTheBoardID is the plain success path: no delete
// call at all when the board create lands.
func TestCreateBoardReturnsTheBoardID(t *testing.T) {
	srv, calls := boardCreateServer(t, http.StatusCreated, http.StatusNoContent)
	b := newBoardBackend(srv)

	id, err := b.CreateBoard(context.Background(), "PLAT", draft)
	if err != nil {
		t.Fatalf("create board: %v", err)
	}
	if id != 42 {
		t.Errorf("id = %d, want 42", id)
	}
	want := []string{"POST /rest/api/2/filter", "POST /rest/agile/1.0/board"}
	if strings.Join(*calls, ", ") != strings.Join(want, ", ") {
		t.Errorf("calls = %v, want %v, no delete on success", *calls, want)
	}
}
