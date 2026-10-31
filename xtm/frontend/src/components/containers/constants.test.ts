import { describe, it, expect } from "vitest";
import { KINDS, RUN_STATUSES, offeredKinds, offeredRunStatuses } from "./constants";

describe("offeredKinds", () => {
  it("offers only the kinds the backend reports", () => {
    // A Zephyr Essential profile has cycles and nothing else: no test sets and
    // no test plans. Offering them anyway sends the user to a view that can
    // only ever be empty, which reads as a sync that did not run.
    expect(offeredKinds(["testexec"]).map((k) => k.value)).toEqual(["testexec"]);
  });

  it("keeps Xray's three in the order the picker shows them", () => {
    expect(offeredKinds(["testexec", "testplan", "testset"])).toEqual(KINDS);
  });

  it("offers all of them when the backend reports none", () => {
    // Capabilities load after the first render, and the permissive default is
    // what every other consumer uses while they are in flight: nothing should
    // disappear and come back.
    expect(offeredKinds([])).toEqual(KINDS);
  });

  it("ignores a kind this build has no view for", () => {
    expect(offeredKinds(["testexec", "testcycle"]).map((k) => k.value)).toEqual(["testexec"]);
  });
});

describe("offeredRunStatuses", () => {
  it("offers the backend's own result vocabulary", () => {
    // Zephyr's statuses are configured per instance and read at connect. A
    // dropdown offering Xray's TODO or ABORTED there resolves to no id and
    // fails at commit, for a result the user was explicitly offered.
    expect(offeredRunStatuses(["UNEXECUTED", "PASS", "FAIL", "WIP"])).toEqual([
      "UNEXECUTED",
      "PASS",
      "FAIL",
      "WIP",
    ]);
  });

  it("falls back to Xray's six when the backend declares none", () => {
    // Capabilities load after the first render, and Kiwi still declares none,
    // so nothing should disappear and come back and no profile should lose
    // its dropdown.
    expect(offeredRunStatuses([])).toEqual(RUN_STATUSES);
  });
});
