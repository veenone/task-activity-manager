import type { IssueQuery, TreeQuery } from "../api";

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
  tree: (profileId: string, q: TreeQuery) => [profileId, "tree", q] as const,
  epics: (profileId: string) => [profileId, "epics"] as const,
  boards: (profileId: string) => [profileId, "boards"] as const,
  boardSprints: (profileId: string, boardId: number) =>
    [profileId, "boardSprints", boardId] as const,
  // Every open sprint across every board the profile has synced, the list a
  // caller with no board of its own offers the Sprint field.
  openSprints: (profileId: string) => [profileId, "openSprints"] as const,
  board: (profileId: string, boardId: number, sprintId: string, swimlane: string) =>
    [profileId, "board", boardId, sprintId, swimlane] as const,
  // What the start dialog opens with: the next sprint's name and the dates
  // the board's own history suggests.
  sprintSuggestion: (profileId: string, boardId: number) =>
    [profileId, "sprintSuggestion", boardId] as const,
  // The one profile setting the Boards view reads: whether this Jira
  // answered the boards call with no Agile API at all.
  boardsUnavailable: (profileId: string) => [profileId, "boardsUnavailable"] as const,
};
