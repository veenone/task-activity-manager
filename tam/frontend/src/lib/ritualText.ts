import type { RitualDocument, RitualRoot, RitualStatus, RitualSyncResult } from "../api";

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
export const ENTRY_UNREADABLE = "Today's entry could not be read.";
export const LINK_REFUSED = "Links must start with http://, https:// or mailto:.";

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

// The missing root page. Placement is stated before anything is written, and
// the forbidden sentence is the same whether the probe said so up front or
// the create was refused.
export const ROOT_TITLE_EMPTY = "The root page needs a title.";
export const ROOT_AFTER_SENTENCE = "Sync runs again straight after. Pages this sprint does not have yet are created under the new root; a page that was under the old root and is gone shows as Gone, and Recreate on next Sync puts it under the new root.";

export function rootMissingSentence(pageId: string): string {
  return `The Confluence root page ${pageId} could not be found. It may have been deleted, moved out of reach, or mistyped.`;
}

export function rootPlacementSentence(spaceKey: string): string {
  return `The new page goes at the top of the ${spaceKey} space, and its id is saved to this profile as the rituals root.`;
}

export function rootForbiddenSentence(spaceKey: string): string {
  return `Your token cannot create pages in ${spaceKey}. Ask a space admin, or set an existing page id in Profile settings.`;
}

export function rootTakenTopSentence(title: string, spaceKey: string): string {
  return `A page titled "${title}" is already at the top of ${spaceKey}. Use that page as the rituals root? Its id is saved to this profile and Sync runs under it.`;
}

export function rootTakenNestedSentence(title: string, spaceKey: string): string {
  return `A page titled "${title}" already exists in ${spaceKey}, below another page. Choose a different title, or set that page's id in Profile settings.`;
}

export function rootDoneSentence(root: RitualRoot): string {
  return root.outcome === "adopted"
    ? `Using "${root.title}" in ${root.spaceKey} as the rituals root.`
    : `Created "${root.title}" at the top of ${root.spaceKey} and saved it as the rituals root.`;
}
