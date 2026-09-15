import { describe, expect, it } from "vitest";
import type { RitualDocument } from "../api";
import { conflictSentence, editorStatusLine, pendingLine, syncSummary } from "./ritualText";

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
