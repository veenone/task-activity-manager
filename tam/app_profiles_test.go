package main

import (
	"context"
	"testing"
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
