package jira

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestBoardsPageUntilIsLast(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/agile/1.0/board" {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("projectKeyOrId") != "PLAT" {
			t.Errorf("projectKeyOrId = %q", q.Get("projectKeyOrId"))
		}
		switch q.Get("startAt") {
		case "0":
			_, _ = w.Write([]byte(`{"isLast":false,"maxResults":2,"startAt":0,"values":[
				{"id":1,"name":"Platform board","type":"scrum"},
				{"id":2,"name":"Kanban board","type":"kanban"}
			]}`))
		case "2":
			_, _ = w.Write([]byte(`{"isLast":true,"maxResults":2,"startAt":2,"values":[
				{"id":3,"name":"Odd board","type":"simple"}
			]}`))
		default:
			t.Errorf("unexpected startAt %q", q.Get("startAt"))
			return
		}
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	boards, err := c.Boards(context.Background(), "PLAT")
	if err != nil {
		t.Fatalf("boards: %v", err)
	}
	if len(boards) != 3 {
		t.Fatalf("boards = %+v", boards)
	}
	if boards[0].ID != 1 || boards[0].Name != "Platform board" || boards[0].Type != "scrum" {
		t.Errorf("board 0 = %+v", boards[0])
	}
	if boards[1].ID != 2 || boards[1].Type != "kanban" {
		t.Errorf("board 1 = %+v", boards[1])
	}
	// A board type TAM does not know yet comes back unchanged: filtering by
	// type is the caller's job, not this transport's.
	if boards[2].ID != 3 || boards[2].Type != "simple" {
		t.Errorf("board 2 = %+v, want the unknown type passed through unchanged", boards[2])
	}
}

