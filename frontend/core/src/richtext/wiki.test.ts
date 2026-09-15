import { describe, it, expect } from "vitest";
import { parseWiki } from "./wiki";
import type { Inline } from "./ast";

// text(s) is the shape every block's content takes in Task 1a: Task 1b owns
// marks, links, escapes and macros, so until then a block or cell's only
// child is the line's raw text.
function text(s: string): Inline[] {
  return [{ t: "text", text: s }];
}

describe("parseWiki headings", () => {
  it("parses h1 through h6", () => {
    for (let level = 1; level <= 6; level++) {
      expect(parseWiki(`h${level}. Heading ${level}`)).toEqual([
        { t: "h", level, children: text(`Heading ${level}`) },
      ]);
    }
  });
});

describe("parseWiki paragraphs", () => {
  it("splits paragraphs on a blank line", () => {
    expect(parseWiki("First paragraph.\n\nSecond paragraph.")).toEqual([
      { t: "p", children: text("First paragraph.") },
      { t: "p", children: text("Second paragraph.") },
    ]);
  });

  it("keeps consecutive non-blank lines in one paragraph", () => {
    expect(parseWiki("Line one\nLine two")).toEqual([{ t: "p", children: text("Line one\nLine two") }]);
  });
});

describe("parseWiki lists", () => {
  it("parses a flat bullet list", () => {
    expect(parseWiki("* a\n* b")).toEqual([
      {
        t: "list",
        ordered: false,
        items: [
          { children: [{ t: "p", children: text("a") }] },
          { children: [{ t: "p", children: text("b") }] },
        ],
      },
    ]);
  });

  it("parses a flat ordered list", () => {
    expect(parseWiki("# a\n# b")).toEqual([
      {
        t: "list",
        ordered: true,
        items: [
          { children: [{ t: "p", children: text("a") }] },
          { children: [{ t: "p", children: text("b") }] },
        ],
      },
    ]);
  });

  it("nests a bullet list under a bullet item with **", () => {
    expect(parseWiki("* a\n** b\n* c")).toEqual([
      {
        t: "list",
        ordered: false,
        items: [
          {
            children: [
              { t: "p", children: text("a") },
              {
                t: "list",
                ordered: false,
                items: [{ children: [{ t: "p", children: text("b") }] }],
              },
            ],
          },
          { children: [{ t: "p", children: text("c") }] },
        ],
      },
    ]);
  });

  it("nests an ordered list under an ordered item with ##", () => {
    expect(parseWiki("# a\n## b\n# c")).toEqual([
      {
        t: "list",
        ordered: true,
        items: [
          {
            children: [
              { t: "p", children: text("a") },
              {
                t: "list",
                ordered: true,
                items: [{ children: [{ t: "p", children: text("b") }] }],
              },
            ],
          },
          { children: [{ t: "p", children: text("c") }] },
        ],
      },
    ]);
  });

  it("nests a bullet list under an ordered item with #*", () => {
    expect(parseWiki("# a\n#* b")).toEqual([
      {
        t: "list",
        ordered: true,
        items: [
          {
            children: [
              { t: "p", children: text("a") },
              {
                t: "list",
                ordered: false,
                items: [{ children: [{ t: "p", children: text("b") }] }],
              },
            ],
          },
        ],
      },
    ]);
  });

  it("starts a new sibling list when the top-level marker type switches", () => {
    expect(parseWiki("* a\n# b")).toEqual([
      { t: "list", ordered: false, items: [{ children: [{ t: "p", children: text("a") }] }] },
      { t: "list", ordered: true, items: [{ children: [{ t: "p", children: text("b") }] }] },
    ]);
  });

  it("keeps every item across two type switches, dropping nothing", () => {
    expect(parseWiki("* a\n** b\n* c\n# d")).toEqual([
      {
        t: "list",
        ordered: false,
        items: [
          {
            children: [
              { t: "p", children: text("a") },
              {
                t: "list",
                ordered: false,
                items: [{ children: [{ t: "p", children: text("b") }] }],
              },
            ],
          },
          { children: [{ t: "p", children: text("c") }] },
        ],
      },
      { t: "list", ordered: true, items: [{ children: [{ t: "p", children: text("d") }] }] },
    ]);
  });

  it("closes a nested list and starts a new sibling on a top-level type switch", () => {
    expect(parseWiki("* a\n** b\n# c")).toEqual([
      {
        t: "list",
        ordered: false,
        items: [
          {
            children: [
              { t: "p", children: text("a") },
              {
                t: "list",
                ordered: false,
                items: [{ children: [{ t: "p", children: text("b") }] }],
              },
            ],
          },
        ],
      },
      { t: "list", ordered: true, items: [{ children: [{ t: "p", children: text("c") }] }] },
    ]);
  });

  it("ends a list at a blank line", () => {
    expect(parseWiki("* a\n* b\n\nNext paragraph")).toEqual([
      {
        t: "list",
        ordered: false,
        items: [
          { children: [{ t: "p", children: text("a") }] },
          { children: [{ t: "p", children: text("b") }] },
        ],
      },
      { t: "p", children: text("Next paragraph") },
    ]);
  });
});

