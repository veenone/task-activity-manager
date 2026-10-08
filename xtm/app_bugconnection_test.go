package main

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"agile-suite/core/connection"
	"agile-suite/core/profile"
)

// newTestAppWithKiwiProfile builds an App (via newTestApp, the shared
// temp-store + in-memory-credential-store helper in app_connection_test.go)
// with one saved Kiwi profile, and returns the profile id.
func newTestAppWithKiwiProfile(t *testing.T) (*App, string) {
	t.Helper()
	a := newTestApp(t)
	p, err := a.CreateProfile("Kiwi", "https://kiwi.example.com", "SNMP", "", "", "", "", "u:p", "", false, "kiwi")
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	return a, p.ID
}

// TestSaveBugConnectionStoresTheCredentialOutsideTheDatabase is the security
// rule the whole feature inherits: a token reaches the OS credential manager
// and never a column. The bug connection is a SECOND credential, keyed by its
// own connection id, which is the rule connection.go already documents.
func TestSaveBugConnectionStoresTheCredentialOutsideTheDatabase(t *testing.T) {
	a, profileID := newTestAppWithKiwiProfile(t)

	c, err := a.SaveBugConnection(profileID, "https://jira.example.com", "DEF", "Bug", "s3cret", "", false)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if c.ID == profileID {
		t.Fatal("bug connection reused the profile id; it must have its own id and its own credential")
	}
	if c.Role != "bugs" {
		t.Errorf("role = %q, want bugs", c.Role)
	}

	tok, err := a.creds.Load(c.ID)
	if err != nil {
		t.Fatalf("load bug credential: %v", err)
	}
	if tok != "s3cret" {
		t.Errorf("stored credential = %q, want the token passed in", tok)
	}

	// The primary connection's credential is untouched.
	if _, err := a.creds.Load(profileID); err != nil {
		t.Errorf("primary credential disturbed: %v", err)
	}

	// The claim in this test's name: the token itself never reaches the
	// database. Scan the raw connection row rather than trusting the typed
	// Connection struct, which has no field the token could even go into.
	rows, err := a.store.DB().Query(`SELECT * FROM connection WHERE id = ?`, c.ID)
	if err != nil {
		t.Fatalf("query connection row: %v", err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatalf("columns: %v", err)
	}
	if !rows.Next() {
		t.Fatal("bug connection row not found in the database")
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		t.Fatalf("scan connection row: %v", err)
	}
	for i, v := range vals {
		var s string
		switch tv := v.(type) {
		case string:
			s = tv
		case []byte:
			s = string(tv)
		default:
			continue
		}
		if strings.Contains(s, "s3cret") {
			t.Errorf("column %q of the connection row contains the literal token: %q", cols[i], s)
		}
	}
}

// TestSaveBugConnectionIsIdempotent checks a second save edits the existing row
// rather than accumulating bug connections.
func TestSaveBugConnectionIsIdempotent(t *testing.T) {
	a, profileID := newTestAppWithKiwiProfile(t)

	first, err := a.SaveBugConnection(profileID, "https://jira.example.com", "DEF", "Bug", "tok", "", false)
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	second, err := a.SaveBugConnection(profileID, "https://jira2.example.com", "OPS", "Defect", "tok2", "", false)
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("second save created a new row (%s then %s)", first.ID, second.ID)
	}
	got, err := a.GetBugConnection(profileID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.URL != "https://jira2.example.com" || got.ProjectKey != "OPS" || got.BugIssueType != "Defect" {
		t.Errorf("got = %+v, want the second save's values", got)
	}
}

// TestGetBugConnectionIsEmptyWhenUnconfigured keeps the frontend simple: an
// unconfigured profile is not an error condition.
func TestGetBugConnectionIsEmptyWhenUnconfigured(t *testing.T) {
	a, profileID := newTestAppWithKiwiProfile(t)

	got, err := a.GetBugConnection(profileID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != "" {
		t.Errorf("got = %+v, want a zero Connection", got)
	}
}

// TestDeleteBugConnectionRemovesTheCredentialToo guards against a token
// outliving the configuration that justified storing it.
func TestDeleteBugConnectionRemovesTheCredentialToo(t *testing.T) {
	a, profileID := newTestAppWithKiwiProfile(t)

	c, err := a.SaveBugConnection(profileID, "https://jira.example.com", "DEF", "Bug", "tok", "", false)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := a.DeleteBugConnection(profileID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := a.connections.ByRole(profileID, "bugs"); !errors.Is(err, connection.ErrNotFound) {
		t.Errorf("bug connection still present after delete: %v", err)
	}
	if tok, err := a.creds.Load(c.ID); err == nil && tok != "" {
		t.Error("bug credential survived the delete")
	}
}

// TestDeleteProfileRemovesTheBugConnection is the cleanup path. The primary
// connection's credential is keyed by the profile id, so DeleteProfile already
// removed it; the bug connection has its own id and was being left behind.
func TestDeleteProfileRemovesTheBugConnection(t *testing.T) {
	a, profileID := newTestAppWithKiwiProfile(t)

	c, err := a.SaveBugConnection(profileID, "https://jira.example.com", "DEF", "Bug", "tok", "", false)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := a.DeleteProfile(profileID); err != nil {
		t.Fatalf("delete profile: %v", err)
	}
	if _, err := a.connections.Get(c.ID); !errors.Is(err, connection.ErrNotFound) {
		t.Errorf("bug connection row survived profile delete: %v", err)
	}
	if tok, err := a.creds.Load(c.ID); err == nil && tok != "" {
		t.Error("bug credential survived profile delete")
	}
}

// TestBugBackendForIsNilWhenUnconfigured pins the routing default. Every
// engine that takes a bug backend must behave exactly as before when this
// returns nil, which is what keeps Xray profiles untouched.
func TestBugBackendForIsNilWhenUnconfigured(t *testing.T) {
	a, profileID := newTestAppWithKiwiProfile(t)

	b, err := a.bugBackendFor(profileID)
	if err != nil {
		t.Fatalf("bugBackendFor: %v", err)
	}
	if b != nil {
		t.Errorf("got %T, want nil for a profile with no bug connection", b)
	}
}

// erroringCredentialStore is a profile.CredentialStore fake whose Load
// returns an error for an id it has never Saved — matching the real
// Windows Credential Manager and OS-keyring stores (credentials_windows.go,
// credentials_nonwindows.go), which both fail on an unknown id rather than
// returning ("", nil). memCredentialStore (app_connection_test.go) returns
// ("", nil) for a missing key instead, so it cannot exercise
// SaveBugConnection's blank-token guard: with memCredentialStore the guard's
// a.creds.Load(id) call always succeeds, even for a connection id nothing
// was ever saved under, which would make deleting the guard entirely pass
// every existing test. This fake is kept separate from memCredentialStore
// specifically so that store's behavior (relied on by other tests) is not
// changed.
type erroringCredentialStore struct {
	mu   sync.Mutex
	data map[string]string
}

func newErroringCredentialStore() *erroringCredentialStore {
	return &erroringCredentialStore{data: map[string]string{}}
}

func (e *erroringCredentialStore) Save(id, secret string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.data[id] = secret
	return nil
}

func (e *erroringCredentialStore) Load(id string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	v, ok := e.data[id]
	if !ok {
		return "", errors.New("load credential: element not found")
	}
	return v, nil
}

func (e *erroringCredentialStore) Delete(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.data, id)
	return nil
}

var _ profile.CredentialStore = (*erroringCredentialStore)(nil)

// TestSaveBugConnectionRejectsBlankTokenWithNoStoredCredential exercises the
// blank-token guard in SaveBugConnection against a credential store that
// fails closed like the real ones do. Without a stored bug credential to
// fall back to, a blank token must be rejected rather than silently creating
// a connection with no credential behind it.
func TestSaveBugConnectionRejectsBlankTokenWithNoStoredCredential(t *testing.T) {
	a := newTestApp(t)
	a.creds = newErroringCredentialStore()
	p, err := a.CreateProfile("Kiwi", "https://kiwi.example.com", "SNMP", "", "", "", "", "u:p", "", false, "kiwi")
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	if _, err := a.SaveBugConnection(p.ID, "https://jira.example.com", "DEF", "Bug", "" /* blank token */, "", false); err == nil {
		t.Fatal("SaveBugConnection with a blank token and no stored bug credential = nil error, want a rejection")
	}
	if _, err := a.connections.ByRole(p.ID, "bugs"); !errors.Is(err, connection.ErrNotFound) {
		t.Errorf("a rejected save still created a bug connection row: %v", err)
	}
}

// failingSaveCredentialStore delegates Load and Delete to an inner
// profile.CredentialStore but always fails Save, to simulate a credential
// store that accepts the connection row write but then fails to persist the
// secret (e.g. a locked keyring).
type failingSaveCredentialStore struct {
	inner profile.CredentialStore
}

func (f failingSaveCredentialStore) Save(id, secret string) error {
	return errors.New("simulated credential store failure")
}
func (f failingSaveCredentialStore) Load(id string) (string, error) { return f.inner.Load(id) }
func (f failingSaveCredentialStore) Delete(id string) error         { return f.inner.Delete(id) }

var _ profile.CredentialStore = failingSaveCredentialStore{}

// TestSaveBugConnectionRollsBackOrphanRowOnCreateCredentialFailure guards the
// window between creating the bug connection row and saving its credential:
// if the credential save fails on the CREATE path (no prior row, no prior
// credential), the just-created row must not be left behind. Left in place,
// GetCapabilities would report SupportsBugRouting = true off the row alone,
// and bugBackendFor would only fail later, when something actually tries to
// use it.
func TestSaveBugConnectionRollsBackOrphanRowOnCreateCredentialFailure(t *testing.T) {
	a, profileID := newTestAppWithKiwiProfile(t)
	a.creds = failingSaveCredentialStore{inner: a.creds}

	if _, err := a.SaveBugConnection(profileID, "https://jira.example.com", "DEF", "Bug", "tok", "", false); err == nil {
		t.Fatal("SaveBugConnection = nil error, want the simulated credential-store failure")
	}
	if _, err := a.connections.ByRole(profileID, "bugs"); !errors.Is(err, connection.ErrNotFound) {
		t.Errorf("orphan bug connection row survived a failed credential save: %v", err)
	}
}
