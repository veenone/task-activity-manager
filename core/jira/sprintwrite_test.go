package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// TestRawSprintDecodesItsGoal pins down the field this task adds: Jira has
// sent goal on every sprint since Phase 3a's boards work and this struct
// was dropping it on the floor.
func TestRawSprintDecodesItsGoal(t *testing.T) {
	var sprint RawSprint
	payload := []byte(`{"id":12,"name":"Sprint 12","state":"active","startDate":"2026-09-09T09:00:00.000+0000","endDate":"2026-09-23T09:00:00.000+0000","goal":"Ship the thing"}`)
	if err := json.Unmarshal(payload, &sprint); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sprint.Goal != "Ship the thing" {
		t.Errorf("goal = %q, want %q", sprint.Goal, "Ship the thing")
	}
}

// TestRawSprintDecodesItsCompleteDate pins down the other field this task
// adds: the moment a sprint actually closed, which a report needs instead of
// the planned end date because the two routinely differ by days.
func TestRawSprintDecodesItsCompleteDate(t *testing.T) {
	var sprint RawSprint
	payload := []byte(`{"id":12,"name":"Sprint 12","state":"closed","startDate":"2026-09-09T09:00:00.000+0000","endDate":"2026-09-23T09:00:00.000+0000","completeDate":"2026-09-26T14:00:00.000+0000"}`)
	if err := json.Unmarshal(payload, &sprint); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sprint.CompleteDate != "2026-09-26T14:00:00.000+0000" {
		t.Errorf("completeDate = %q, want the closed date three days after endDate", sprint.CompleteDate)
	}
}

// TestRawSprintDecodesANullCompleteDateAsEmpty is the ordinary case: a
// sprint still open sends completeDate as JSON null, not as an absent key.
func TestRawSprintDecodesANullCompleteDateAsEmpty(t *testing.T) {
	var sprint RawSprint
	payload := []byte(`{"id":13,"name":"Sprint 13","state":"active","completeDate":null}`)
	if err := json.Unmarshal(payload, &sprint); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sprint.CompleteDate != "" {
		t.Errorf("completeDate = %q, want empty for a sprint still open", sprint.CompleteDate)
	}
}

func TestCreateSprintSendsBoardIDNameAndDates(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":13,"name":"Sprint 13","state":"future","goal":"Ship the thing","startDate":"2026-09-09T09:00:00.000+0000","endDate":"2026-09-23T09:00:00.000+0000"}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	sprint, err := c.CreateSprint(context.Background(), 1, "Sprint 13", "Ship the thing", "2026-09-09T09:00:00.000+0000", "2026-09-23T09:00:00.000+0000")
	if err != nil {
		t.Fatalf("create sprint: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/rest/agile/1.0/sprint" {
		t.Errorf("path = %s", gotPath)
	}
	wantBody := map[string]any{
		"originBoardId": float64(1),
		"name":          "Sprint 13",
		"goal":          "Ship the thing",
		"startDate":     "2026-09-09T09:00:00.000+0000",
		"endDate":       "2026-09-23T09:00:00.000+0000",
	}
	if !reflect.DeepEqual(gotBody, wantBody) {
		t.Errorf("body = %+v, want %+v", gotBody, wantBody)
	}
	// The id and goal Jira answered with come back on the decoded sprint.
	if sprint.ID != 13 || sprint.Goal != "Ship the thing" {
		t.Errorf("sprint = %+v, want id 13 and the goal echoed back", sprint)
	}
}

// TestCreateSprintOmitsAnEmptyGoal pins down the same partial-update trap
// StartSprint already guards against: originBoardId, name, startDate and
// endDate go unconditionally, but an empty goal is left out of the body
// rather than sent as "".
func TestCreateSprintOmitsAnEmptyGoal(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":13,"name":"Sprint 13","state":"future","startDate":"2026-09-09T09:00:00.000+0000","endDate":"2026-09-23T09:00:00.000+0000"}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	if _, err := c.CreateSprint(context.Background(), 1, "Sprint 13", "", "2026-09-09T09:00:00.000+0000", "2026-09-23T09:00:00.000+0000"); err != nil {
		t.Fatalf("create sprint: %v", err)
	}
	if _, ok := gotBody["goal"]; ok {
		t.Errorf("body = %+v, want no goal key at all for an empty goal", gotBody)
	}
	wantBody := map[string]any{
		"originBoardId": float64(1),
		"name":          "Sprint 13",
		"startDate":     "2026-09-09T09:00:00.000+0000",
		"endDate":       "2026-09-23T09:00:00.000+0000",
	}
	if !reflect.DeepEqual(gotBody, wantBody) {
		t.Errorf("body = %+v, want %+v", gotBody, wantBody)
	}
}

// TestCreateSprintOnAnEmptyBodyReturnsTheZeroValue pins down the rule that a
// create Jira accepted (a 2xx) but answered with nothing usable is not a
// failure: the caller gets the zero value and is expected to refresh its
// sprint list rather than trust this call's own echo.
func TestCreateSprintOnAnEmptyBodyReturnsTheZeroValue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	sprint, err := c.CreateSprint(context.Background(), 1, "Sprint 13", "", "2026-09-09T09:00:00.000+0000", "2026-09-23T09:00:00.000+0000")
	if err != nil {
		t.Fatalf("create sprint: %v, want no error for an empty body", err)
	}
	if sprint != (RawSprint{}) {
		t.Errorf("sprint = %+v, want the zero value", sprint)
	}
}

