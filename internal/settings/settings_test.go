package settings_test

import (
	"path/filepath"
	"testing"

	"xray-test-manager/internal/settings"
	"xray-test-manager/internal/store"
)

func newManager(t *testing.T) *settings.Manager {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return settings.NewManager(st)
}

func TestDefaultSettingsAreEmpty(t *testing.T) {
	m := newManager(t)
	s, err := m.Get()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if s.DefaultProfileID != "" {
		t.Errorf("DefaultProfileID = %q, want empty by default", s.DefaultProfileID)
	}
}

func TestSetAndGetDefaultProfile(t *testing.T) {
	m := newManager(t)

	if err := m.SetDefaultProfileID("abc-123"); err != nil {
		t.Fatalf("set: %v", err)
	}

	s, _ := m.Get()
	if s.DefaultProfileID != "abc-123" {
		t.Errorf("DefaultProfileID = %q, want abc-123", s.DefaultProfileID)
	}

	// Overwrite (upsert).
	if err := m.SetDefaultProfileID("def-456"); err != nil {
		t.Fatalf("set 2: %v", err)
	}
	s, _ = m.Get()
	if s.DefaultProfileID != "def-456" {
		t.Errorf("DefaultProfileID = %q, want def-456 after overwrite", s.DefaultProfileID)
	}

	// Theme persists independently of the default profile.
	if err := m.SetTheme("dark"); err != nil {
		t.Fatalf("set theme: %v", err)
	}
	s, _ = m.Get()
	if s.Theme != "dark" || s.DefaultProfileID != "def-456" {
		t.Errorf("settings = %+v, want theme dark + default def-456", s)
	}
}