func TestBoardsReturnErrNoAgileOn404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<html><body>Not Found</body></html>`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	boards, err := c.Boards(context.Background(), "PLAT")
	if !errors.Is(err, ErrNoAgile) {
		t.Fatalf("err = %v, want ErrNoAgile", err)
	}
	if boards != nil {
		t.Errorf("boards = %+v, want nil", boards)
	}
}

func TestBoardConfigurationDecodesNestedColumnConfig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/agile/1.0/board/1/configuration" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":1,"columnConfig":{"columns":[
			{"name":"To Do","statuses":[{"id":"1"}]},
			{"name":"In Progress","statuses":[{"id":"3"},{"id":"10001"}]},
			{"name":"Done","statuses":[{"id":"5"}]}
		]}}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	cfg, err := c.BoardConfiguration(context.Background(), 1)
	if err != nil {
		t.Fatalf("configuration: %v", err)
	}
	if cfg.ID != 1 || len(cfg.ColumnConfig.Columns) != 3 {
		t.Fatalf("cfg = %+v", cfg)
	}
	if cfg.ColumnConfig.Columns[0].Name != "To Do" {
		t.Errorf("column 0 name = %q", cfg.ColumnConfig.Columns[0].Name)
	}
	if got := cfg.ColumnConfig.Columns[0].StatusIDs(); !reflect.DeepEqual(got, []string{"1"}) {
		t.Errorf("column 0 status ids = %v", got)
	}
	if got := cfg.ColumnConfig.Columns[1].StatusIDs(); !reflect.DeepEqual(got, []string{"3", "10001"}) {
		t.Errorf("column 1 status ids = %v, want in order", got)
	}
	if got := cfg.ColumnConfig.Columns[2].StatusIDs(); !reflect.DeepEqual(got, []string{"5"}) {
		t.Errorf("column 2 status ids = %v", got)
	}
}

func TestBoardConfigurationSurfacesA403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errorMessages":["You do not have permission to view this board."]}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	_, err := c.BoardConfiguration(context.Background(), 1)
	if err == nil {
		t.Fatal("want an error for a 403, the sync decides what to do with it")
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.Code != http.StatusForbidden {
		t.Fatalf("err = %v, want *HTTPError with Code 403", err)
	}
}

func TestSprintsPageAndAKanbanBoardHasNone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/agile/1.0/board/1/sprint":
			q := r.URL.Query()
			switch q.Get("startAt") {
			case "0":
				_, _ = w.Write([]byte(`{"isLast":false,"maxResults":2,"startAt":0,"values":[
					{"id":10,"name":"Sprint 10","state":"closed","startDate":"2024-01-01T00:00:00.000Z","endDate":"2024-01-15T00:00:00.000Z"},
					{"id":11,"name":"Sprint 11","state":"closed","startDate":"2024-01-16T00:00:00.000Z","endDate":"2024-01-30T00:00:00.000Z"}
				]}`))
			case "2":
				_, _ = w.Write([]byte(`{"isLast":true,"maxResults":2,"startAt":2,"values":[
					{"id":12,"name":"Sprint 12","state":"active","startDate":"2024-01-31T00:00:00.000Z","endDate":"2024-02-14T00:00:00.000Z"}
				]}`))
			default:
				t.Errorf("unexpected startAt %q", q.Get("startAt"))
				return
			}
		case "/rest/agile/1.0/board/2/sprint":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errorMessages":["The board does not support sprints"],"errors":{}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			return
		}
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())

	sprints, err := c.Sprints(context.Background(), 1)
	if err != nil {
		t.Fatalf("sprints: %v", err)
	}
	if len(sprints) != 3 {
		t.Fatalf("sprints = %+v", sprints)
	}
	if sprints[2].ID != 12 || sprints[2].Name != "Sprint 12" || sprints[2].State != "active" {
		t.Errorf("sprint 2 = %+v", sprints[2])
	}

	_, err = c.Sprints(context.Background(), 2)
	if !errors.Is(err, ErrNoSprints) {
		t.Fatalf("err = %v, want ErrNoSprints", err)
	}
	if err.Error() != ErrNoSprints.Error() || strings.Contains(err.Error(), "<") {
		t.Errorf("error text = %q, want exactly ErrNoSprints with no leaked HTML", err.Error())
	}
}

// TestSprintsOnlyRemapsTheFirstPageError pins down the guard in pageAgile
// that hands an error to onFirstPageErr only when start == 0. A board whose
// first page is fine but whose second page 400s is a real failure, not a
// kanban board reporting it has no sprints, so it must come back as the raw
// *HTTPError and not ErrNoSprints.
func TestSprintsOnlyRemapsTheFirstPageError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/agile/1.0/board/3/sprint" {
			t.Errorf("path = %s", r.URL.Path)
			return
		}
		switch r.URL.Query().Get("startAt") {
		case "0":
			_, _ = w.Write([]byte(`{"isLast":false,"maxResults":2,"startAt":0,"values":[
				{"id":20,"name":"Sprint 20","state":"closed","startDate":"2024-01-01T00:00:00.000Z","endDate":"2024-01-15T00:00:00.000Z"},
				{"id":21,"name":"Sprint 21","state":"closed","startDate":"2024-01-16T00:00:00.000Z","endDate":"2024-01-30T00:00:00.000Z"}
			]}`))
		case "2":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errorMessages":["Internal server error"],"errors":{}}`))
		default:
			t.Errorf("unexpected startAt %q", r.URL.Query().Get("startAt"))
			return
		}
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	sprints, err := c.Sprints(context.Background(), 3)
	if err == nil {
		t.Fatalf("sprints = %+v, want an error from the second page's 400", sprints)
	}
	if errors.Is(err, ErrNoSprints) {
		t.Fatalf("err = %v, want the real error, not ErrNoSprints: a 400 past the first page is not a kanban board", err)
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.Code != http.StatusBadRequest {
		t.Fatalf("err = %v, want *HTTPError with Code 400", err)
	}
}

func TestBoardIssueKeysPageTheSearchEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("fields") != "key" {
			t.Errorf("fields = %q", r.URL.Query().Get("fields"))
		}
		switch r.URL.Path {
		case "/rest/agile/1.0/board/1/issue":
			switch r.URL.Query().Get("startAt") {
			case "0":
				_, _ = w.Write([]byte(`{"startAt":0,"maxResults":2,"total":3,"issues":[{"key":"PLAT-1"},{"key":"PLAT-2"}]}`))
			case "2":
				_, _ = w.Write([]byte(`{"startAt":2,"maxResults":2,"total":3,"issues":[{"key":"PLAT-3"}]}`))
			default:
				t.Errorf("unexpected startAt %q", r.URL.Query().Get("startAt"))
				return
			}
		case "/rest/agile/1.0/board/1/sprint/12/issue":
			_, _ = w.Write([]byte(`{"startAt":0,"maxResults":50,"total":1,"issues":[{"key":"PLAT-12"}]}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			return
		}
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())

	keys, err := c.BoardIssueKeys(context.Background(), 1, "")
	if err != nil {
		t.Fatalf("board issue keys: %v", err)
	}
	if want := []string{"PLAT-1", "PLAT-2", "PLAT-3"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}

	keys, err = c.BoardIssueKeys(context.Background(), 1, "12")
	if err != nil {
		t.Fatalf("sprint issue keys: %v", err)
	}
	if want := []string{"PLAT-12"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
}

// TestBoardIssueKeysEscapesSprintID pins down url.PathEscape on the sprint
// id. "12" escapes to itself, which is why every other test in this file
// cannot catch a dropped escape; this one uses an id that actually changes
// under escaping, and compares the escaped path the handler received, not
// just the request outcome.
func TestBoardIssueKeysEscapesSprintID(t *testing.T) {
	var gotEscapedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEscapedPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{"startAt":0,"maxResults":50,"total":1,"issues":[{"key":"PLAT-9"}]}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	keys, err := c.BoardIssueKeys(context.Background(), 1, "12 a/b")
	if err != nil {
		t.Fatalf("sprint issue keys: %v", err)
	}
	if want := []string{"PLAT-9"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	if want := "/rest/agile/1.0/board/1/sprint/12%20a%2Fb/issue"; gotEscapedPath != want {
		t.Errorf("escaped path = %q, want %q", gotEscapedPath, want)
	}
}

func TestRankIssueSendsRankBeforeAndAfter(t *testing.T) {
	var gotBodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/agile/1.0/issue/rank" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Errorf("method = %s", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		gotBodies = append(gotBodies, body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	if err := c.RankIssue(context.Background(), "PLAT-412", "PLAT-409", true); err != nil {
		t.Fatalf("rank before: %v", err)
	}
	if err := c.RankIssue(context.Background(), "PLAT-412", "PLAT-409", false); err != nil {
		t.Fatalf("rank after: %v", err)
	}

	wantBefore := map[string]any{"issues": []any{"PLAT-412"}, "rankBeforeIssue": "PLAT-409"}
	wantAfter := map[string]any{"issues": []any{"PLAT-412"}, "rankAfterIssue": "PLAT-409"}
	if len(gotBodies) != 2 {
		t.Fatalf("gotBodies = %+v, want 2 requests", gotBodies)
	}
	if !reflect.DeepEqual(gotBodies[0], wantBefore) {
		t.Errorf("rank before body = %+v, want %+v", gotBodies[0], wantBefore)
	}
	if !reflect.DeepEqual(gotBodies[1], wantAfter) {
		t.Errorf("rank after body = %+v, want %+v", gotBodies[1], wantAfter)
	}
}

// TestRankIssueSurfacesA404 pins down the other failure mode a Multi-Status
// check can hide behind: a 404 on the rank path (the issue or the neighbour
// gone) has to come back as an error the caller can read, not be swallowed.
func TestRankIssueSurfacesA404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorMessages":["Issue does not exist"]}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	if err := c.RankIssue(context.Background(), "PLAT-412", "PLAT-409", true); err == nil {
		t.Fatal("want an error for a 404 on the rank path")
	}
}

func TestMoveToSprintSendsIssues(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/agile/1.0/sprint/12/issue" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	if err := c.MoveToSprint(context.Background(), "12", []string{"PLAT-412"}); err != nil {
		t.Fatalf("move to sprint: %v", err)
	}
	if want := (map[string]any{"issues": []any{"PLAT-412"}}); !reflect.DeepEqual(gotBody, want) {
		t.Errorf("body = %+v, want %+v", gotBody, want)
	}
}

func TestMoveToSprintEscapesSprintID(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	if err := c.MoveToSprint(context.Background(), "12 a/b", []string{"PLAT-412"}); err != nil {
		t.Fatalf("move to sprint: %v", err)
	}
	if want := "/rest/agile/1.0/sprint/12%20a%2Fb/issue"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
}

func TestMoveToBacklogSendsIssues(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/agile/1.0/backlog/issue" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	if err := c.MoveToBacklog(context.Background(), []string{"PLAT-412"}); err != nil {
		t.Fatalf("move to backlog: %v", err)
	}
	if want := (map[string]any{"issues": []any{"PLAT-412"}}); !reflect.DeepEqual(gotBody, want) {
		t.Errorf("body = %+v, want %+v", gotBody, want)
	}
}

// TestMoveToSprintRejectsOn207 is the case that matters most: WriteJSON
// treats any status under 300 as a win, so a 207 whose body rejects one of
// two issues has to be read and turned into an error naming that issue, not
// recorded as though both landed. A rank Jira refused that is treated as
// pushed is exactly the silent failure this transport exists to avoid.
func TestMoveToSprintRejectsOn207(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(`{"errors":{"PLAT-409":"The issue is already in this sprint"}}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	err := c.MoveToSprint(context.Background(), "12", []string{"PLAT-412", "PLAT-409"})
	if err == nil {
		t.Fatal("want an error for a 207 rejecting one of two issues")
	}
	if !strings.Contains(err.Error(), "PLAT-409") {
		t.Errorf("err = %v, want it to name the rejected issue PLAT-409", err)
	}
}
