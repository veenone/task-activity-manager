package profile_test

import (
	"path/filepath"
	"testing"

	"xray-test-manager/internal/profile"
	"xray-test-manager/internal/store"
)

func newManager(t *testing.T) *profile.Manager {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return profile.NewManager(st)
}

func TestCreateProfileStoresScopeJQL(t *testing.T) {
	m := newManager(t)

	p, err := m.Create("QA", "https://jira.example.com", "QA", "labels = smoke")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.ScopeJQL != "labels = smoke" {
		t.Errorf("ScopeJQL = %q, want 'labels = smoke'", p.ScopeJQL)
	}

	got, err := m.Get(p.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ScopeJQL != "labels = smoke" {
		t.Errorf("persisted ScopeJQL = %q, want 'labels = smoke'", got.ScopeJQL)
	}
}

func TestUpdateScopeChangesJQL(t *testing.T) {
	m := newManager(t)
	p, err := m.Create("QA", "https://jira.example.com", "QA", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := m.UpdateScope(p.ID, "component = Login"); err != nil {
		t.Fatalf("update scope: %v", err)
	}

	got, _ := m.Get(p.ID)
	if got.ScopeJQL != "component = Login" {
		t.Errorf("ScopeJQL = %q, want 'component = Login'", got.ScopeJQL)
	}
}

func TestUpdateScopeUnknownProfileErrors(t *testing.T) {
	m := newManager(t)
	if err := m.UpdateScope("nope", "x"); err == nil {
		t.Error("updating an unknown profile's scope should error")
	}
}
