package main

import (
	"context"
	"strings"
	"testing"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
)

// unauthorizedBackend fails the connection test with the error Jira's client
// really returns, an *corejira.HTTPError, rather than a flat string. Humanize
// finds it with errors.As, so only the real type exercises the path.
type unauthorizedBackend struct {
	stubIssueBackend
}

func (unauthorizedBackend) TestConnection(context.Context) (backend.User, error) {
	return backend.User{}, &corejira.HTTPError{
		Method: "GET", Path: "/rest/api/2/myself", Code: 401,
		Status: "401 ", Message: "Unauthorized",
	}
}

// The status bar showed what Jira answered, verbatim: "jira: 401 : Unauthorized".
// XTM has turned that into a sentence since #170; TAM never did, and both now
// read it out of core (#315).
//
// Both of TAM's error surfaces are checked here, because they are shown in the
// same place and have to read the same: the error SyncIssues returns, which
// the status bar shows for the run that just failed, and sync_state.last_error,
// which it reads back on the next launch.
func TestFailedSyncReadsAsASentenceOnBothSurfaces(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	a.backends[p.ID] = unauthorizedBackend{}

	_, err := a.SyncIssues(p.ID, false)
	if err == nil {
		t.Fatal("a 401 from the connection test must fail the sync")
	}
	st, stErr := a.GetSyncState(p.ID)
	if stErr != nil {
		t.Fatalf("sync state: %v", stErr)
	}

	for _, s := range []struct{ what, text string }{
		{"the returned error", err.Error()},
		{"sync_state.last_error", st.LastError},
	} {
		if s.text == "" {
			t.Errorf("%s is empty", s.what)
			continue
		}
		for _, leak := range []string{"jira:", "401", "Unauthorized", "/rest/api/2/myself"} {
			if strings.Contains(s.text, leak) {
				t.Errorf("%s still carries %q: %s", s.what, leak, s.text)
			}
		}
		if !strings.Contains(strings.ToLower(s.text), "token") {
			t.Errorf("%s should say the token needs re-entering: %s", s.what, s.text)
		}
	}
}
