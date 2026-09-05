package settings_test

import "testing"

func TestScopeKeysDoNotCollideAcrossApps(t *testing.T) {
	m := newManager(t)
	tam, err := m.Scope("tam")
	if err != nil {
		t.Fatalf("scope tam: %v", err)
	}
	xtm, err := m.Scope("xtm")
	if err != nil {
		t.Fatalf("scope xtm: %v", err)
	}
	if err := tam.Set("tour_seen_version", "3"); err != nil {
		t.Fatalf("tam set: %v", err)
	}
	if err := xtm.Set("tour_seen_version", "1"); err != nil {
		t.Fatalf("xtm set: %v", err)
	}
	if got, _ := tam.Get("tour_seen_version"); got != "3" {
		t.Errorf("tam value = %q, want 3", got)
	}
	if got, _ := xtm.Get("tour_seen_version"); got != "1" {
		t.Errorf("xtm value = %q, want 1", got)
	}
	// The unprefixed key XTM's Manager reads today is untouched.
	s, err := m.Get()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if s.TourSeenVersion != 0 {
		t.Errorf("unprefixed tour_seen_version = %d, want 0", s.TourSeenVersion)
	}
}

func TestScopeUnsetKeyReadsEmpty(t *testing.T) {
	m := newManager(t)
	sc, err := m.Scope("tam")
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	got, err := sc.Get("missing")
	if err != nil || got != "" {
		t.Errorf("Get(missing) = %q, %v; want \"\", nil", got, err)
	}
}

func TestScopeRejectsBadAppIDs(t *testing.T) {
	m := newManager(t)
	for _, app := range []string{"", "a.b", "."} {
		if _, err := m.Scope(app); err == nil {
			t.Errorf("Scope(%q) accepted, want error", app)
		}
	}
}
