// The Releases view's reads and writes (#605). A release is a Jira fix
// version; every write is journaled and Commit sends it.

import * as App from "../../wailsjs/go/main/App";
import { issuerepo } from "../../wailsjs/go/models";
import type { Issue } from "./issues";
import type { Version } from "./catalog";

// VersionSummary is one release with how many of its cached issues there
// are and how many of them are finished.
export interface VersionSummary extends Version {
  done: number;
  total: number;
}

// ReleaseMove is the Release dialog's choice for one unfinished issue: carry
// it over to release `to`, or descope it when `to` is 0.
export interface ReleaseMove {
  key: string;
  to: number;
}

// The journal entities of a release, mirroring issuerepo's. The key is the
// release id, a draft's negative id for a create.
export const ENTITY_VERSION_CREATE = "version_create";
export const ENTITY_VERSION_EDIT = "version_edit";
export const ENTITY_VERSION_RELEASE = "version_release";
export const ENTITY_VERSION_ARCHIVE = "version_archive";
export const ENTITY_VERSION_DELETE = "version_delete";
export const VERSION_ENTITIES: string[] = [
  ENTITY_VERSION_CREATE, ENTITY_VERSION_EDIT, ENTITY_VERSION_RELEASE, ENTITY_VERSION_ARCHIVE, ENTITY_VERSION_DELETE,
];

// VersionFields is what a create and an edit carry, and a version_edit row
// holds on both sides; a version_create row adds the project.
export interface VersionFields {
  name: string;
  description: string;
  startDate: string;
  releaseDate: string;
}

// The payloads of the other three rows.
export interface VersionRelease {
  name: string;
  releaseDate: string;
  moveTo: number;
  moveToName: string;
}

export interface VersionArchive {
  name: string;
  archived: boolean;
}

export interface VersionDelete {
  name: string;
  moveTo: number;
  moveToName: string;
}

export const ListVersionSummaries: (profileId: string) => Promise<VersionSummary[]> =
  App.ListVersionSummaries as unknown as (profileId: string) => Promise<VersionSummary[]>;

export const VersionIssues: (profileId: string, id: number) => Promise<Issue[]> =
  App.VersionIssues as unknown as (profileId: string, id: number) => Promise<Issue[]>;

export const CreateVersion: (profileId: string, name: string, description: string, start: string, release: string) => Promise<Version> =
  App.CreateVersion as unknown as (profileId: string, name: string, description: string, start: string, release: string) => Promise<Version>;

export const EditVersion: (profileId: string, id: number, name: string, description: string, start: string, release: string) => Promise<void> =
  App.EditVersion;

export function ReleaseVersion(profileId: string, id: number, date: string, moveTo: number, moves: ReleaseMove[]): Promise<void> {
  return App.ReleaseVersion(profileId, id, date, moveTo, moves.map((m) => issuerepo.ReleaseMove.createFrom(m)));
}

export const ArchiveVersion: (profileId: string, id: number, archived: boolean) => Promise<void> = App.ArchiveVersion;

export const DeleteVersion: (profileId: string, id: number, moveTo: number) => Promise<void> = App.DeleteVersion;
