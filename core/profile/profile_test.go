package profile_test

import (
	"path/filepath"
	"testing"

	"agile-suite/core/profile"
	"agile-suite/core/shareddb"
)

func newManager(t *testing.T) *profile.Manager {
	t.Helper()
	st, err := shareddb.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return profile.NewManager(st.DB())
}

func TestCreateProfileStoresScopeJQL(t *testing.T) {
	m := newManager(t)

	p, err := m.Create("QA", "https://jira.example.com", "QA", "labels = smoke", "Defect", "", "", "", false, "")
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

func TestDeleteProfilePurgesConfluenceRowsOnlyForProfile(t *testing.T) {
	st, err := shareddb.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	m := profile.NewManager(st.DB())
	one, err := m.Create("One", "https://jira.one", "ONE", "", "", "", "", "", false, "")
	if err != nil {
		t.Fatalf("create one: %v", err)
	}
	two, err := m.Create("Two", "https://jira.two", "TWO", "", "", "", "", "", false, "")
	if err != nil {
		t.Fatalf("create two: %v", err)
	}
	if _, err := st.DB().Exec(`INSERT INTO confluence_profile(profile_id, base_url) VALUES(?, ?)`, one.ID, "https://confluence.one"); err != nil {
		t.Fatalf("seed profile config: %v", err)
	}
	if _, err := st.DB().Exec(`INSERT INTO confluence_profile(profile_id, base_url) VALUES(?, ?)`, two.ID, "https://confluence.two"); err != nil {
		t.Fatalf("seed other config: %v", err)
	}
	if _, err := st.DB().Exec(`INSERT INTO confluence_association(profile_id, ritual_type, page_id) VALUES(?, ?, ?)`, one.ID, "standup", "p1"); err != nil {
		t.Fatalf("seed association: %v", err)
	}
	if _, err := st.DB().Exec(`INSERT INTO confluence_association(profile_id, ritual_type, page_id) VALUES(?, ?, ?)`, two.ID, "retro", "p2"); err != nil {
		t.Fatalf("seed other association: %v", err)
	}
	if _, err := st.DB().Exec(`INSERT INTO confluence_page_cache(profile_id, page_id, fetched_at, payload) VALUES(?, ?, ?, ?)`, one.ID, "p1", "now", "one"); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	if _, err := st.DB().Exec(`INSERT INTO confluence_page_cache(profile_id, page_id, fetched_at, payload) VALUES(?, ?, ?, ?)`, two.ID, "p2", "now", "two"); err != nil {
		t.Fatalf("seed other cache: %v", err)
	}
	if err := m.Delete(one.ID); err != nil {
		t.Fatalf("delete one: %v", err)
	}
	for _, table := range []string{"confluence_profile", "confluence_association", "confluence_page_cache"} {
		var n int
		if err := st.DB().QueryRow("SELECT COUNT(*) FROM "+table+" WHERE profile_id = ?", one.ID).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if n != 0 {
			t.Errorf("%s rows for deleted profile = %d, want 0", table, n)
		}
		if err := st.DB().QueryRow("SELECT COUNT(*) FROM "+table+" WHERE profile_id = ?", two.ID).Scan(&n); err != nil {
			t.Fatalf("count other %s: %v", table, err)
		}
		if n != 1 {
			t.Errorf("%s rows for other profile = %d, want 1", table, n)
		}
	}
}

func TestConfluenceConfigRoundTripsWithoutCredential(t *testing.T) {
	m := newManager(t)
	p, err := m.Create("QA", "https://jira.example.com", "QA", "", "", "", "", "", false, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	want := profile.ConfluenceConfig{BaseURL: "https://confluence.example.com/", SpaceKey: "ENG", RootPageID: "42"}
	if err := m.SetConfluenceConfig(p.ID, want); err != nil {
		t.Fatalf("set Confluence config: %v", err)
	}
	got, err := m.ConfluenceConfig(p.ID)
	if err != nil {
		t.Fatalf("get Confluence config: %v", err)
	}
	if got.BaseURL != "https://confluence.example.com" || got.SpaceKey != want.SpaceKey || got.RootPageID != want.RootPageID {
		t.Fatalf("config = %+v", got)
	}
}

func TestBugIssueTypeDefaultsAndPersists(t *testing.T) {
	m := newManager(t)

	// A blank issue type defaults to "Bug".
	def, err := m.Create("Prod", "https://jira.example.com", "PROJ", "", "", "", "", "", false, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if def.BugIssueType != "Bug" {
		t.Errorf("default BugIssueType = %q, want 'Bug'", def.BugIssueType)
	}

	// A configured issue type persists, and Update can change it.
	got, err := m.Create("Stg", "https://jira.example.com", "STG", "", "Defect", "", "", "", false, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if reread, _ := m.Get(got.ID); reread.BugIssueType != "Defect" {
		t.Errorf("persisted BugIssueType = %q, want 'Defect'", reread.BugIssueType)
	}
	if err := m.Update(got.ID, "Stg", "https://jira.example.com", "STG", "", "Incident", "", "", "", false, ""); err != nil {
		t.Fatalf("update: %v", err)
	}
	if reread, _ := m.Get(got.ID); reread.BugIssueType != "Incident" {
		t.Errorf("after update BugIssueType = %q, want 'Incident'", reread.BugIssueType)
	}
}

func TestBugProjectModeDefaultsAndPersists(t *testing.T) {
	m := newManager(t)

	// A blank mode defaults to "test".
	def, err := m.Create("Prod", "https://jira.example.com", "PROJ", "", "", "", "", "", false, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if def.BugProjectMode != "test" {
		t.Errorf("default BugProjectMode = %q, want 'test'", def.BugProjectMode)
	}

	// An unknown mode is normalised to "test".
	bad, _ := m.Create("Bad", "https://jira.example.com", "BAD", "", "", "garbage", "", "", false, "")
	if bad.BugProjectMode != "test" {
		t.Errorf("unknown mode = %q, want 'test'", bad.BugProjectMode)
	}

	// Dedicated mode + key persist, and Update can change them.
	got, err := m.Create("Stg", "https://jira.example.com", "STG", "", "", "dedicated", "DEFECTS", "", false, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if reread, _ := m.Get(got.ID); reread.BugProjectMode != "dedicated" || reread.BugProjectKey != "DEFECTS" {
		t.Errorf("persisted bug project = (%q, %q), want (dedicated, DEFECTS)",
			reread.BugProjectMode, reread.BugProjectKey)
	}
	if err := m.Update(got.ID, "Stg", "https://jira.example.com", "STG", "", "", "execution", "", "", false, ""); err != nil {
		t.Fatalf("update: %v", err)
	}
	if reread, _ := m.Get(got.ID); reread.BugProjectMode != "execution" {
		t.Errorf("after update BugProjectMode = %q, want 'execution'", reread.BugProjectMode)
	}
}

func TestUpdateScopeChangesJQL(t *testing.T) {
	m := newManager(t)
	p, err := m.Create("QA", "https://jira.example.com", "QA", "", "", "", "", "", false, "")
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

// TestBackendDefaultsAndPersists verifies a blank backend reads as "xray"
// (back-compat) and a "kiwi" backend round-trips through Create, Get, List,
// and Update.
func TestBackendDefaultsAndPersists(t *testing.T) {
	m := newManager(t)

	// A blank backend defaults to "xray".
	def, err := m.Create("Prod", "https://jira.example.com", "PROJ", "", "", "", "", "", false, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if def.Backend != "xray" {
		t.Errorf("default Backend = %q, want 'xray'", def.Backend)
	}
	if reread, err := m.Get(def.ID); err != nil || reread.Backend != "xray" {
		t.Errorf("persisted default Backend = %q (err %v), want 'xray'", reread.Backend, err)
	}

	// An unrecognized value normalises to "xray" too.
	bad, err := m.Create("Bad", "https://jira.example.com", "BAD", "", "", "", "", "", false, "bogus")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if bad.Backend != "xray" {
		t.Errorf("unknown Backend = %q, want 'xray'", bad.Backend)
	}

	// A "kiwi" backend persists through Create, Get, and List, and Update can
	// change it back to "xray".
	got, err := m.Create("Lab", "https://kiwi.example.com", "LAB", "", "", "", "", "", false, "kiwi")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got.Backend != "kiwi" {
		t.Errorf("Backend = %q, want 'kiwi'", got.Backend)
	}
	reread, err := m.Get(got.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if reread.Backend != "kiwi" {
		t.Errorf("persisted Backend = %q, want 'kiwi'", reread.Backend)
	}

	all, err := m.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, p := range all {
		if p.ID == got.ID {
			found = true
			if p.Backend != "kiwi" {
				t.Errorf("listed Backend = %q, want 'kiwi'", p.Backend)
			}
		}
	}
	if !found {
		t.Fatalf("created profile %s not found in List", got.ID)
	}

	if err := m.Update(got.ID, "Lab", "https://kiwi.example.com", "LAB", "", "", "", "", "", false, "xray"); err != nil {
		t.Fatalf("update: %v", err)
	}
	if reread, _ := m.Get(got.ID); reread.Backend != "xray" {
		t.Errorf("after update Backend = %q, want 'xray'", reread.Backend)
	}
}

// TestCreateCarriesTLSSettings pins the fields a cloned profile must not lose.
// A profile created from another one on the same server needs that server's
// certificate settings, or it is created in a state that cannot connect.
func TestCreateCarriesTLSSettings(t *testing.T) {
	m := newManager(t)

	const pem = "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----"
	p, err := m.Create("Internal", "https://jira.internal", "PROJ", "", "Bug", "test", "",
		pem, true, "xray")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.CACert != pem {
		t.Errorf("CACert = %q, want the certificate it was created with", p.CACert)
	}
	if !p.AllowUntrustedTLS {
		t.Error("AllowUntrustedTLS = false, want true")
	}

	// And it survives a round trip through the store.
	got, err := m.Get(p.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.CACert != pem || !got.AllowUntrustedTLS {
		t.Errorf("after reload: CACert=%q AllowUntrustedTLS=%v", got.CACert, got.AllowUntrustedTLS)
	}
}
