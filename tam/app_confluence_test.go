package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"agile-suite/core/profile"
	"agile-suite/core/shareddb"
)

type testCredentialStore map[string]string

func (s testCredentialStore) Save(id, secret string) error { s[id] = secret; return nil }
func (s testCredentialStore) Load(id string) (string, error) {
	v, ok := s[id]
	if !ok {
		return "", errMissingCredential{}
	}
	return v, nil
}
func (s testCredentialStore) Delete(id string) error { delete(s, id); return nil }

type errMissingCredential struct{}

func (errMissingCredential) Error() string { return "missing credential" }

func testConfluenceApp(t *testing.T, handler http.Handler) (*App, profile.Profile) {
	t.Helper()
	st, err := shareddb.Open(filepath.Join(t.TempDir(), "shared.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	pm := profile.NewManager(st.DB())
	p, err := pm.Create("QA", "https://jira.example.com", "QA", "", "", "", "", "", false, "")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	if err := pm.SetConfluenceConfig(p.ID, profile.ConfluenceConfig{BaseURL: srv.URL, SpaceKey: "QA"}); err != nil {
		t.Fatal(err)
	}
	app := &App{ctx: context.Background(), shared: st, profiles: pm, creds: testCredentialStore{}}
	return app, p
}

func TestConfluenceClientReportsMissingCredential(t *testing.T) {
	app, p := testConfluenceApp(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("request should not be made") }))
	_, err := app.GetConfluencePage(p.ID, "42")
	if err == nil || err.Error() != "Confluence credentials are not configured for this profile" {
		t.Fatalf("error = %v", err)
	}
}

func TestConfluenceClientReportsMissingConfiguration(t *testing.T) {
	app, p := testConfluenceApp(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("request should not be made") }))
	if err := app.profiles.SetConfluenceConfig(p.ID, profile.ConfluenceConfig{}); err != nil {
		t.Fatal(err)
	}
	_, err := app.GetConfluencePage(p.ID, "42")
	if err == nil || err.Error() != "Confluence is not configured for this profile" {
		t.Fatalf("error = %v", err)
	}
}

func TestConfluenceClientReadsPage(t *testing.T) {
	app, p := testConfluenceApp(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/content/42" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"42","title":"Standup","body":{"storage":{"value":"<p>Done</p>"}}}`))
	}))
	app.creds.(testCredentialStore)[profile.ConfluenceCredentialID(p.ID)] = "secret"
	page, err := app.GetConfluencePage(p.ID, "42")
	if err != nil {
		t.Fatal(err)
	}
	if page.ID != "42" || page.Title != "Standup" || page.Body.Storage.Value != "<p>Done</p>" {
		t.Fatalf("page = %+v", page)
	}
}
