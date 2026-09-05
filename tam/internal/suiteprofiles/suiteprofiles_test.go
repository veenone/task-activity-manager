package suiteprofiles_test

import (
	"testing"

	"agile-suite/core/profile"
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
