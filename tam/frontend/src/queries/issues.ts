import { useQuery } from "@tanstack/react-query";
import { call } from "@agile-suite/core";
import {
  GetIssueDetail,
  GetProfileSetting,
  GetSyncState,
  ListIssues,
  ListLinkedTests,
  ListSprints,
  SETTING_JIRA_DISPLAY_NAME,
  SETTING_JIRA_USERNAME,
} from "../api";
import type { IssueQuery } from "../api";
import { keys } from "./keys";

// useIssues loads one page of the Backlog. placeholderData keeps the previous
// page on screen while the next one loads, so paging does not flash. It is
// kept only when the previous page belonged to the same profile: showing one
// profile's rows under another profile's name would be a lie, so a profile
// switch falls back to the pending state.
export function useIssues(profileId: string, q: IssueQuery) {
  return useQuery({
    queryKey: keys.issues(profileId, q),
    queryFn: () => call(() => ListIssues(profileId, q)),
    enabled: !!profileId,
    placeholderData: (prev, prevQuery) =>
      prevQuery?.queryKey[0] === profileId ? prev : undefined,
  });
}

// useIssueDetail runs when the panel opens. The backend serves a fresh cache
// without a network call, so staleTime is short and the refetch cheap.
export function useIssueDetail(profileId: string, key: string) {
  return useQuery({
    queryKey: keys.issue(profileId, key),
    queryFn: () => call(() => GetIssueDetail(profileId, key)),
    enabled: !!profileId && !!key,
    retry: false,
  });
}

export function useLinkedTests(profileId: string, key: string) {
  return useQuery({
    queryKey: keys.linkedTests(profileId, key),
    queryFn: () => call(() => ListLinkedTests(profileId, key)),
    enabled: !!profileId && !!key,
    retry: false,
  });
}

export function useSprints(profileId: string) {
  return useQuery({
    queryKey: keys.sprints(profileId),
    queryFn: () => call(() => ListSprints(profileId)),
    enabled: !!profileId,
  });
}

export function useSyncState(profileId: string) {
  return useQuery({
    queryKey: keys.syncState(profileId),
    queryFn: () => call(() => GetSyncState(profileId)),
    enabled: !!profileId,
  });
}

// useJiraUsername and useJiraDisplayName are who "me" is for the Assigned to
// me tab: the two settings TestProfileConnection and the start of every sync
// write. Missing (never synced or tested) reads back "", the boards.ts
// GetProfileSetting pattern.
export function useJiraUsername(profileId: string) {
  return useQuery({
    queryKey: keys.jiraUsername(profileId),
    queryFn: () => call(() => GetProfileSetting(profileId, SETTING_JIRA_USERNAME)),
    enabled: !!profileId,
  });
}

export function useJiraDisplayName(profileId: string) {
  return useQuery({
    queryKey: keys.jiraDisplayName(profileId),
    queryFn: () => call(() => GetProfileSetting(profileId, SETTING_JIRA_DISPLAY_NAME)),
    enabled: !!profileId,
  });
}
