// api.ts — typed access to the Wails Go backend.
//
// The data interfaces below are plain shapes that match the JSON the backend
// returns. They are intentionally separate from the generated wailsjs model
// classes so plain object literals (initial state, query objects) type-check.

export {
  ListProfiles,
  CreateProfile,
  DeleteProfile,
  TestConnection,
  SyncProfile,
  GetSyncState,
  ListFolders,
  ListTests,
  GetTest,
} from "../wailsjs/go/main/App";
export { EventsOn } from "../wailsjs/runtime/runtime";

export interface Profile {
  id: string;
  name: string;
  jiraUrl: string;
  projectKey: string;
  createdAt: string;
}

export interface TestCase {
  key: string;
  id: string;
  summary: string;
  description: string;
  status: string;
  priority: string;
  labels: string[];
  updated: string;
  folderId: string;
}

export interface TestPage {
  tests: TestCase[];
  total: number;
}

export interface TestQuery {
  search: string;
  status: string;
  folderId: string;
  sortBy: string;
  desc: boolean;
  limit: number;
  offset: number;
}

export interface SyncState {
  profileId: string;
  lastSyncedAt: string;
  testCount: number;
}

export interface Folder {
  id: string;
  parentId: string;
  name: string;
}

// SyncProgress mirrors the Go syncer.Progress payload emitted on "sync:progress".
export interface SyncProgress {
  fetched: number;
  total: number;
  done: boolean;
}

// errMsg renders any thrown value (unknown in strict mode) as a string.
export function errMsg(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === "string" ? e : String(e);
}
