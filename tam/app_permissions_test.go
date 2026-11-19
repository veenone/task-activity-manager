package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/importer"
	"agile-suite/tam/internal/permission"
)

// seedPermissionApp is an app holding one cached issue in PLAT, with the
// given permissions stored as Jira's answer for that project.
func seedPermissionApp(t *testing.T, perms map[string]bool) (*App, string) {
	t.Helper()
	a := newTestApp(t)
	p := newTestProfile(t, a)
	ctx := context.Background()
	rows := []backend.Issue{{Key: "PLAT-1", ID: "1", Project: "PLAT", Type: backend.TypeTask, Summary: "one", Status: "To Do", Updated: "2026-09-01T00:00:00Z"}}
	if err := a.repo.UpsertPage(ctx, p.ID, rows, time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := a.repo.PutPermissions(ctx, p.ID, "PLAT", perms); err != nil {
		t.Fatalf("seed permissions: %v", err)
	}
	return a, p.ID
}

func TestGetPermissionsAnswersTheProjectTheStoredAnswerAndTheNames(t *testing.T) {
	a, id := seedPermissionApp(t, map[string]bool{permission.ManageSprints: false})
	got, err := a.GetPermissions(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Project != "PLAT" || got.ByProject["PLAT"][permission.ManageSprints] || got.Names[permission.ManageSprints] != "Manage Sprints" {
		t.Errorf("GetPermissions = %+v", got)
	}
}

// Every binding behind a gated action refuses, naming the permission, when
// Jira has said the account lacks it, and writes nothing to the journal.
func TestBindingsRefuseAnActionWhosePermissionIsDenied(t *testing.T) {
	denied := map[string]bool{}
	for key := range permission.Names {
		denied[key] = false
	}
	a, id := seedPermissionApp(t, denied)
	file := writeFile(t, "shot.png", "12345")

	cases := []struct {
		name, perm string
		call       func() error
	}{
		{"CreateSprint", permission.ManageSprints, func() error { _, err := a.CreateSprint(id, 1, "S1", "", "", ""); return err }},
		{"EditSprint", permission.ManageSprints, func() error { return a.EditSprint(id, 1, 7, "S1", "", "", "", false) }},
		{"DeleteSprint", permission.ManageSprints, func() error { return a.DeleteSprint(id, 1, 7) }},
		{"StartSprint", permission.ManageSprints, func() error { return a.StartSprint(id, 1, 7, "S1", "", "2026-11-02", "2026-11-16") }},
		{"CompleteSprint", permission.ManageSprints, func() error { return a.CompleteSprint(id, 1, 7, "") }},
		{"RankIssue", permission.ScheduleIssues, func() error { return a.RankIssue(id, "PLAT-1", "PLAT-2", true, 1) }},
		{"MoveIssueToSprint", permission.ScheduleIssues, func() error { return a.MoveIssueToSprint(id, "PLAT-1", "7") }},
		{"JournalSprintMoves", permission.ScheduleIssues, func() error { _, err := a.JournalSprintMoves(id, []string{"PLAT-1"}, "7"); return err }},
		{"MoveIssueToColumn", permission.TransitionIssues, func() error { return a.MoveIssueToColumn(id, "PLAT-1", "3") }},
		{"CreateIssue", permission.CreateIssues, func() error {
			_, err := a.CreateIssue(id, backend.IssueDraft{Type: backend.TypeTask, Summary: "new"})
			return err
		}},
		{"ImportIssues", permission.CreateIssues, func() error {
			_, err := a.ImportIssues(id, "", false, "f.csv", importer.Mapping{}, false)
			return err
		}},
		{"EditIssue summary", permission.EditIssues, func() error { return a.EditIssue(id, "PLAT-1", "summary", "changed") }},
		{"EditIssue assignee", permission.AssignIssues, func() error { return a.EditIssue(id, "PLAT-1", "assignee", "jdoe") }},
		{"AddLink", permission.LinkIssues, func() error { return a.AddLink(id, "PLAT-1", backend.LinkDraft{}) }},
		{"LogWork", permission.WorkOnIssues, func() error { return a.LogWork(id, "PLAT-1", "1h", "", "") }},
		{"AttachFiles", permission.CreateAttachments, func() error { return a.AttachFiles(id, "PLAT-1", []string{file}) }},
	}
	for _, c := range cases {
		err := c.call()
		if err == nil || !strings.Contains(err.Error(), permission.Names[c.perm]) {
			t.Errorf("%s = %v, want a refusal naming %q", c.name, err, permission.Names[c.perm])
		}
	}
	pending, err := a.ListPendingChanges(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Errorf("%d change(s) were journalled by refused actions: %+v", len(pending), pending)
	}
}

// A permission Jira did not report, and an issue in a project no sync asked
// about, are unknown, and unknown does not block.
func TestBindingsAllowAnActionWhosePermissionIsUnknown(t *testing.T) {
	a, id := seedPermissionApp(t, map[string]bool{permission.ManageSprints: false})
	if err := a.EditIssue(id, "PLAT-1", "summary", "changed"); err != nil {
		t.Errorf("EditIssue with Edit Issues unreported = %v, want it journalled", err)
	}
	if err := a.LogWork(id, "PLAT-1", "1h", "", ""); err != nil {
		t.Errorf("LogWork with Work On Issues unreported = %v, want it journalled", err)
	}
	pending, err := a.ListPendingChanges(id)
	if err != nil || len(pending) != 2 {
		t.Errorf("pending = %d (%v), want the two allowed changes", len(pending), err)
	}
}

// A dry-run import writes nothing, so it stays open without Create Issues.
func TestADryRunImportIsNotRefusedForCreateIssues(t *testing.T) {
	a, id := seedPermissionApp(t, map[string]bool{permission.CreateIssues: false})
	_, err := a.ImportIssues(id, "", false, "f.csv", importer.Mapping{}, true)
	if err != nil && strings.Contains(err.Error(), "Create Issues") {
		t.Errorf("dry run refused for the permission: %v", err)
	}
}
