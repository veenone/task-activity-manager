import { describe, it, expect } from "vitest";
import { KINDS, offeredKinds } from "./constants";

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
