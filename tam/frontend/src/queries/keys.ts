import type { IssueQuery } from "../api";

// keys is the single source of query keys, so an invalidation can never
// drift from the read it must refresh. Every key starts with the profile id.
export const keys = {
  issues: (profileId: string, q: IssueQuery) => [profileId, "issues", q] as const,
  issue: (profileId: string, key: string) => [profileId, "issue", key] as const,
  linkedTests: (profileId: string, key: string) =>
    [profileId, "issue", key, "tests"] as const,
  sprints: (profileId: string) => [profileId, "sprints"] as const,
  syncState: (profileId: string) => [profileId, "syncState"] as const,
  pending: (profileId: string) => [profileId, "pending"] as const,
  activity: (profileId: string, key: string) => [profileId, "issue", key, "activity"] as const,
  createFields: (profileId: string, type: string) => [profileId, "createFields", type] as const,
  linkTypes: (profileId: string) => [profileId, "linkTypes"] as const,
};