describe("parseWiki tables", () => {
  it("parses a header row then a normal row", () => {
    expect(parseWiki("||h1||h2||\n|a|b|")).toEqual([
      {
        t: "table",
        rows: [
          [
            { header: true, children: text("h1") },
            { header: true, children: text("h2") },
          ],
          [
            { header: false, children: text("a") },
            { header: false, children: text("b") },
          ],
        ],
      },
    ]);
  });

  it("parses a row mixing header and normal cells", () => {
    expect(parseWiki("||h1||h2|c1|")).toEqual([
      {
        t: "table",
        rows: [
          [
            { header: true, children: text("h1") },
            { header: true, children: text("h2") },
            { header: false, children: text("c1") },
          ],
        ],
      },
    ]);
  });
});

describe("parseWiki code blocks", () => {
  it("keeps {code:lang} content verbatim, marks untouched", () => {
    expect(parseWiki("{code:java}\nint x = 1;\n*bold* stays literal\n{code}")).toEqual([
      { t: "codeblock", lang: "java", text: "int x = 1;\n*bold* stays literal" },
    ]);
  });

  it("parses {code} with no language", () => {
    expect(parseWiki("{code}\nplain text\n{code}")).toEqual([{ t: "codeblock", lang: "", text: "plain text" }]);
  });

  it("parses {noformat}", () => {
    expect(parseWiki("{noformat}\nverbatim *text*\n{noformat}")).toEqual([
      { t: "codeblock", lang: "", text: "verbatim *text*" },
    ]);
  });

  it("runs an unclosed {code} to the end", () => {
    expect(parseWiki("{code:java}\nint x = 1;\nint y = 2;")).toEqual([
      { t: "codeblock", lang: "java", text: "int x = 1;\nint y = 2;" },
    ]);
  });
});

describe("parseWiki quote, panel and rule", () => {
  it("parses a {quote} block", () => {
    expect(parseWiki("{quote}\nQuoted text\n{quote}")).toEqual([
      { t: "quote", children: [{ t: "p", children: text("Quoted text") }] },
    ]);
  });

  it("parses a bq. line", () => {
    expect(parseWiki("bq. Quoted line")).toEqual([
      { t: "quote", children: [{ t: "p", children: text("Quoted line") }] },
    ]);
  });

  it("parses {panel:title=...}", () => {
    expect(parseWiki("{panel:title=Notes}\nPanel body\n{panel}")).toEqual([
      { t: "panel", title: "Notes", children: [{ t: "p", children: text("Panel body") }] },
    ]);
  });

  it("parses {panel} with no title", () => {
    expect(parseWiki("{panel}\nNo title\n{panel}")).toEqual([
      { t: "panel", title: "", children: [{ t: "p", children: text("No title") }] },
    ]);
  });

  it("parses ---- as a rule", () => {
    expect(parseWiki("Before\n----\nAfter")).toEqual([
      { t: "p", children: text("Before") },
      { t: "rule" },
      { t: "p", children: text("After") },
    ]);
  });
});

// D3 (binding): the parser scans character by character, never running a
// regex with nested quantifiers over user text, so both a very long line
// and a very long run of list items stay linear. ~200ms is generous so CI
// stays stable; a quadratic parser would blow well past it on either input.
describe("parseWiki timing", () => {
  it("parses 200,000 alternating mark characters in well under 200ms", () => {
    const text = "*_+-".repeat(50_000);
    expect(text.length).toBe(200_000);

    const start = performance.now();
    parseWiki(text);
    expect(performance.now() - start).toBeLessThan(200);
  });

  it("parses 5,000 stacked list markers in well under 200ms", () => {
    const lines = Array.from({ length: 5_000 }, (_, i) => `* item ${i}`);

    const start = performance.now();
    const result = parseWiki(lines.join("\n"));
    expect(performance.now() - start).toBeLessThan(200);

    expect(result).toHaveLength(1);
    expect(result[0]).toMatchObject({ t: "list", ordered: false });
    expect((result[0] as { items: unknown[] }).items).toHaveLength(5_000);
  });
});
