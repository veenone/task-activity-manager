package jira

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// recorded is what a test server saw of the one request it answered.
type recorded struct {
	method, path, query string
	body                map[string]any
}

func recordingServer(t *testing.T, code int, answer string, rec *recorded) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method, rec.path, rec.query = r.Method, r.URL.Path, r.URL.RawQuery
		rec.body = nil
		_ = json.NewDecoder(r.Body).Decode(&rec.body)
		w.WriteHeader(code)
		_, _ = io.WriteString(w, answer)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestProjectVersionsDecodes(t *testing.T) {
	var rec recorded
	srv := recordingServer(t, http.StatusOK, `[
		{"id":"10100","name":"v2.4","description":"d","startDate":"2026-12-01","releaseDate":"2026-12-15","released":false,"archived":false},
		{"id":"10090","name":"v2.3","released":true,"archived":true}]`, &rec)

	got, err := newTestClient(srv).ProjectVersions(context.Background(), "PLAT")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rec.path != "/rest/api/2/project/PLAT/versions" {
		t.Errorf("path %s", rec.path)
	}
	want := []Version{
		{ID: "10100", Name: "v2.4", Description: "d", StartDate: "2026-12-01", ReleaseDate: "2026-12-15"},
		{ID: "10090", Name: "v2.3", Released: true, Archived: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestCreateVersionOmitsEmptyOptionalFields(t *testing.T) {
	var rec recorded
	srv := recordingServer(t, http.StatusCreated, `{"id":"10200","name":"v3"}`, &rec)

	got, err := newTestClient(srv).CreateVersion(context.Background(), VersionInput{Project: "PLAT", Name: "v3"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rec.method != http.MethodPost || rec.path != "/rest/api/2/version" {
		t.Errorf("%s %s", rec.method, rec.path)
	}
	if want := map[string]any{"project": "PLAT", "name": "v3"}; !reflect.DeepEqual(rec.body, want) {
		t.Errorf("body %v", rec.body)
	}
	if got != (Version{ID: "10200", Name: "v3"}) {
		t.Errorf("got %+v", got)
	}
}

func TestUpdateVersionClearsAnEmptyDate(t *testing.T) {
	var rec recorded
	srv := recordingServer(t, http.StatusOK, `{"id":"10100","name":"v2.4"}`, &rec)

	if _, err := newTestClient(srv).UpdateVersion(context.Background(), "10100",
		VersionInput{Project: "PLAT", Name: "v2.4", ReleaseDate: "2026-12-20"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if rec.method != http.MethodPut || rec.path != "/rest/api/2/version/10100" {
		t.Errorf("%s %s", rec.method, rec.path)
	}
	want := map[string]any{"name": "v2.4", "description": "", "startDate": nil, "releaseDate": "2026-12-20"}
	if !reflect.DeepEqual(rec.body, want) {
		t.Errorf("body %v", rec.body)
	}
}

func TestReleaseVersionMovesUnfixedByURI(t *testing.T) {
	var rec recorded
	srv := recordingServer(t, http.StatusOK, `{}`, &rec)

	if err := newTestClient(srv).ReleaseVersion(context.Background(), "10100", "2026-12-15", "10200"); err != nil {
		t.Fatalf("err: %v", err)
	}
	want := map[string]any{"released": true, "releaseDate": "2026-12-15",
		"moveUnfixedIssuesTo": srv.URL + "/rest/api/2/version/10200"}
	if rec.method != http.MethodPut || rec.path != "/rest/api/2/version/10100" || !reflect.DeepEqual(rec.body, want) {
		t.Errorf("%s %s %v", rec.method, rec.path, rec.body)
	}
}

func TestReleaseVersionWithoutMoveSendsNoMove(t *testing.T) {
	var rec recorded
	srv := recordingServer(t, http.StatusOK, `{}`, &rec)

	if err := newTestClient(srv).ReleaseVersion(context.Background(), "10100", "2026-12-15", ""); err != nil {
		t.Fatalf("err: %v", err)
	}
	if want := map[string]any{"released": true, "releaseDate": "2026-12-15"}; !reflect.DeepEqual(rec.body, want) {
		t.Errorf("body %v", rec.body)
	}
}

func TestArchiveVersionSendsOnlyArchived(t *testing.T) {
	var rec recorded
	srv := recordingServer(t, http.StatusOK, `{}`, &rec)

	if err := newTestClient(srv).ArchiveVersion(context.Background(), "10100", false); err != nil {
		t.Fatalf("err: %v", err)
	}
	if want := map[string]any{"archived": false}; rec.method != http.MethodPut || !reflect.DeepEqual(rec.body, want) {
		t.Errorf("%s %v", rec.method, rec.body)
	}
}

func TestDeleteVersionMovesFixIssues(t *testing.T) {
	var rec recorded
	srv := recordingServer(t, http.StatusNoContent, ``, &rec)
	c := newTestClient(srv)

	if err := c.DeleteVersion(context.Background(), "10100", "10200"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if rec.method != http.MethodDelete || rec.path != "/rest/api/2/version/10100" || rec.query != "moveFixIssuesTo=10200" {
		t.Errorf("%s %s ?%s", rec.method, rec.path, rec.query)
	}
	if err := c.DeleteVersion(context.Background(), "10100", ""); err != nil {
		t.Fatalf("err: %v", err)
	}
	if rec.query != "" {
		t.Errorf("query %q without a move", rec.query)
	}
}

func TestVersionWriteReportsJiraMessage(t *testing.T) {
	var rec recorded
	srv := recordingServer(t, http.StatusBadRequest,
		`{"errors":{"name":"A version with this name already exists in this project."}}`, &rec)

	_, err := newTestClient(srv).CreateVersion(context.Background(), VersionInput{Project: "PLAT", Name: "v2.4"})
	var he *HTTPError
	if !errors.As(err, &he) || he.Code != http.StatusBadRequest {
		t.Fatalf("want *HTTPError 400, got %v", err)
	}
	if !strings.Contains(he.Message, "name: A version with this name") {
		t.Errorf("message %q", he.Message)
	}
}
