package jira_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
	jirabackend "agile-suite/tam/internal/backend/jira"
)

type seen struct {
	method, path, query string
	body                map[string]any
}

func versionBackend(t *testing.T, answer func(r *http.Request) (int, string)) (*jirabackend.Backend, *[]seen, string) {
	t.Helper()
	var got []seen
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := seen{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &s.body)
		got = append(got, s)
		code, body := answer(r)
		w.WriteHeader(code)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return jirabackend.New(corejira.NewClientWithHTTP(srv.URL, "tok", srv.Client()), "Requirement"), &got, srv.URL
}

func TestVersionWritesConvertIDs(t *testing.T) {
	b, got, base := versionBackend(t, func(r *http.Request) (int, string) {
		if r.Method == http.MethodPost {
			return http.StatusCreated, `{"id":"10200","name":"v2.0","releaseDate":"2026-12-15"}`
		}
		return http.StatusOK, `{"id":"10100","name":"v1.2b"}`
	})
	var _ backend.VersionBackend = b
	ctx := context.Background()
	made, err := b.CreateVersion(ctx, "PLAT", backend.VersionInput{Name: "v2.0", ReleaseDate: "2026-12-15"})
	if err != nil {
		t.Fatal(err)
	}
	if made.ID != 10200 || made.Project != "PLAT" || made.Name != "v2.0" || made.ReleaseDate != "2026-12-15" {
		t.Fatalf("made %+v", made)
	}
	if _, err := b.UpdateVersion(ctx, 10100, backend.VersionInput{Name: "v1.2b"}); err != nil {
		t.Fatal(err)
	}
	if err := b.ReleaseVersion(ctx, 10100, "2026-12-15", 10200); err != nil {
		t.Fatal(err)
	}
	if err := b.ArchiveVersion(ctx, 10100, true); err != nil {
		t.Fatal(err)
	}
	if err := b.DeleteVersion(ctx, 10100, 10200); err != nil {
		t.Fatal(err)
	}
	calls := *got
	if len(calls) != 5 {
		t.Fatalf("calls %+v", calls)
	}
	if calls[0].body["project"] != "PLAT" || calls[1].path != "/rest/api/2/version/10100" {
		t.Errorf("create/update %+v %+v", calls[0], calls[1])
	}
	if calls[2].body["moveUnfixedIssuesTo"] != base+"/rest/api/2/version/10200" || calls[2].body["released"] != true {
		t.Errorf("release body %v", calls[2].body)
	}
	if calls[3].body["archived"] != true {
		t.Errorf("archive body %v", calls[3].body)
	}
	if calls[4].method != http.MethodDelete || calls[4].query != "moveFixIssuesTo=10200" {
		t.Errorf("delete %+v", calls[4])
	}
	// No target means no move at all, not a move to version 0.
	if err := b.DeleteVersion(ctx, 10100, 0); err != nil {
		t.Fatal(err)
	}
	if last := (*got)[len(*got)-1]; last.query != "" {
		t.Errorf("delete without a target sent %q", last.query)
	}
}

func TestUnresolvedInVersionPagesTheSearch(t *testing.T) {
	b, got, _ := versionBackend(t, func(r *http.Request) (int, string) {
		start := r.URL.Query().Get("startAt")
		if start == "0" {
			return http.StatusOK, `{"total":3,"issues":[{"key":"PLAT-1","fields":{}},{"key":"PLAT-2","fields":{}}]}`
		}
		return http.StatusOK, `{"total":3,"issues":[{"key":"OTHER-9","fields":{}}]}`
	})
	keys, err := b.UnresolvedInVersion(context.Background(), 10100)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(keys, []string{"PLAT-1", "PLAT-2", "OTHER-9"}) {
		t.Fatalf("keys %v", keys)
	}
	first := (*got)[0]
	jql := first.body["jql"]
	if jql == nil {
		jql = first.query
	}
	text := fmt.Sprint(jql)
	if !strings.Contains(text, "fixVersion") || !strings.Contains(text, "10100") || !strings.Contains(text, "statusCategory") {
		t.Errorf("search %+v", first)
	}
}
