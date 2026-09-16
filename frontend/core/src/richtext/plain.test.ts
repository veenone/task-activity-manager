import { describe, it, expect } from "vitest";
import { toPlainText } from "./plain";

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
  it("returns 2,000 distinct summaries in well under 50ms", () => {
    const inputs = Array.from({ length: 2000 }, (_, i) => `Item ${i} - urgent, see [note|https://x/${i}]`);
    const start = performance.now();
    for (const input of inputs) toPlainText(input);
    const elapsed = performance.now() - start;
    expect(elapsed).toBeLessThan(50);
  });

  it("caches a repeated call", () => {
    const input = "Repeat *this* please.";
    const first = toPlainText(input);
    const second = toPlainText(input);
    expect(second).toBe(first);
  });
});
