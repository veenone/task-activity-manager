import { describe, expect, it } from "vitest";
import type { RitualDocument } from "../api";
import {
  ROOT_AFTER_SENTENCE, conflictSentence, editorStatusLine, pendingLine, rootDoneSentence, rootForbiddenSentence,
  rootMissingSentence, rootPlacementSentence, rootTakenNestedSentence, rootTakenTopSentence, syncSummary,
} from "./ritualText";

const doc = (status: RitualDocument["status"]): RitualDocument => ({
  profileId: "p1", boardId: 1, sprintId: 14, ritualType: "planning", title: "T", body: "", baseBody: "",
  pageId: "", version: 0, conflictBody: "", conflictVersion: 0, status, updatedAt: "2026-09-14T09:05:00Z", syncedAt: "2026-09-14T09:00:00Z",
});

describe("ritual text", () => {
  it("says what a Sync did, in the order it matters", () => {
    expect(syncSummary({ created: 5, pulled: 1, pushed: 2, conflicts: 1, gone: 0, failed: [{ sprintName: "S", title: "T", reason: "r" }], syncedAt: "" }))
      .toBe("Sync finished: 5 created, 2 pushed, 1 pulled, 1 in conflict, 1 failed.");
    expect(syncSummary({ created: 0, pulled: 0, pushed: 0, conflicts: 0, gone: 0, failed: [], syncedAt: "" }))
      .toBe("Sync finished. Everything was already in step.");
  });

  it("counts local and unsynced pages as unsynced, and conflicts apart", () => {
    const line = pendingLine([doc("local"), doc("unsynced"), doc("synced"), doc("conflict")], "");
    expect(line).toBe("Not synced yet · 2 unsynced · 1 conflict");
    expect(pendingLine([doc("synced")], "2026-09-14T09:00:00Z")).toMatch(/^Last synced .+/);
  });

  it("names the newer version in the conflict sentence", () => {
    expect(conflictSentence(8)).toBe("Confluence has a newer version (v8). Your local edits are kept until you choose.");
  });

  it("picks the editor status line from what is happening now", () => {
    expect(editorStatusLine({ doc: doc("unsynced"), saving: true, saveError: "", savedAt: "" })).toBe("Saving…");
    expect(editorStatusLine({ doc: doc("unsynced"), saving: false, saveError: "disk full", savedAt: "" })).toBe("Not saved: disk full");
    expect(editorStatusLine({ doc: doc("conflict"), saving: false, saveError: "", savedAt: "" })).toBe("Conflict with Confluence");
    expect(editorStatusLine({ doc: doc("synced"), saving: false, saveError: "", savedAt: "" })).toMatch(/^Synced .+/);
    expect(editorStatusLine({ doc: doc("local"), saving: false, saveError: "", savedAt: "" })).toMatch(/^Saved locally .+ · not synced$/);
  });

  it("uses no em dash anywhere", async () => {
    const text = await import("./ritualText");
    for (const value of Object.values(text)) if (typeof value === "string") expect(value).not.toContain(String.fromCharCode(0x2014));
  });
});

describe("root page text", () => {
  it("words the missing root, where the new one goes, and what Sync does next", () => {
    expect(rootMissingSentence("653264152")).toBe("The Confluence root page 653264152 could not be found. It may have been deleted, moved out of reach, or mistyped.");
    expect(rootPlacementSentence("TEAM")).toBe("The new page goes at the top of the TEAM space, and its id is saved to this profile as the rituals root.");
    expect(ROOT_AFTER_SENTENCE).toBe("Sync runs again straight after. This sprint's ritual pages are created under the new root; a page that was under the old root and is gone shows as Gone, and Recreate on next Sync puts it under the new root.");
  });

  it("words the refusals", () => {
    expect(rootForbiddenSentence("TEAM")).toBe("Your token cannot create pages in TEAM. Ask a space admin, or set an existing page id in Profile settings.");
    expect(rootTakenTopSentence("PLAT Rituals", "TEAM")).toBe('A page titled "PLAT Rituals" is already at the top of TEAM. Use that page as the rituals root? Its id is saved to this profile and Sync runs under it.');
    expect(rootTakenNestedSentence("PLAT Rituals", "TEAM")).toBe(`A page titled "PLAT Rituals" already exists in TEAM, below another page. Choose a different title, or set that page's id in Profile settings.`);
  });

  it("says which root was set", () => {
    const root = { outcome: "created" as const, pageId: "9001", title: "PLAT Rituals", spaceKey: "TEAM", topLevel: true };
    expect(rootDoneSentence(root)).toBe('Created "PLAT Rituals" at the top of TEAM and saved it as the rituals root.');
    expect(rootDoneSentence({ ...root, outcome: "adopted" })).toBe('Using "PLAT Rituals" in TEAM as the rituals root.');
  });

  it("prints no em dash anywhere", () => {
    const all = [rootMissingSentence("1"), rootPlacementSentence("T"), ROOT_AFTER_SENTENCE, rootForbiddenSentence("T"), rootTakenTopSentence("a", "T"), rootTakenNestedSentence("a", "T")];
    for (const s of all) expect(s).not.toContain(String.fromCharCode(0x2014));
  });
});
