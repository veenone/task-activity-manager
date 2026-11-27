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

// The Dependencies view reads links a sync stores and a local link edit
// journals, so both have to refresh it (#612).
describe("the dependency graph", () => {
  it("is refreshed by a sync and by a local write", async () => {
    const { invalidateWrites } = await import("./invalidate");
    const { keys } = await import("./keys");
    for (const run of [(qc: QueryClient) => invalidateProfileData(qc, "p1"), (qc: QueryClient) => invalidateWrites(qc, "p1", "PLAT-1")]) {
      const qc = new QueryClient();
      qc.setQueryData(keys.dependencies("p1"), { issues: [], external: [], links: [] });
      run(qc);
      expect(qc.getQueryState(keys.dependencies("p1"))?.isInvalidated).toBe(true);
    }
  });
});

// A fix version edit moves an issue between releases, so the Releases
// view's counts and issue lists have to refresh with it, as they do on a
// sync (#605).
describe("the releases", () => {
  it("are refreshed by a sync and by a local write", async () => {
    const { invalidateWrites } = await import("./invalidate");
    const { keys } = await import("./keys");
    for (const run of [(qc: QueryClient) => invalidateProfileData(qc, "p1"), (qc: QueryClient) => invalidateWrites(qc, "p1", "PLAT-1")]) {
      const qc = new QueryClient();
      qc.setQueryData(keys.versionSummaries("p1"), []);
      run(qc);
      expect(qc.getQueryState(keys.versionSummaries("p1"))?.isInvalidated).toBe(true);
    }
  });
});
