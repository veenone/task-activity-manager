// Ritual pages, their roots, and the done agreement's ticks.

import * as App from "../../wailsjs/go/main/App";
import { profile } from "../../wailsjs/go/models";
import type { ConfluenceConfig } from "./core";
import type { Issue } from "./issues";

// The done agreement's per-issue ticks. These are the one pair of writes in
// TAM that never reaches Jira: a tick is not journalled and is not pushed on
// Commit, because Jira has nowhere to put it and it is TAM's own record of
// what a team checked rather than a change to a Jira issue.
//
// DoneAgreementTicks answers the wording of every item ticked against each
// issue named, whether or not the agreement still states it, and takes a list
// of issues so the report's section can ask about a whole sprint in the call
// the panel makes for one. Deciding which of those wordings the agreement
// still states is the caller's, through lib/doneAgreement.
export const DoneAgreementTicks: (
  profileId: string,
  boardId: number,
  issueKeys: string[],
) => Promise<Record<string, string[]>> = App.DoneAgreementTicks;
export const SetDoneAgreementTick: (
  profileId: string,
  boardId: number,
  issueKey: string,
  item: string,
  ticked: boolean,
) => Promise<void> = App.SetDoneAgreementTick;

export const GetConfluenceConfig: (profileId: string) => Promise<ConfluenceConfig> = App.GetConfluenceConfig as any;
export const SetConfluenceConfig = (profileId: string, config: ConfluenceConfig, token: string): Promise<void> =>
  App.SetConfluenceConfig(profileId, profile.ConfluenceConfig.createFrom(config), token);
// A ritual page as tam.db keeps it. body is the local page in Confluence
// storage format; baseBody the page as of version, the last one synced;
// conflictBody a newer remote a Sync found while local edits were pending.
export type RitualStatus = "local" | "synced" | "unsynced" | "conflict" | "gone";
export interface RitualDocument {
  profileId: string;
  boardId: number;
  sprintId: number;
  ritualType: string;
  title: string;
  body: string;
  baseBody: string;
  pageId: string;
  version: number;
  conflictBody: string;
  conflictVersion: number;
  status: RitualStatus;
  updatedAt: string;
  syncedAt: string;
}
export interface RitualPageFailure { sprintName: string; title: string; reason: string }
// RitualRootMissing is a Sync that stopped because the configured root page
// answered 404. canCreate is the permission probe's answer, unknown read as yes.
export interface RitualRootMissing { pageId: string; spaceKey: string; canCreate: boolean; suggestedTitle: string }
export interface RitualSyncResult {
  // Set, and nothing else is, when the root page is gone. Optional only so
  // fixtures written before it stay valid; Go always sends it, null when the
  // root was read.
  rootMissing?: RitualRootMissing | null;
  created: number;
  pulled: number;
  pushed: number;
  conflicts: number;
  gone: number;
  failed: RitualPageFailure[];
  syncedAt: string;
}
export interface RitualMacroPreview { supported: boolean; jql: string; issues: Issue[] }

export const EnsureSprintRituals: (profileId: string, boardId: number, sprintId: number) => Promise<RitualDocument[]> = App.EnsureSprintRituals as any;
export const ListRitualDocuments: (profileId: string, boardId: number, sprintId: number) => Promise<RitualDocument[]> = App.ListRitualDocuments as any;
// The done agreement is asked for, never written for every sprint: sprintId 0
// is the board's standing one and a sprint's own id is its additions. Both
// arrive in the sprint's list afterwards.
// The cast goes through unknown rather than any: the generated class types
// status as a plain string where RitualDocument names the five it can hold.
export const CreateDoneAgreement = App.CreateDoneAgreement as unknown as
  (profileId: string, boardId: number, sprintId: number) => Promise<RitualDocument>;
// version and pageId are what the editor was opened on; the save is refused
// once a Sync has moved the row past either.
export const SaveRitualBody: (profileId: string, boardId: number, sprintId: number, ritualType: string, body: string, version: number, pageId: string) => Promise<RitualDocument> = App.SaveRitualBody as any;
export const ResolveRitualConflict: (profileId: string, boardId: number, sprintId: number, ritualType: string, choice: "mine" | "theirs") => Promise<void> = App.ResolveRitualConflict;
export const ForgetRitualPage: (profileId: string, boardId: number, sprintId: number, ritualType: string) => Promise<void> = App.ForgetRitualPage;
export const DeleteRitualDocument: (profileId: string, boardId: number, sprintId: number, ritualType: string) => Promise<void> = App.DeleteRitualDocument;
export const RitualMacroIssues: (profileId: string, jql: string) => Promise<RitualMacroPreview> = App.RitualMacroIssues as any;
export const StandupEntry: (day: string) => Promise<string> = App.StandupEntry;
export const LastRitualSync: (profileId: string, boardId: number) => Promise<string> = App.LastRitualSync;
export const SyncRituals: (profileId: string, boardId: number) => Promise<RitualSyncResult> = App.SyncRituals as any;

export type RitualRootOutcome = "created" | "adopted" | "forbidden" | "titleTaken";
export interface RitualRoot { outcome: RitualRootOutcome; pageId: string; title: string; spaceKey: string; topLevel: boolean }
// sync is the pass Go ran on the new root, null when no root was set; syncError
// is that pass's refusal, with the new root saved either way.
export interface RitualRootResult { root: RitualRoot; sync: RitualSyncResult | null; syncError: string }
// CreateRitualRoot takes Go's "rituals" lock. Call it only through
// SyncContext.runRitualRoot, never directly.
export const CreateRitualRoot: (profileId: string, boardId: number, title: string, adopt: boolean) => Promise<RitualRootResult> = App.CreateRitualRoot as any;
// CreateReportRoot makes, or adopts, the top level page a profile's sprint
// reports hang under, with the same outcomes the rituals root has. An empty
// space key means the rituals space. It saves nothing: the profile form saves
// the page id it answers with.
export const CreateReportRoot = (profileId: string, spaceKey: string, title: string, adopt: boolean): Promise<RitualRoot> =>
  App.CreateReportRoot(profileId, spaceKey, title, adopt) as Promise<RitualRoot>;
