import type { RitualDocument, RitualStatus, RitualSyncResult } from "../api";

// Every sentence the Rituals view and its editor print, in one place and
// testable without rendering anything, the way reportText does for Reports.

export const RITUAL_ORDER = ["_sprint", "planning", "standup", "review", "retro"] as const;

export const RITUAL_LABEL: Record<string, string> = {
  _sprint: "Overview", planning: "Planning", standup: "Standup", review: "Review", retro: "Retrospective",
};

export const STATUS_LABEL: Record<RitualStatus, string> = {
  local: "Local", synced: "Synced", unsynced: "Unsynced", conflict: "Conflict", gone: "Gone",
};

export const GONE_SENTENCE = "This page was deleted or moved in Confluence.";
export const READ_ONLY_SENTENCE = "This page has content TAM cannot edit safely. Edit it in Confluence; Sync will bring the changes back.";
export const MACRO_CAVEAT = "From TAM's cache. Done here means the status name; Confluence shows the live result.";
export const UNCONFIGURED_SENTENCE = "Add Confluence in Profile settings to sync.";
export const CLOSED_EMPTY_SENTENCE = "This sprint closed before it had ritual pages.";
export const NO_SCRUM_BOARD_SENTENCE = "No scrum board has been synced for this project, so there are no sprints to hold rituals.";
export const ENTRY_EXISTS = "Today's entry is already in the log.";
export const DAILY_LOG_MISSING = "The Daily log heading is gone, so there is nowhere to add today's entry.";

export function clock(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "" : d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

export function conflictSentence(version: number): string {
  return `Confluence has a newer version (v${version}). Your local edits are kept until you choose.`;
}

export function syncSummary(r: RitualSyncResult): string {
  const parts: string[] = [];
  if (r.created) parts.push(`${r.created} created`);
  if (r.pushed) parts.push(`${r.pushed} pushed`);
  if (r.pulled) parts.push(`${r.pulled} pulled`);
  if (r.conflicts) parts.push(`${r.conflicts} in conflict`);
  if (r.gone) parts.push(`${r.gone} gone from Confluence`);
  if (r.failed.length) parts.push(`${r.failed.length} failed`);
  return parts.length ? `Sync finished: ${parts.join(", ")}.` : "Sync finished. Everything was already in step.";
}

export function pendingLine(docs: RitualDocument[], lastSync: string): string {
  const unsynced = docs.filter((d) => d.status === "local" || d.status === "unsynced").length;
  const conflicts = docs.filter((d) => d.status === "conflict").length;
  return [
    lastSync ? `Last synced ${clock(lastSync)}` : "Not synced yet",
    unsynced ? `${unsynced} unsynced` : "",
    conflicts ? `${conflicts} ${conflicts === 1 ? "conflict" : "conflicts"}` : "",
  ].filter(Boolean).join(" · ");
}

export function editorStatusLine(input: { doc: RitualDocument; saving: boolean; saveError: string; savedAt: string }): string {
  const { doc, saving, saveError, savedAt } = input;
  if (saving) return "Saving…";
  if (saveError) return `Not saved: ${saveError}`;
  if (doc.status === "conflict") return "Conflict with Confluence";
  if (doc.status === "gone") return "Gone from Confluence";
  if (doc.status === "synced") return `Synced ${clock(doc.syncedAt)}`;
  return `Saved locally ${clock(savedAt || doc.updatedAt)} · not synced`;
}
