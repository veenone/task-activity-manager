// Package permission holds what TAM knows about the Jira project permissions
// of the signed-in token, and the rule for reading it: an action is refused
// only when Jira has said the permission is missing. A permission nobody has
// read, in a project no sync has asked about, is allowed, because Jira still
// refuses at Commit and a gap in TAM's knowledge must not lock a user out of
// something they hold.
package permission

import (
	"fmt"
	"strings"
)

// The Jira permission keys TAM gates an action on, as `mypermissions`
// names them.
const (
	ManageSprints     = "MANAGE_SPRINTS_PERMISSION"
	ScheduleIssues    = "SCHEDULE_ISSUES"
	CreateIssues      = "CREATE_ISSUES"
	EditIssues        = "EDIT_ISSUES"
	TransitionIssues  = "TRANSITION_ISSUES"
	AssignIssues      = "ASSIGN_ISSUES"
	LinkIssues        = "LINK_ISSUES"
	WorkOnIssues      = "WORK_ON_ISSUES"
	CreateAttachments = "CREATE_ATTACHMENTS"
	ResolveIssues     = "RESOLVE_ISSUES"
	// AdministerProjects is what Jira asks of anyone creating, editing,
	// releasing, archiving or deleting a fix version (#605).
	AdministerProjects = "ADMINISTER_PROJECTS"
)

// Names is each gated permission under the name Jira's permission scheme
// page shows, which is what a user asks an administrator for.
var Names = map[string]string{
	ManageSprints:      "Manage Sprints",
	ScheduleIssues:     "Schedule Issues",
	CreateIssues:       "Create Issues",
	EditIssues:         "Edit Issues",
	TransitionIssues:   "Transition Issues",
	AssignIssues:       "Assign Issues",
	LinkIssues:         "Link Issues",
	WorkOnIssues:       "Work On Issues",
	CreateAttachments:  "Create Attachments",
	ResolveIssues:      "Resolve Issues",
	AdministerProjects: "Administer Projects",
}

// Set is what Jira answered, by project key and then permission key. A
// project or a key that is absent is unknown, not denied.
type Set map[string]map[string]bool

// Gated narrows a full `mypermissions` answer to the keys TAM acts on, so
// the stored value does not grow with every permission an instance defines.
func Gated(all map[string]bool) map[string]bool {
	out := map[string]bool{}
	for key := range Names {
		if have, ok := all[key]; ok {
			out[key] = have
		}
	}
	return out
}

// Allows reports whether the action may go ahead: true unless Jira said the
// permission is missing in that project.
func (s Set) Allows(project, perm string) bool {
	have, known := s[project][perm]
	return !known || have
}

// Check is Allows as an error a binding can return, naming the permission
// and the project so the user knows what to ask for.
func (s Set) Check(project, perm string) error {
	if s.Allows(project, perm) {
		return nil
	}
	name := Names[perm]
	if name == "" {
		name = perm
	}
	return fmt.Errorf("this account does not have the Jira permission %q in %s", name, project)
}

// ProjectOf is the project key an issue key starts with: "PLAT" for
// "PLAT-431". A key with no dash has no project, which reads as unknown.
func ProjectOf(issueKey string) string {
	i := strings.LastIndex(issueKey, "-")
	if i <= 0 {
		return ""
	}
	return issueKey[:i]
}
