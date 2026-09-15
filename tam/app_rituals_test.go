package main

import (
	"testing"
	"time"

	"agile-suite/core/profile"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/ritualrepo"
)

// newTestAppWithRituals builds on the shared newTestApp/newTestProfile
// helpers (app_boards_test.go) and wires the ritual repository the way
// initStore does in production. Neither existing helper does this on its
// own: newTestApp replicates initStore's field assignments by hand and
// predates the ritual store, so a test exercising it has to finish that
// wiring itself rather than trust a helper that cannot know about a field
// it was written before.
func newTestAppWithRituals(t *testing.T) (*App, profile.Profile) {
	t.Helper()
	a := newTestApp(t)
	p := newTestProfile(t, a)
	a.rituals = ritualrepo.New(a.local.DB())
	return a, p
}

// seedSprintIssues puts four issues in a sprint so the defaults have something
// to choose between.
func seedSprintIssues(t *testing.T, a *App, profileID string, sprintID int) {
	t.Helper()
	issues := []backend.Issue{
		{Key: "PLAT-1", Project: "PLAT", Type: "story", Summary: "Checkout", Status: "To Do", SprintID: "12"},
		{Key: "PLAT-2", Project: "PLAT", Type: "story", Summary: "Payments", Status: "In Progress", SprintID: "12"},
		{Key: "PLAT-3", Project: "PLAT", Type: "bug", Summary: "Timeout", Status: "Done", SprintID: "12"},
		{Key: "PLAT-4", Project: "PLAT", Type: "story", Summary: "Search", Status: "Blocked", SprintID: "12"},
	}
	if err := a.repo.UpsertPage(a.ctx, profileID, issues, time.Now().UTC(), false); err != nil {
		t.Fatalf("seed issues: %v", err)
	}
}
