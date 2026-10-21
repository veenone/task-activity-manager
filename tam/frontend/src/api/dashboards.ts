// Dashboards pinned from saved Jira filters, and the backlog export.

import * as App from "../../wailsjs/go/main/App";

// A dashboard is a saved Jira filter pinned to this profile, with the last
// figures it answered with. The figures are a snapshot rather than a live
// query because a dashboard has to say what it counts while Jira is
// unreachable, and say the same thing a week later when it is reopened.
export interface DashboardBucket {
  name: string;
  count: number;
}

export interface DashboardSnapshot {
  total: number;
  points: number;
  estimateSeconds: number;
  spentSeconds: number;
  byStatus: DashboardBucket[];
  byType: DashboardBucket[];
  byAssignee: DashboardBucket[];
  // The filter matched more than one refresh counts, so these figures
  // describe what was read and not the whole filter.
  capped: boolean;
}

export interface Dashboard {
  id: string;
  name: string;
  // The saved filter this is pinned to, empty for one built from typed JQL.
  filterId: string;
  jql: string;
  // When the figures were taken, empty for a dashboard nobody has
  // refreshed. The view prints it: a number with no date on it reads as
  // today's.
  refreshedAt: string;
  snapshot: DashboardSnapshot;
}

// JiraFilter is one saved filter as Jira holds it, with the JQL it stands
// for, so a dashboard can keep the words rather than the id alone.
export interface JiraFilter {
  id: string;
  name: string;
  jql: string;
}

export const ListJiraFilters: (profileId: string) => Promise<JiraFilter[]> =
  App.ListJiraFilters as unknown as (profileId: string) => Promise<JiraFilter[]>;
export const ListDashboards: (profileId: string) => Promise<Dashboard[]> =
  App.ListDashboards as unknown as (profileId: string) => Promise<Dashboard[]>;
// CreateDashboard pins a filter and fills it in, so a new dashboard opens
// with its figures rather than waiting to be refreshed.
export const CreateDashboard: (profileId: string, name: string, filterId: string, jql: string) => Promise<Dashboard> =
  App.CreateDashboard as unknown as (profileId: string, name: string, filterId: string, jql: string) => Promise<Dashboard>;
export const RefreshDashboard: (profileId: string, id: string) => Promise<Dashboard> =
  App.RefreshDashboard as unknown as (profileId: string, id: string) => Promise<Dashboard>;
export const DeleteDashboard: (profileId: string, id: string) => Promise<void> = App.DeleteDashboard;
