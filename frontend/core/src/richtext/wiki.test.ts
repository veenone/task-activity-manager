import { describe, it, expect } from "vitest";
import { parseWiki } from "./wiki";
import type { Inline } from "./ast";

// text(s) is the shape a run of plain content takes: one text node, since
// lineToInline merges adjacent literal characters (escapes, macro fallback,
// unclosed-mark repair) into a single node rather than emitting one per
// character or per branch taken.
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

  // Outside-voice fix 4 (binding): a cell's own "|" (inside a link or a
  // {code} span) must not be read as the cell delimiter.
  it("keeps a link's | from splitting the cell it lives in", () => {
    expect(parseWiki("|[Figma|https://f]|")).toEqual([
      {
        t: "table",
        rows: [
          [{ header: false, children: [{ t: "link", href: "https://f", children: text("Figma") }] }],
        ],
      },
    ]);
  });

  it("keeps a {code} span's | from splitting the cell it lives in", () => {
    expect(parseWiki("|{code}a|b{code}|")).toEqual([
      {
        t: "table",
        rows: [[{ header: false, children: [{ t: "code", text: "a|b" }] }]],
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

describe("parseWiki inline marks", () => {
  it("parses each single-character mark", () => {
    expect(parseWiki("*bold*")).toEqual([{ t: "p", children: [{ t: "mark", mark: "bold", children: text("bold") }] }]);
    expect(parseWiki("_italic_")).toEqual([
      { t: "p", children: [{ t: "mark", mark: "italic", children: text("italic") }] },
    ]);
    expect(parseWiki("+u+")).toEqual([{ t: "p", children: [{ t: "mark", mark: "underline", children: text("u") }] }]);
    expect(parseWiki("-strike-")).toEqual([
      { t: "p", children: [{ t: "mark", mark: "strike", children: text("strike") }] },
    ]);
    expect(parseWiki("^sup^")).toEqual([{ t: "p", children: [{ t: "mark", mark: "sup", children: text("sup") }] }]);
    expect(parseWiki("~sub~")).toEqual([{ t: "p", children: [{ t: "mark", mark: "sub", children: text("sub") }] }]);
  });

  it("parses {{monospace}} as a code node", () => {
    expect(parseWiki("{{mono}}")).toEqual([{ t: "p", children: [{ t: "code", text: "mono" }] }]);
  });

  it("nests italic inside bold", () => {
    expect(parseWiki("*_both_*")).toEqual([
      {
        t: "p",
        children: [
          { t: "mark", mark: "bold", children: [{ t: "mark", mark: "italic", children: text("both") }] },
        ],
      },
    ]);
  });
});

describe("parseWiki mark boundaries", () => {
  it("leaves a hyphenated word, an underscored identifier and digits around a star alone", () => {
    expect(parseWiki("mid-session re-run")).toEqual([{ t: "p", children: text("mid-session re-run") }]);
    expect(parseWiki("my_var_name")).toEqual([{ t: "p", children: text("my_var_name") }]);
    expect(parseWiki("2*3*4")).toEqual([{ t: "p", children: text("2*3*4") }]);
  });

  it("strikes a word set off by spaces", () => {
    expect(parseWiki("a -gone- b")).toEqual([
      {
        t: "p",
        children: [
          { t: "text", text: "a " },
          { t: "mark", mark: "strike", children: text("gone") },
          { t: "text", text: " b" },
        ],
      },
    ]);
  });
});

describe("parseWiki links", () => {
  it("parses a labelled link", () => {
    expect(parseWiki("[text|https://x]")).toEqual([
      { t: "p", children: [{ t: "link", href: "https://x", children: text("text") }] },
    ]);
  });

  it("parses a bracketed bare url", () => {
    expect(parseWiki("[https://x]")).toEqual([
      { t: "p", children: [{ t: "link", href: "https://x", children: text("https://x") }] },
    ]);
  });

  it("autolinks a bare url in running text", () => {
    expect(parseWiki("See https://x now")).toEqual([
      {
        t: "p",
        children: [
          { t: "text", text: "See " },
          { t: "link", href: "https://x", children: text("https://x") },
          { t: "text", text: " now" },
        ],
      },
    ]);
  });

  it("carries a user mention's and a relative link's own text as href, for the renderer to refuse", () => {
    expect(parseWiki("[~jdoe]")).toEqual([
      { t: "p", children: [{ t: "link", href: "~jdoe", children: text("~jdoe") }] },
    ]);
    expect(parseWiki("[text|/relative]")).toEqual([
      { t: "p", children: [{ t: "link", href: "/relative", children: text("text") }] },
    ]);
  });

  it("does not autolink a url stuck to the end of a word", () => {
    expect(parseWiki("seehttps://x")).toEqual([{ t: "p", children: text("seehttps://x") }]);
  });
});

// Fix round 1, Important #3: unwindFrame folds a mark that never found its
// closing delimiter back into plain text, both when a different outer mark
// closes around it and at the very end of input. It had no direct test.
describe("parseWiki unclosed marks", () => {
  it("folds an unclosed inner mark back to literal text when an outer mark closes around it", () => {
    expect(parseWiki("*a _b c*")).toEqual([
      { t: "p", children: [{ t: "mark", mark: "bold", children: text("a _b c") }] },
    ]);
  });

  it("folds an entirely unclosed mark back to literal text at the end of input", () => {
    expect(parseWiki("a *bold text")).toEqual([{ t: "p", children: text("a *bold text") }]);
  });
});

// Outside-voice fix 3 (binding): an escaped special character renders as
// itself, with no backslash and no mark firing.
describe("parseWiki escapes", () => {
  it("renders each escaped character as itself with no mark", () => {
    // Only the opening delimiters (*, |, {, [, \) are in the escape
    // grammar; a bare closing "}" or "]" was never going to start a
    // construct on its own, so it needs no escape of its own.
    expect(parseWiki("\\*not bold\\*")).toEqual([{ t: "p", children: text("*not bold*") }]);
    expect(parseWiki("a\\|b")).toEqual([{ t: "p", children: text("a|b") }]);
    expect(parseWiki("\\{not a macro}")).toEqual([{ t: "p", children: text("{not a macro}") }]);
    expect(parseWiki("\\[not a link]")).toEqual([{ t: "p", children: text("[not a link]") }]);
    expect(parseWiki("a\\\\b")).toEqual([{ t: "p", children: text("a\\b") }]);
  });

  it("reads \\\\ at the end of a line as a hard break instead of an escape", () => {
    expect(parseWiki("a\\\\")).toEqual([{ t: "p", children: [{ t: "text", text: "a" }, { t: "br" }] }]);
  });
});

describe("parseWiki colour, image and unknown macros", () => {
  it("unwraps {color}, keeping only the text inside", () => {
    expect(parseWiki("{color:red}warm{color}")).toEqual([{ t: "p", children: text("warm") }]);
  });

  it("makes an image node named after the file, attributes dropped", () => {
    expect(parseWiki("!screen.png!")).toEqual([{ t: "p", children: [{ t: "image", name: "screen.png" }] }]);
    expect(parseWiki("!screen.png|thumbnail!")).toEqual([
      { t: "p", children: [{ t: "image", name: "screen.png" }] },
    ]);
  });

  it("drops a lone unknown macro with no matching close", () => {
    expect(parseWiki("{status}")).toEqual([{ t: "p", children: [] }]);
  });

  it("unwraps an unknown macro that has a matching close, keeping its text", () => {
    expect(parseWiki("{expand}inner{expand}")).toEqual([{ t: "p", children: text("inner") }]);
  });

  it("reads a brace followed by a space as literal text, not a macro", () => {
    expect(parseWiki('{ "code": 1 }')).toEqual([{ t: "p", children: text('{ "code": 1 }') }]);
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
// D3's real promise is linear scan time, not any particular wall-clock
// number: that number is only ever as good as the machine measuring it,
// and a busy CI box measures differently from a quiet one (a review run of
// this suite alongside everything else once turned the old absolute
// "under 200ms" bound into a false failure at 344ms, while the same test
// passed standalone). So every case below times the same pathological
// shape at a base size and at 4x that size, and asserts the 4x run costs
// at most ~6x the base run (linear, with slack for jitter) rather than
// pinning either run to an absolute number: a quadratic regression fails
// that ratio on any machine, a busy machine does not. The 2s bound on each
// run is only a backstop against something hanging outright, not the real
// assertion. { retry: 2 } absorbs the rare scheduler hiccup a ratio check
// is still exposed to.
function elapsedMs(run: () => void): number {
  const start = performance.now();
  run();
  return performance.now() - start;
}

function expectLinearScaling(baseTime: number, largeTime: number): void {
  expect(baseTime).toBeLessThan(2000);
  expect(largeTime).toBeLessThan(2000);
  expect(largeTime).toBeLessThan(Math.max(baseTime * 6, 25));
}

describe("parseWiki timing", () => {
  it("parses alternating mark characters in time that scales linearly", { retry: 2 }, () => {
    const build = (reps: number) => "*_+-".repeat(reps);

    const baseTime = elapsedMs(() => parseWiki(build(12_500))); // 50,000 characters
    const largeTime = elapsedMs(() => parseWiki(build(50_000))); // 200,000 characters

    expectLinearScaling(baseTime, largeTime);
  });

  it("parses stacked list markers correctly, in time that scales linearly", { retry: 2 }, () => {
    const build = (count: number) => Array.from({ length: count }, (_, i) => `* item ${i}`).join("\n");

    const baseTime = elapsedMs(() => parseWiki(build(1_250)));
    let result: ReturnType<typeof parseWiki> = [];
    const largeTime = elapsedMs(() => {
      result = parseWiki(build(5_000));
    });

    expectLinearScaling(baseTime, largeTime);

    expect(result).toHaveLength(1);
    expect(result[0]).toMatchObject({ t: "list", ordered: false });
    expect((result[0] as { items: unknown[] }).items).toHaveLength(5_000);
  });

  // D3 extended to inline (binding): an opening {code that never closes
  // must not turn a long field into a quadratic scan.
  it("parses an unclosed {code fragment in time that scales linearly", { retry: 2 }, () => {
    const build = (count: number) => "{code" + "x".repeat(count);

    const baseTime = elapsedMs(() => parseWiki(build(50_000)));
    const largeTime = elapsedMs(() => parseWiki(build(200_000)));

    expectLinearScaling(baseTime, largeTime);
  });

  // Fix round 1, Critical: a run of unclosed "[" or "!" used to re-scan to
  // the end of the string on every one of them, the same class of
  // quadratic blowup {{ and {code} were already guarded against.
  // bracketFailFrom / imageFailFrom close that.
  it("parses unclosed [ characters in time that scales linearly", { retry: 2 }, () => {
    const build = (count: number) => "[".repeat(count);

    const baseTime = elapsedMs(() => parseWiki(build(50_000)));
    const largeTime = elapsedMs(() => parseWiki(build(200_000)));

    expectLinearScaling(baseTime, largeTime);
  });

  it("parses unclosed ! characters in time that scales linearly", { retry: 2 }, () => {
    const build = (count: number) => "!".repeat(count);

    const baseTime = elapsedMs(() => parseWiki(build(50_000)));
    const largeTime = elapsedMs(() => parseWiki(build(200_000)));

    expectLinearScaling(baseTime, largeTime);
  });

  it("parses a table row of unclosed [ characters in time that scales linearly", { retry: 2 }, () => {
    const build = (count: number) => "|" + "[".repeat(count);

    const baseTime = elapsedMs(() => parseWiki(build(1_250)));
    const largeTime = elapsedMs(() => parseWiki(build(5_000)));

    expectLinearScaling(baseTime, largeTime);
  });

  // Fix round 2, closing the gap fix round 1 only documented: scanBraceOpen's
  // ":"-attrs search used to re-scan to the end of the string on every
  // "{name:" fragment whose attributes never close, the same shape of bug
  // just fixed for "[" and "!". BraceAttrsCache closes it the same way.
  it("parses repeated {a: fragments with no closing brace in time that scales linearly", { retry: 2 }, () => {
    const build = (reps: number) => "{a:".repeat(reps);

    const baseTime = elapsedMs(() => parseWiki(build(16_666))); // ~50,000 characters
    const largeTime = elapsedMs(() => parseWiki(build(66_664))); // ~200,000 characters

    expectLinearScaling(baseTime, largeTime);
  });
});
