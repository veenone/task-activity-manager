package main

// The routing half of the bug-connection feature: which backend a bug call
// reaches, what browse base a bug key opens against, and what happens when
// nothing is routed. The connection's own lifecycle (save, get, delete, and
// what the credential store does when it fails) is in
// app_bugconnection_test.go, which shares this package's newTestAppWithKiwiProfile.

import (
	"testing"

	"agile-suite/xtm/internal/backend"
)

// TestBugBackendForCarriesTheConnectionSettings checks the backend that comes
// back is bound to the BUG connection, not the profile's own. A profile whose
// tests live in Kiwi must not end up filing its bugs into Kiwi.
func TestBugBackendForCarriesTheConnectionSettings(t *testing.T) {
	a, profileID := newTestAppWithKiwiProfile(t)

	if _, err := a.SaveBugConnection(profileID, "https://jira.example.com", "DEF", "Bug", "tok", "", false); err != nil {
		t.Fatalf("save bug connection: %v", err)
	}
	b, err := a.bugBackendFor(profileID)
	if err != nil {
		t.Fatalf("bugBackendFor: %v", err)
	}
	if b == nil {
		t.Fatal("no bug backend returned after configuring one")
	}
	if name := b.Capabilities().Name; name != "xray" {
		t.Errorf("bug backend is %q, want xray: bugs are filed into Jira", name)
	}
}

// TestBugBrowseBaseForReturnsTheConnectionURLPlusBrowse pins bugBrowseBaseFor
// itself: the browse prefix is the connection's URL with exactly one
// trailing slash then "browse/" appended. (Formerly misnamed
// "TestPrimaryBackendLearnsTheBugBrowseBase" — no primary backend appears
// here; that wiring is covered separately by
// TestBugRoutingOptionsWiresTheBugBackendAndBrowseBaseIntoThePrimary below.)
func TestBugBrowseBaseForReturnsTheConnectionURLPlusBrowse(t *testing.T) {
	a, profileID := newTestAppWithKiwiProfile(t)

	if _, err := a.SaveBugConnection(profileID, "https://jira.example.com", "DEF", "Bug", "tok", "", false); err != nil {
		t.Fatalf("save bug connection: %v", err)
	}
	base, err := a.bugBrowseBaseFor(profileID)
	if err != nil {
		t.Fatalf("bugBrowseBaseFor: %v", err)
	}
	if base != "https://jira.example.com/browse/" {
		t.Errorf("browse base = %q, want the connection URL plus /browse/", base)
	}
}

