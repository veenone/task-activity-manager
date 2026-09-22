package main

import (
	"context"
	"testing"

	"agile-suite/tam/internal/backend"
)

// TestProfileConnectionStoresJiraUserSettings exercises the second write
// site named in the brief: verifying a saved profile's connection persists
// who "me" is, the same way a sync does.
func TestProfileConnectionStoresJiraUserSettings(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)

	name, err := a.TestProfileConnection(p.ID, "demo", "", false)
	if err != nil {
		t.Fatalf("test connection: %v", err)
	}
	if name != "Demo User" {
		t.Errorf("display name = %q, want %q", name, "Demo User")
	}

	username, err := a.repo.ProfileSetting(context.Background(), p.ID, "jira_username")
	if err != nil || username != "demo" {
		t.Errorf("jira_username = %q, %v, want %q", username, err, "demo")
	}
	displayName, err := a.repo.ProfileSetting(context.Background(), p.ID, "jira_display_name")
	if err != nil || displayName != "Demo User" {
		t.Errorf("jira_display_name = %q, %v, want %q", displayName, err, "Demo User")
	}
}

// TestChangingTheProjectKeyClearsTheBoardCacheToo pins the second half of
// the purge UpdateProfile promises. The issue cache was cleared and the
// board cache was not, which was survivable while every boards sync deleted
// and rewrote the board's membership on the way past. It stopped being
// survivable when the sync started reading that membership back and
// treating it as an answer: the old project's keys would be re-supplied for
// ever, and the sprint would report itself read while holding another
// project's cards.
func TestChangingTheProjectKeyClearsTheBoardCacheToo(t *testing.T) {
	a := newTestApp(t)
	p := newTestProfile(t, a)
	ctx := context.Background()

	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	sprints := []backend.Sprint{{ID: 11, BoardID: 1, Name: "Sprint 11", State: "closed", StartDate: "2026-08-04T09:00:00Z"}}
	keys := map[string][]string{"": {"PLAT-1"}, "11": {"PLAT-1"}}
	cols := []backend.BoardColumn{{Name: "Done", StatusIDs: []string{"5"}}}
	if err := a.boards.ReplaceBoard(ctx, p.ID, board, cols, sprints, keys); err != nil {
		t.Fatalf("seed the board cache: %v", err)
	}

	if _, err := a.UpdateProfile(p.ID, "Test", p.JiraURL, "OTHER", "", "", "", false); err != nil {
		t.Fatalf("update profile: %v", err)
	}

	listed, err := a.boards.ListBoards(ctx, p.ID)
	if err != nil {
		t.Fatalf("list boards: %v", err)
	}
	if len(listed) != 0 {
		t.Errorf("boards after a project change = %+v, want none: they belong to the old project", listed)
	}
	held, err := a.boards.SprintIssues(ctx, p.ID, 1, "11")
	if err != nil {
		t.Fatalf("sprint issues: %v", err)
	}
	if len(held) != 0 {
		t.Errorf("sprint 11 still holds %v, which are the old project's cards", held)
	}
	// And the read-once flag goes with them, or the next sync would take
	// the old project's silence for an answer about the new one.
	synced, err := a.boards.SyncedSprints(ctx, p.ID, 1)
	if err != nil {
		t.Fatalf("synced sprints: %v", err)
	}
	if len(synced) != 0 {
		t.Errorf("sprints still marked read = %v, want none after the project changed", synced)
	}
}
