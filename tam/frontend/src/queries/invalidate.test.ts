import { describe, it, expect } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { invalidateProfileData } from "./invalidate";
import { peopleKeys } from "./people";

// A sync rewrites the project's issue types: #68 made it drop Xray's, and
// #65 made the New issue dialog and the type filter read the stored list.
// The query holds that list for as long as priorities, so without this the
// two surfaces keep a pre-sync list for the whole session and a user sees
// types the sync has just removed.
describe("invalidateProfileData", () => {
  it("refreshes the project's issue types, which only a sync changes", async () => {
    const qc = new QueryClient();
    const key = peopleKeys.projectTypes("p1");
    qc.setQueryData(key, [{ id: "1", name: "Test", subtask: false, logical: "" }]);
    expect(qc.getQueryState(key)?.isInvalidated).toBe(false);

    invalidateProfileData(qc, "p1");

    expect(qc.getQueryState(key)?.isInvalidated).toBe(true);
  });
});
