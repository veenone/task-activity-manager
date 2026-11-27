import * as App from "../../wailsjs/go/main/App";

// The Jira permission keys TAM gates an action on. They mirror the constants
// in tam/internal/permission, which is where the bindings check them.
export const PERM_MANAGE_SPRINTS = "MANAGE_SPRINTS_PERMISSION";
export const PERM_SCHEDULE_ISSUES = "SCHEDULE_ISSUES";
export const PERM_CREATE_ISSUES = "CREATE_ISSUES";
export const PERM_EDIT_ISSUES = "EDIT_ISSUES";
export const PERM_TRANSITION_ISSUES = "TRANSITION_ISSUES";
export const PERM_ASSIGN_ISSUES = "ASSIGN_ISSUES";
export const PERM_LINK_ISSUES = "LINK_ISSUES";
export const PERM_WORK_ON_ISSUES = "WORK_ON_ISSUES";
export const PERM_CREATE_ATTACHMENTS = "CREATE_ATTACHMENTS";
export const PERM_RESOLVE_ISSUES = "RESOLVE_ISSUES";
export const PERM_ADMINISTER_PROJECTS = "ADMINISTER_PROJECTS";

// PermissionView is what the last sync learned about the token: the
// profile's own project, Jira's answer per project and permission key, and
// each permission's name as Jira's permission scheme page shows it. A
// project or a key that is missing from byProject is unknown, not denied.
export interface PermissionView {
  project: string;
  byProject: Record<string, Record<string, boolean>>;
  names: Record<string, string>;
}

// GetPermissions reads the stored answer. It never reaches Jira.
export const GetPermissions: (profileId: string) => Promise<PermissionView> =
  App.GetPermissions as unknown as (profileId: string) => Promise<PermissionView>;