// TestBugBrowseBaseForTrimsExactlyOneTrailingSlash pins the Jira DC
// context-path case alongside the bare-host ones: a base URL that already
// ends in a path segment (a context path like /jira) must keep that segment,
// with or without its own trailing slash.
func TestBugBrowseBaseForTrimsExactlyOneTrailingSlash(t *testing.T) {
	cases := []struct{ url, want string }{
		{"https://h", "https://h/browse/"},
		{"https://h/", "https://h/browse/"},
		{"https://h/jira", "https://h/jira/browse/"},
		{"https://h/jira/", "https://h/jira/browse/"},
	}
	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			a, profileID := newTestAppWithKiwiProfile(t)
			if _, err := a.SaveBugConnection(profileID, tc.url, "DEF", "Bug", "tok", "", false); err != nil {
				t.Fatalf("save bug connection: %v", err)
			}
			got, err := a.bugBrowseBaseFor(profileID)
			if err != nil {
				t.Fatalf("bugBrowseBaseFor: %v", err)
			}
			if got != tc.want {
				t.Errorf("bugBrowseBaseFor(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}

// fakePrimaryWithBrowseBase is a minimal backend.Backend fake (the embedded
// interface is left nil; bugRoutingOptions never calls any method on it other
// than the SetBugBrowseBase type assertion) used to prove bugRoutingOptions
// actually reaches the PRIMARY backend, not just the returned option slice.
type fakePrimaryWithBrowseBase struct {
	backend.Backend
	gotBase string
}

func (f *fakePrimaryWithBrowseBase) SetBugBrowseBase(base string) { f.gotBase = base }

// TestBugRoutingOptionsWiresTheBugBackendAndBrowseBaseIntoThePrimary is the
// wiring assertion this task exists to build: a routed profile must yield
// exactly one syncer.Option, and the PRIMARY backend must learn the bug
// tracker's browse URL through SetBugBrowseBase. Deleting that type assertion
// in bugRoutingOptions leaves every other bug-routing test in this file
// passing — this is the one that catches it.
func TestBugRoutingOptionsWiresTheBugBackendAndBrowseBaseIntoThePrimary(t *testing.T) {
	a, profileID := newTestAppWithKiwiProfile(t)

	if _, err := a.SaveBugConnection(profileID, "https://jira.example.com", "DEF", "Bug", "tok", "", false); err != nil {
		t.Fatalf("save bug connection: %v", err)
	}

	primary := &fakePrimaryWithBrowseBase{}
	opts := a.bugRoutingOptions(profileID, primary)
	if len(opts) != 1 {
		t.Fatalf("bugRoutingOptions returned %d option(s), want exactly 1 for a routed profile", len(opts))
	}
	if primary.gotBase != "https://jira.example.com/browse/" {
		t.Errorf("primary.SetBugBrowseBase got %q, want the connection URL plus /browse/", primary.gotBase)
	}
}

// TestCreateBugForTestRoutesToTheBugConnectionsProjectAndIssueType is the
// CRITICAL regression this round exists to close: a Kiwi profile's defects
// must land in the BUG CONNECTION's Jira project under its issue type, never
// in a project named after the Kiwi product configured on the profile itself
// (which has no corresponding Jira project to create an issue in).
func TestCreateBugForTestRoutesToTheBugConnectionsProjectAndIssueType(t *testing.T) {
	a, profileID := newTestAppWithKiwiProfile(t)

	if _, err := a.SaveBugConnection(profileID, "https://jira.example.com", "DEF", "Defect", "tok", "", false); err != nil {
		t.Fatalf("save bug connection: %v", err)
	}

	if _, err := a.CreateBugForTest(profileID, "T-1", "", "Boom", "desc", "High", nil, nil); err != nil {
		t.Fatalf("create bug for test: %v", err)
	}

	bugs, err := a.ListBugsWithTests(profileID)
	if err != nil {
		t.Fatalf("list bugs: %v", err)
	}
	if len(bugs) != 1 {
		t.Fatalf("got %d queued bugs, want 1", len(bugs))
	}
	if bugs[0].ProjectKey != "DEF" {
		t.Errorf("queued bug project = %q, want the bug connection's project DEF, not the Kiwi profile's own project (SNMP)", bugs[0].ProjectKey)
	}
	if bugs[0].IssueType != "Defect" {
		t.Errorf("queued bug issue type = %q, want the bug connection's issue type Defect", bugs[0].IssueType)
	}
}

// TestCreateBugForTestUsesTheProfileWhenUnrouted is the regression guard for
// every Xray profile (and an unconfigured Kiwi one): without a bug
// connection, a queued bug still uses the profile's own project/issue type,
// exactly as before bug routing existed.
func TestCreateBugForTestUsesTheProfileWhenUnrouted(t *testing.T) {
	a, profileID := newTestAppWithKiwiProfile(t)

	if _, err := a.CreateBugForTest(profileID, "T-1", "", "Boom", "desc", "High", nil, nil); err != nil {
		t.Fatalf("create bug for test: %v", err)
	}

	bugs, err := a.ListBugsWithTests(profileID)
	if err != nil {
		t.Fatalf("list bugs: %v", err)
	}
	if len(bugs) != 1 {
		t.Fatalf("got %d queued bugs, want 1", len(bugs))
	}
	if bugs[0].ProjectKey != "SNMP" {
		t.Errorf("queued bug project = %q, want the profile's own project SNMP (no bug connection configured)", bugs[0].ProjectKey)
	}
}

// TestGetFieldDefsAsksTheBugConnectionsBackendWhenRouted proves
// GetFieldDefs asks the BUG backend, not the profile's own Kiwi one, for
// create-screen fields once routing is configured. The Kiwi adapter's
// GetFieldDefs always returns backend.ErrUnsupported (see
// internal/backend/kiwi/adapter.go), so a successful, non-empty result here
// is only possible if the call reached the bug connection's (demo) Jira
// backend instead.
func TestGetFieldDefsAsksTheBugConnectionsBackendWhenRouted(t *testing.T) {
	a, profileID := newTestAppWithKiwiProfile(t)

	if _, err := a.SaveBugConnection(profileID, "demo", "DEF", "Bug", "tok", "", false); err != nil {
		t.Fatalf("save bug connection: %v", err)
	}

	fields, err := a.GetFieldDefs(profileID)
	if err != nil {
		t.Fatalf("GetFieldDefs: %v (want the demo bug connection's fields, not Kiwi's ErrUnsupported)", err)
	}
	if len(fields) == 0 {
		t.Fatal("got no create fields; want the demo Jira bug connection's fields")
	}
}

// TestCreateBugForTestSucceedsWithoutAReadableCredential is the local-first
// regression this round exists to close: queuing a bug is a pending-change
// journal write, like every other mutating method in this app, so it must
// not require the credential store to be readable. Before this fix,
// CreateBugForTest called bugCreateTarget (via the shared resolver), which
// always called a.creds.Load and built a Backend even though the Backend was
// discarded — so a locked/unreadable keyring broke queuing a bug on the most
// widely used (unrouted, Xray) path too.
func TestCreateBugForTestSucceedsWithoutAReadableCredential(t *testing.T) {
	a, profileID := newTestAppWithKiwiProfile(t)

	if _, err := a.SaveBugConnection(profileID, "https://jira.example.com", "DEF", "Defect", "tok", "", false); err != nil {
		t.Fatalf("save bug connection: %v", err)
	}

	// Simulate the credential store becoming unreadable (a locked keyring)
	// after the bug connection was configured: swap in a fresh erroring store
	// whose Load fails for every id, including ones a working store already
	// saved a token under.
	a.creds = newErroringCredentialStore()

	if _, err := a.CreateBugForTest(profileID, "T-1", "", "Boom", "desc", "High", nil, nil); err != nil {
		t.Fatalf("CreateBugForTest with an unreadable credential store: %v (want it to succeed: queuing is a local journal write, not a remote call)", err)
	}

	bugs, err := a.ListBugsWithTests(profileID)
	if err != nil {
		t.Fatalf("list bugs: %v", err)
	}
	if len(bugs) != 1 {
		t.Fatalf("got %d queued bugs, want 1", len(bugs))
	}
	if bugs[0].ProjectKey != "DEF" || bugs[0].IssueType != "Defect" {
		t.Errorf("queued bug = %+v, want project DEF / issue type Defect from the bug connection", bugs[0])
	}
}

// TestGetBugDetailAsksTheBugConnectionsBackendWhenRouted is the read-side twin
// of the create-side routing: on a routed profile the defect lives in the bug
// tracker, so asking the profile's own Kiwi backend can only ever answer
// ErrUnsupported (internal/backend/kiwi/adapter.go). A non-empty result here
// is only possible if the call reached the bug connection's (demo) Jira
// backend instead.
func TestGetBugDetailAsksTheBugConnectionsBackendWhenRouted(t *testing.T) {
	a, profileID := newTestAppWithKiwiProfile(t)

	if _, err := a.SaveBugConnection(profileID, "demo", "DEF", "Bug", "tok", "", false); err != nil {
		t.Fatalf("save bug connection: %v", err)
	}

	detail, err := a.GetBugDetail(profileID, "DEF-42")
	if err != nil {
		t.Fatalf("GetBugDetail: %v (want the demo bug connection's detail, not Kiwi's ErrUnsupported)", err)
	}
	if detail.Description == "" {
		t.Error("got an empty detail; want the demo Jira bug connection's fields")
	}
}

// TestGetBugBrowseBaseIsEmptyWhenUnrouted keeps the frontend's fallback
// honest: an empty base means "use the profile's own URL", which is what every
// Xray profile must keep doing.
func TestGetBugBrowseBaseIsEmptyWhenUnrouted(t *testing.T) {
	a, profileID := newTestAppWithKiwiProfile(t)

	base, err := a.GetBugBrowseBase(profileID)
	if err != nil {
		t.Fatalf("GetBugBrowseBase: %v", err)
	}
	if base != "" {
		t.Errorf("browse base = %q, want empty for a profile with no bug connection", base)
	}
}

// TestGetBugBrowseBaseNamesTheBugTracker is what stops a bug key from opening
// a 404 on the Kiwi host: the browse prefix must come from the BUG
// connection's URL, never the profile's.
func TestGetBugBrowseBaseNamesTheBugTracker(t *testing.T) {
	a, profileID := newTestAppWithKiwiProfile(t)

	if _, err := a.SaveBugConnection(profileID, "https://jira.example.com", "DEF", "Bug", "tok", "", false); err != nil {
		t.Fatalf("save bug connection: %v", err)
	}
	base, err := a.GetBugBrowseBase(profileID)
	if err != nil {
		t.Fatalf("GetBugBrowseBase: %v", err)
	}
	if base != "https://jira.example.com/browse/" {
		t.Errorf("browse base = %q, want the bug connection's URL plus /browse/", base)
	}
}
