import { describe, it, expect, vi, beforeEach } from "vitest";
import { toPlainText } from "./plain";
import { parseRich } from "./detect";

// D4's own promise is never "fast" in wall-clock terms (that bound is only
// ever as good as the machine running it, and a full-suite run is not a
// quiet machine); it is "the fast path skips the parser, and the cache
// skips it too, within a bound". vi.mock({ spy: true }) keeps detect.ts's
// real implementation (parseRich still parses for real) while recording
// every call, so the tests below assert exactly that promise rather than a
// timing.
vi.mock("./detect", { spy: true });

describe("toPlainText", () => {
  it("unwraps a wiki mark and a monospace macro", () => {
    expect(toPlainText("Fix *login* at {{/auth}}")).toBe("Fix login at /auth");
  });

  it("unwraps a wiki link to its label", () => {
    expect(toPlainText("[Figma|https://f]")).toBe("Figma");
  });

  it("unwraps Markdown bold", () => {
    expect(toPlainText("**Bold** move")).toBe("Bold move");
  });

  it("leaves a hyphenated word alone", () => {
    expect(toPlainText("mid-session")).toBe("mid-session");
  });

  it("collapses multi-line text, blank line and all, to one line", () => {
    expect(toPlainText("Line one\n\nLine two with *bold*.")).toBe("Line one Line two with bold.");
  });

  it("returns text with no markup characters unchanged, uncached path included", () => {
    expect(toPlainText("just a plain sentence")).toBe("just a plain sentence");
  });
});

describe("toPlainText summary scope (D5)", () => {
  it("unwraps inline code and a link, never a mark", () => {
    expect(toPlainText("Fix {{login}} at [Figma|https://f]", "summary")).toBe("Fix login at Figma");
  });

  it("leaves numbers-with-asterisks exactly as typed", () => {
    expect(toPlainText("2*3*4 items", "summary")).toBe("2*3*4 items");
  });

  it("leaves hyphen-separated words exactly as typed", () => {
    expect(toPlainText("cost - benefit - tradeoff", "summary")).toBe("cost - benefit - tradeoff");
  });

  it("leaves a Markdown-shaped bold marker exactly as typed", () => {
    expect(toPlainText("Ship **now**", "summary")).toBe("Ship **now**");
  });

  it("unwraps a Markdown link and a codespan the same way", () => {
    expect(toPlainText("See `/payments` and [docs](https://x)", "summary")).toBe("See /payments and docs");
  });
});

describe("toPlainText performance and caching (D4)", () => {
  beforeEach(() => {
    vi.mocked(parseRich).mockClear();
  });

  it("never parses 2,000 markup-free strings (the HAS_MARKUP fast path)", () => {
    const inputs = Array.from(
      { length: 2000 },
      (_, i) => `d4 fast path plain sentence number ${i} with no markup at all`
    );
    for (const input of inputs) toPlainText(input);
    expect(parseRich).not.toHaveBeenCalled();
  });

  it("parses a markup-bearing input once, and serves a repeat from the cache", () => {
    const input = "d4 cache probe *bold* text, unique to this test";
    toPlainText(input);
    toPlainText(input);
    expect(parseRich).toHaveBeenCalledTimes(1);
  });

  it("evicts the oldest entry once the bounded cache fills, so it is re-parsed", () => {
    const CACHE_CAP = 2000;
    const first = "d4 eviction probe 0 *mark*";

    toPlainText(first);
    expect(parseRich).toHaveBeenCalledTimes(1);

    // CACHE_CAP more distinct entries: by the last of these, every entry
    // that existed in the cache before "first" was inserted has aged out,
    // and this loop's own final insertion is what pushes "first" itself
    // out (see plain.ts's remember: a bounded Map evicts its single oldest
    // entry once size exceeds the cap, so a run of N new keys following an
    // (N-1)-old entry evicts exactly that entry on the Nth new insertion).
    for (let i = 1; i <= CACHE_CAP; i++) toPlainText(`d4 eviction probe ${i} *mark*`);

    toPlainText(first);
    expect(parseRich).toHaveBeenCalledTimes(CACHE_CAP + 2);
  });
});
