package main

import (
	"path/filepath"
	"strings"
	"testing"

	"agile-suite/core/profile"
	"agile-suite/core/settings"
	"agile-suite/core/store"
	"agile-suite/pie/internal/saved"
)

// memCreds is a credential store in memory.
type memCreds map[string]string

func (m memCreds) Save(id, secret string) error   { m[id] = secret; return nil }
func (m memCreds) Load(id string) (string, error) { return m[id], nil }
func (m memCreds) Delete(id string) error         { delete(m, id); return nil }

func profileApp(t *testing.T) (*App, memCreds) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "pie.db"), saved.Schema())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	creds := memCreds{}
	return &App{
		profiles: profile.NewManager(db.DB()),
		settings: settings.NewManager(db.DB()),
		queries:  saved.New(db.DB()),
		creds:    creds,
	}, creds
}

func TestUpdateProfileKeepsTheTokenAndQueriesUnlessANewTokenIsTyped(t *testing.T) {
	a, creds := profileApp(t)
	p, err := a.CreateProfile("Old Jira", "https://jira.example.com", "first")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.SaveQuery(p.ID, "Open bugs", "type = Bug"); err != nil {
		t.Fatal(err)
	}
	got, err := a.UpdateProfile(p.ID, " Renamed ", "https://jira2.example.com", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Renamed" || got.JiraURL != "https://jira2.example.com" {
		t.Fatalf("updated to %+v", got)
	}
	if creds[p.ID] != "first" {
		t.Fatalf("token after an edit without one = %q", creds[p.ID])
	}
	if qs, _ := a.ListQueries(p.ID); len(qs) != 1 {
		t.Fatalf("queries after an edit: %d", len(qs))
	}
	if _, err := a.UpdateProfile(p.ID, "Renamed", "https://jira2.example.com", " second "); err != nil {
		t.Fatal(err)
	}
	if creds[p.ID] != "second" {
		t.Fatalf("token after typing one = %q", creds[p.ID])
	}
	if _, err := a.UpdateProfile(p.ID, "", "https://jira2.example.com", ""); err == nil {
		t.Fatal("a profile was renamed to nothing")
	}
}

func TestUpdateProfileAsksForATokenWhenNoneIsStored(t *testing.T) {
	a, _ := profileApp(t)
	p, err := a.CreateProfile("Demo", "demo", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.UpdateProfile(p.ID, "Demo", "https://jira.example.com", ""); err == nil {
		t.Fatal("a demo profile moved to a real Jira without a token")
	}
}

func TestDefaultProfileIsStored(t *testing.T) {
	a, _ := profileApp(t)
	if err := a.SetDefaultProfile("p1"); err != nil {
		t.Fatal(err)
	}
	if got := a.DefaultProfile(); got != "p1" {
		t.Fatalf("default = %q", got)
	}
}

func TestAProfileFileCarriesNoTokenAndReadsBack(t *testing.T) {
	data, err := profileFile(
		profile.Profile{Name: "Old Jira", JiraURL: "https://jira.example.com"},
		profile.ConfluenceConfig{BaseURL: "https://wiki.example.com", SpaceKey: "QA", RootPageID: "42"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(data)), "token") {
		t.Fatalf("the file names a token: %s", data)
	}
	cfg, err := readProfileFile(data)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "Old Jira" || cfg.JiraURL != "https://jira.example.com" || cfg.ConfluenceSpace != "QA" || cfg.ConfluenceRootPageID != "42" {
		t.Fatalf("read back %+v", cfg)
	}
	for _, bad := range []string{`not json`, `{"name":"","jiraUrl":"https://x"}`, `{"name":"x","jiraUrl":"ftp://x"}`} {
		if _, err := readProfileFile([]byte(bad)); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
}
