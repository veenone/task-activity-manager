package jira_test

import (
	"context"
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

func catalogBackend(t *testing.T, versions, components string) *jirabackend.Backend {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/2/project/PLAT/versions":
			_, _ = io.WriteString(w, versions)
		case "/rest/api/2/project/PLAT/components":
			_, _ = io.WriteString(w, components)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return jirabackend.New(corejira.NewClientWithHTTP(srv.URL, "tok", srv.Client()), "Requirement")
}

func TestCatalogConvertsVersionsAndComponents(t *testing.T) {
	b := catalogBackend(t,
		`[{"id":"10100","name":"v1.2","description":"d","startDate":"2026-12-01","releaseDate":"2026-12-15","released":true,"archived":true}]`,
		`[{"id":"10000","name":"Auth","description":"login","lead":{"name":"alee","displayName":"A. Lee"},"assigneeType":"COMPONENT_LEAD"}]`)
	var _ backend.CatalogBackend = b
	ctx := context.Background()

	vs, err := b.ProjectVersions(ctx, "PLAT")
	if err != nil {
		t.Fatalf("versions: %v", err)
	}
	wantV := []backend.Version{{ID: 10100, Project: "PLAT", Name: "v1.2", Description: "d", StartDate: "2026-12-01", ReleaseDate: "2026-12-15", Released: true, Archived: true}}
	if !reflect.DeepEqual(vs, wantV) {
		t.Errorf("versions %+v", vs)
	}
	cs, err := b.ProjectComponents(ctx, "PLAT")
	if err != nil {
		t.Fatalf("components: %v", err)
	}
	wantC := []backend.Component{{ID: 10000, Project: "PLAT", Name: "Auth", Description: "login", LeadName: "alee", LeadDisplayName: "A. Lee", AssigneeType: "COMPONENT_LEAD"}}
	if !reflect.DeepEqual(cs, wantC) {
		t.Errorf("components %+v", cs)
	}
}

func TestCatalogRejectsANonNumericID(t *testing.T) {
	b := catalogBackend(t, `[{"id":"abc","name":"v1"}]`, `[{"id":"x1","name":"Auth"}]`)
	if _, err := b.ProjectVersions(context.Background(), "PLAT"); err == nil || !strings.Contains(err.Error(), "abc") {
		t.Errorf("versions err %v, want one naming the id", err)
	}
	if _, err := b.ProjectComponents(context.Background(), "PLAT"); err == nil || !strings.Contains(err.Error(), "x1") {
		t.Errorf("components err %v, want one naming the id", err)
	}
}
