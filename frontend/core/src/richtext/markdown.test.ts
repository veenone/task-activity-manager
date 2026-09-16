import { describe, it, expect } from "vitest";
import { parseMarkdown } from "./markdown";
import { parseWiki } from "./wiki";
import type { Inline } from "./ast";

function text(s: string): Inline[] {
  return [{ t: "text", text: s }];
}

describe("parseMarkdown headings", () => {
  it("parses h1 through h6", () => {
    for (let level = 1; level <= 6; level++) {
      expect(parseMarkdown(`${"#".repeat(level)} Heading ${level}`)).toEqual([
        { t: "h", level, children: text(`Heading ${level}`) },
      ]);
    }
  });
});

describe("parseMarkdown marks", () => {
  it("maps bold, italic, strike and code", () => {
    expect(parseMarkdown("**b** *i* ~~s~~ `c`")).toEqual([
      {
        t: "p",
        children: [
          { t: "mark", mark: "bold", children: text("b") },
          { t: "text", text: " " },
          { t: "mark", mark: "italic", children: text("i") },
          { t: "text", text: " " },
          { t: "mark", mark: "strike", children: text("s") },
          { t: "text", text: " " },
          { t: "code", text: "c" },
        ],
      },
    ]);
  });
});

describe("parseMarkdown lists", () => {
  it("parses a nested bullet list", () => {
    expect(parseMarkdown("- a\n  - b\n- c")).toEqual([
      {
        t: "list",
        ordered: false,
        items: [
          {
            children: [
              { t: "p", children: text("a") },
              { t: "list", ordered: false, items: [{ children: [{ t: "p", children: text("b") }] }] },
            ],
          },
          { children: [{ t: "p", children: text("c") }] },
        ],
      },
    ]);
  });

  it("parses an ordered list", () => {
    expect(parseMarkdown("1. a\n2. b")).toEqual([
      {
        t: "list",
        ordered: true,
        items: [{ children: [{ t: "p", children: text("a") }] }, { children: [{ t: "p", children: text("b") }] }],
      },
    ]);
  });

  it("gives a task item checked: true and its sibling checked: false", () => {
    expect(parseMarkdown("- [x] done\n- [ ] not done")).toEqual([
      {
        t: "list",
        ordered: false,
        items: [
          { checked: true, children: [{ t: "p", children: text("done") }] },
          { checked: false, children: [{ t: "p", children: text("not done") }] },
        ],
      },
    ]);
  });
});

describe("parseMarkdown table", () => {
  it("flags the header row", () => {
    expect(parseMarkdown("| H1 | H2 |\n| --- | --- |\n| a | b |")).toEqual([
      {
        t: "table",
        rows: [
          [
            { header: true, children: text("H1") },
            { header: true, children: text("H2") },
          ],
          [
            { header: false, children: text("a") },
            { header: false, children: text("b") },
          ],
        ],
      },
    ]);
  });
});

describe("parseMarkdown code, quote and rule", () => {
  it("parses a fenced code block with its language", () => {
    expect(parseMarkdown("```js\nvar x = 1;\n```")).toEqual([{ t: "codeblock", lang: "js", text: "var x = 1;" }]);
  });

  it("parses a blockquote", () => {
    expect(parseMarkdown("> quoted text")).toEqual([{ t: "quote", children: [{ t: "p", children: text("quoted text") }] }]);
  });

  it("parses a rule", () => {
    expect(parseMarkdown("---")).toEqual([{ t: "rule" }]);
  });
});

describe("parseMarkdown image", () => {
  it("names the image from its URL", () => {
    expect(parseMarkdown("![alt](http://x/p.png)")).toEqual([{ t: "p", children: [{ t: "image", name: "p.png" }] }]);
  });
});

describe("parseMarkdown raw HTML", () => {
  it("keeps <b>hi</b> as literal text", () => {
    expect(parseMarkdown("<b>hi</b>")).toEqual([{ t: "p", children: text("<b>hi</b>") }]);
  });
});

describe("parseMarkdown entity decoding (outside-voice fix 5)", () => {
  it("decodes marked's escaped entities exactly once in a codespan", () => {
    // marked's lexer can hand back text/codespan tokens with &amp; &lt;
    // &gt; &quot; &#39; already substituted in; markdown.ts must undo that
    // once (not zero times, and not twice) so the AST holds the actual
    // characters rather than the HTML-safe stand-ins.
    expect(parseMarkdown("`a<b &amp;`")).toEqual([{ t: "p", children: [{ t: "code", text: "a<b &" }] }]);
  });

  it("does not re-decode a literal &amp;lt; into <", () => {
    expect(parseMarkdown("`&amp;lt;`")).toEqual([{ t: "p", children: [{ t: "code", text: "&lt;" }] }]);
  });
});

describe("collisions between the two grammars", () => {
  it("*x* is bold in wiki markup and italic in Markdown", () => {
    expect(parseWiki("*x*")).toEqual([{ t: "p", children: [{ t: "mark", mark: "bold", children: text("x") }] }]);
    expect(parseMarkdown("*x*")).toEqual([{ t: "p", children: [{ t: "mark", mark: "italic", children: text("x") }] }]);
  });

  it("# one\\n# two is one ordered list in wiki markup and two headings in Markdown", () => {
    expect(parseWiki("# one\n# two")).toEqual([
      {
        t: "list",
        ordered: true,
        items: [{ children: [{ t: "p", children: text("one") }] }, { children: [{ t: "p", children: text("two") }] }],
      },
    ]);
    expect(parseMarkdown("# one\n# two")).toEqual([
      { t: "h", level: 1, children: text("one") },
      { t: "h", level: 1, children: text("two") },
    ]);
  });
});