// TestCreateSprintOnABodyWithNoIDReturnsTheZeroValue is the other half of the
// rule above: a body that is real JSON but carries no id is the same kind of
// nothing as no body at all, and neither fails a create Jira accepted.
func TestCreateSprintOnABodyWithNoIDReturnsTheZeroValue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	sprint, err := c.CreateSprint(context.Background(), 1, "Sprint 13", "", "2026-09-09T09:00:00.000+0000", "2026-09-23T09:00:00.000+0000")
	if err != nil {
		t.Fatalf("create sprint: %v, want no error for a body with no id", err)
	}
	if sprint.ID != 0 {
		t.Errorf("sprint = %+v, want the zero value", sprint)
	}
}

// TestCreateSprintOnAnAnswerThatIsNotJSONFails draws the line the comment on
// CreateSprint draws: a shape difference in Jira's echo is tolerable, but an
// HTML page answered with a 200, which is what a proxy in front of Jira
// produces, is evidence the request never reached Jira and the caller has to
// hear about it.
func TestCreateSprintOnAnAnswerThatIsNotJSONFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>Sign in to continue</body></html>`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	if _, err := c.CreateSprint(context.Background(), 1, "Sprint 13", "", "2026-09-09T09:00:00.000+0000", "2026-09-23T09:00:00.000+0000"); err == nil {
		t.Fatal("create sprint = nil error, want a failure for an answer that is not JSON")
	}
}

// TestUpdateSprintSendsOnlyTheGivenKeys pins down the same partial-update
// trap StartSprint's own comment explains: an edit changing only the name
// must send only the name, and state must never be sent since a rename is
// not a state change.
func TestUpdateSprintSendsOnlyTheGivenKeys(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":12,"name":"Sprint 12 renamed","state":"active"}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	if err := c.UpdateSprint(context.Background(), 12, "Sprint 12 renamed", "", "", "", false); err != nil {
		t.Fatalf("update sprint: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/rest/agile/1.0/sprint/12" {
		t.Errorf("path = %s", gotPath)
	}
	want := map[string]any{"name": "Sprint 12 renamed"}
	if !reflect.DeepEqual(gotBody, want) {
		t.Errorf("body = %+v, want %+v, nothing else and never state", gotBody, want)
	}
}

// TestUpdateSprintClearGoalFalseOmitsAnEmptyGoal and its sibling below pin
// down clearGoal, the one deliberate exception to the omit-when-empty rule:
// false leaves an empty goal out of the body the same as any other
// untouched field, true sends goal: "" on purpose.
func TestUpdateSprintClearGoalFalseOmitsAnEmptyGoal(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":12,"name":"Sprint 12","state":"active"}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	if err := c.UpdateSprint(context.Background(), 12, "", "", "", "", false); err != nil {
		t.Fatalf("update sprint: %v", err)
	}
	if _, ok := gotBody["goal"]; ok {
		t.Errorf("body = %+v, want no goal key when clearGoal is false", gotBody)
	}
	if len(gotBody) != 0 {
		t.Errorf("body = %+v, want an empty body when nothing changed", gotBody)
	}
}

func TestUpdateSprintClearGoalTrueSendsTheEmptyString(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":12,"name":"Sprint 12","state":"active","goal":""}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	if err := c.UpdateSprint(context.Background(), 12, "", "", "", "", true); err != nil {
		t.Fatalf("update sprint: %v", err)
	}
	want := map[string]any{"goal": ""}
	if !reflect.DeepEqual(gotBody, want) {
		t.Errorf("body = %+v, want %+v", gotBody, want)
	}
}

// TestUpdateSprintANonEmptyGoalWinsOverClearGoal pins the ordering between
// the two goal parameters: a goal to set takes the body over clearGoal, so
// a caller cannot be misread as clearing when it supplied a real one.
func TestUpdateSprintANonEmptyGoalWinsOverClearGoal(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":12,"goal":"New goal"}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	if err := c.UpdateSprint(context.Background(), 12, "", "New goal", "", "", true); err != nil {
		t.Fatalf("update sprint: %v", err)
	}
	want := map[string]any{"goal": "New goal"}
	if !reflect.DeepEqual(gotBody, want) {
		t.Errorf("body = %+v, want %+v", gotBody, want)
	}
}

func TestDeleteSprintSendsThePathWithTheID(t *testing.T) {
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	if err := c.DeleteSprint(context.Background(), 12); err != nil {
		t.Fatalf("delete sprint: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", gotMethod)
	}
	if gotPath != "/rest/agile/1.0/sprint/12" {
		t.Errorf("path = %s, want the sprint id in it", gotPath)
	}
}

// TestDeleteSprintOn404NamesTheSprint pins down the last leg of the spec:
// a 404 must come back naming which sprint, not just "not found". The path
// this call builds already carries the id, so the error text is checked for
// that id rather than for a made-up "sprint 999" phrase this package has
// never produced.
func TestDeleteSprintOn404NamesTheSprint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorMessages":["The sprint could not be found."]}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "tok", srv.Client())
	err := c.DeleteSprint(context.Background(), 999)
	if err == nil {
		t.Fatal("want an error for a 404")
	}
	id := strconv.Itoa(999)
	if !strings.Contains(err.Error(), id) {
		t.Errorf("err = %q, want it to name sprint %s", err.Error(), id)
	}
	if !strings.Contains(err.Error(), "The sprint could not be found.") {
		t.Errorf("err = %q, want Jira's own message kept", err.Error())
	}
}
