package main

import (
	"log"

	"agile-suite/tam/internal/permission"
)

// PermissionView is what the frontend needs to decide whether to offer an
// action: the profile's own project, what Jira answered for each project a
// sync asked about, and the name of each permission for the text beside a
// disabled control.
type PermissionView struct {
	Project   string                     `json:"project"`
	ByProject map[string]map[string]bool `json:"byProject"`
	Names     map[string]string          `json:"names"`
}

// GetPermissions reads the permissions the last sync stored. It reads the
// cache only, so it answers offline. A profile no sync has asked about has
// an empty ByProject, which the frontend reads as everything allowed.
func (a *App) GetPermissions(profileID string) (PermissionView, error) {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return PermissionView{}, err
	}
	set, err := a.repo.Permissions(a.ctx, p.ID)
	if err != nil {
		return PermissionView{}, err
	}
	return PermissionView{Project: p.ProjectKey, ByProject: set, Names: permission.Names}, nil
}

// allow refuses an action Jira has said this account may not perform in the
// project. A permission set that cannot be read allows the action: the
// refusal is a courtesy ahead of Jira's own, and a failed local read is not
// grounds to block a write.
func (a *App) allow(profileID, project, perm string) error {
	set, err := a.repo.Permissions(a.ctx, profileID)
	if err != nil {
		log.Printf("tam: read the permissions of %s: %v", profileID, err)
		return nil
	}
	return set.Check(project, perm)
}

// allowOnIssue is allow for the project the issue key belongs to. A draft's
// temporary key names no project Jira knows, so it is always allowed.
func (a *App) allowOnIssue(profileID, key, perm string) error {
	return a.allow(profileID, permission.ProjectOf(key), perm)
}

// allowOnIssues is allowOnIssue over a selection, refusing on the first key
// whose project denies the permission.
func (a *App) allowOnIssues(profileID string, keys []string, perm string) error {
	for _, key := range keys {
		if err := a.allowOnIssue(profileID, key, perm); err != nil {
			return err
		}
	}
	return nil
}

// editPermission is the permission one field edit needs: changing who an
// issue is assigned to is its own permission in Jira, and everything else
// the detail panel edits is Edit Issues.
func editPermission(field string) string {
	if field == "assignee" {
		return permission.AssignIssues
	}
	return permission.EditIssues
}
