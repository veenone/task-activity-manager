package suiteprofiles_test

import (
	"path/filepath"
	"testing"

	"agile-suite/core/profile"
	"agile-suite/core/shareddb"
	"agile-suite/tam/internal/suiteprofiles"
)

func TestVisibleDropsKiwiProfiles(t *testing.T) {
	in := []profile.Profile{
		{ID: "a", Backend: "xray"},
		{ID: "b", Backend: "kiwi"},
		{ID: "c", Backend: "Kiwi"},
		{ID: "d", Backend: ""},
		{ID: "e", Backend: "jira"},
	}
	got := suiteprofiles.Visible(in)
	want := []string{"a", "d", "e"}
	if len(got) != len(want) {
		t.Fatalf("got %d profiles, want %d", len(got), len(want))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("got[%d] = %s, want %s", i, got[i].ID, id)
		}
	}
}

func TestBackendConstantStoresXray(t *testing.T) {
	db, err := shareddb.Open(filepath.Join(t.TempDir(), "profiles.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	m := profile.NewManager(db.DB())
	p, err := m.Create("Team", "https://jira.acme.example", "PLAT", "", "", "", "", "", false, suiteprofiles.Backend)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.Backend != "xray" {
		t.Errorf("Backend = %q, want %q", p.Backend, "xray")
	}

	visible := suiteprofiles.Visible([]profile.Profile{p})
	if len(visible) != 1 || visible[0].ID != p.ID {
		t.Errorf("Visible did not include the created profile: %+v", visible)
	}
}

func TestIsDemoURL(t *testing.T) {
	cases := map[string]bool{
		"demo":                      true,
		" DEMO ":                    true,
		"demo:pkcs":                 true,
		"demo-agile":                true,
		"https://jira.acme.example": false,
		"":                          false,
		"kiwi-demo":                 false,
	}
	for url, want := range cases {
		if got := suiteprofiles.IsDemoURL(url); got != want {
			t.Errorf("IsDemoURL(%q) = %v, want %v", url, got, want)
		}
	}
}

func TestValidateNew(t *testing.T) {
	if err := suiteprofiles.ValidateNew("Team", "demo", "DEMO", ""); err != nil {
		t.Errorf("demo profile without a token rejected: %v", err)
	}
	if err := suiteprofiles.ValidateNew("Team", "https://jira.acme.example", "PLAT", ""); err == nil {
		t.Error("live profile without a token accepted")
	}
	if err := suiteprofiles.ValidateNew("", "demo", "DEMO", ""); err == nil {
		t.Error("blank name accepted")
	}
	if err := suiteprofiles.ValidateNew("Team", "", "DEMO", "t"); err == nil {
		t.Error("blank URL accepted")
	}
	if err := suiteprofiles.ValidateNew("Team", "demo", "", "t"); err == nil {
		t.Error("blank project key accepted")
	}
}
