import { describe, expect, it } from "vitest";

import { splitPlaceholders } from "./placeholders";

describe("splitPlaceholders", () => {
  it("marks a reference that matches a parameter", () => {
    const got = splitPlaceholders("Log in as ${USERNAME}", ["USERNAME"]);
    expect(got).toEqual([
      { text: "Log in as " },
      { text: "${USERNAME}", parameter: "USERNAME", unknown: false },
    ]);
  });

  it("marks a reference that matches no parameter, which is how a typo shows", () => {
    const got = splitPlaceholders("Log in as ${USERNMAE}", ["USERNAME"]);
    expect(got).toEqual([
      { text: "Log in as " },
      { text: "${USERNMAE}", parameter: "USERNMAE", unknown: true },
    ]);
  });

  it("every reference is unknown when the test has no dataset", () => {
    const got = splitPlaceholders("${A} and ${B}", []);
    expect(got.filter((s) => s.parameter).every((s) => s.unknown)).toBe(true);
  });

  it("returns plain text as a single segment", () => {
    expect(splitPlaceholders("no placeholders here", ["A"])).toEqual([
      { text: "no placeholders here" },
    ]);
  });

  it("keeps the text between and around several references", () => {
    const got = splitPlaceholders("a ${X} b ${Y} c", ["X", "Y"]);
    expect(got.map((s) => s.text).join("")).toBe("a ${X} b ${Y} c");
    expect(got.filter((s) => s.parameter).map((s) => s.parameter)).toEqual(["X", "Y"]);
  });

  it("does not throw on an unclosed reference and leaves it as text", () => {
    const got = splitPlaceholders("broken ${UNCLOSED", ["UNCLOSED"]);
    expect(got.map((s) => s.text).join("")).toBe("broken ${UNCLOSED");
    expect(got.some((s) => s.parameter)).toBe(false);
  });

  it("matches parameter names case sensitively, as Xray does", () => {
    const got = splitPlaceholders("${username}", ["USERNAME"]);
    expect(got[0].unknown).toBe(true);
  });

  it("ignores surrounding whitespace inside the braces", () => {
    const got = splitPlaceholders("${ USERNAME }", ["USERNAME"]);
    expect(got[0]).toEqual({ text: "${ USERNAME }", parameter: "USERNAME", unknown: false });
  });

  it("handles empty text", () => {
    expect(splitPlaceholders("", ["A"])).toEqual([]);
  });

  it("treats an empty reference as unknown rather than matching a blank name", () => {
    const got = splitPlaceholders("${}", []);
    expect(got[0].unknown).toBe(true);
  });
});
