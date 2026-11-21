import * as App from "../../wailsjs/go/main/App";
import type { Issue, Worklog } from "./issues";

// LoggedWork is one worklog with the issue it was logged on. summary is what
// Jira calls the issue, for a row the cache does not hold.
export interface LoggedWork {
  key: string;
  summary: string;
  log: Worklog;
}

// WorklogRow is one issue in the Worklog view's grid. assigned says it is the
// reader's and still open. cached says TAM holds it, which is what lets work
// be journalled against it: a row Jira knows and the cache does not is read
// only.
export interface WorklogRow {
  issue: Issue;
  assigned: boolean;
  cached: boolean;
}

export interface WorklogWeekView {
  rows: WorklogRow[];
  entries: LoggedWork[];
}

// SETTING_WORKLOG_DAY_HOURS is the profile setting that holds how many hours
// the reader means to log on a workday. Unset reads as DEFAULT_DAY_HOURS.
export const SETTING_WORKLOG_DAY_HOURS = "worklog_day_hours";
export const DEFAULT_DAY_HOURS = 8;
// MIN_DAY_HOURS and MAX_DAY_HOURS bound the day target the Worklog view takes.
export const MIN_DAY_HOURS = 0.5;
export const MAX_DAY_HOURS = 24;

// WorklogWeek is the reader's own time for the days fromDay to toDay
// ("2026-11-16"): Jira's entries and the pending ones, with the issues to
// draw them against.
export const WorklogWeek: (profileId: string, fromDay: string, toDay: string) => Promise<WorklogWeekView> =
  App.WorklogWeek as unknown as (profileId: string, fromDay: string, toDay: string) => Promise<WorklogWeekView>;
