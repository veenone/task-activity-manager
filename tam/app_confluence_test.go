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

func TestConfluencePagesReportsMissingCredential(t *testing.T) {
	app, p := testConfluenceApp(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("request should not be made") }))
	if err := app.profiles.SetConfluenceConfig(p.ID, profile.ConfluenceConfig{BaseURL: "https://confluence.example.com", SpaceKey: "QA", RootPageID: "1"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := app.confluencePages(p); err == nil || err.Error() != "Confluence credentials are not configured for this profile" {
		t.Fatalf("error = %v", err)
	}
}

func TestConfluencePagesNeedsASpaceAndARoot(t *testing.T) {
	app, p := testConfluenceApp(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("request should not be made") }))
	if _, _, err := app.confluencePages(p); err == nil || err.Error() != "Confluence needs a space key and a root page id before rituals can sync" {
		t.Fatalf("error = %v", err)
	}
}
