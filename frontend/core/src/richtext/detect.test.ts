import { describe, it, expect } from "vitest";
import { detectFormat, parseRich } from "./detect";

// Fifteen Jira-style samples, each written the way a real Jira Data Center
// issue reads: steps, tables, code, panels, quotes, links and monospace.
const JIRA_SAMPLES = [
  "h2. Steps to reproduce\n\n# Open the app\n# Click login\n# Observe crash",
  "* First point\n* Second point\n* Third point",
  "{code:java}\npublic void run() {}\n{code}",
  "||Name||Type||\n|Id|number|\n|Name|string|",
  "bq. This is a known limitation.",
  "{quote}\nThird-party API is flaky.\n{quote}",
  "{panel:title=Note}\nThis only affects Chrome.\n{panel}",
  "Check [Confluence page|https://example.atlassian.net/wiki/page].",
  "Run {{npm install}} before starting.",
  "----\nSee below for details.",
  "h3. Summary\n\n||Field||Value||\n|Status|Open|",
  "# Step one\n# Step two\n# Step three",
  "{noformat}\nraw stack trace here\n{noformat}",
  "h4. Error\n\n{code}\nNullPointerException\n{code}",
  "* See [docs|https://x]\n* Then restart",
];

// Fifteen GitHub-style samples, written the way a real GitHub issue reads:
// headings, task lists, fenced code, tables and links.
const GITHUB_SAMPLES = [
  "# Bug: crash on login\n\nSteps:\n1. Open app\n2. Tap login",
  "- Item one\n- Item two\n- Item three",
  "```js\nconsole.log('hi');\n```",
  "| Name | Type |\n| --- | --- |\n| id | number |",
  "> This is a known issue upstream.",
  "**Important**: restart after update.",
  "~~Deprecated~~ Use v2 instead.",
  "- [ ] Write tests\n- [x] Fix bug",
  "See [the docs](https://example.com/docs) for more.",
  "![Screenshot](https://example.com/shot.png)",
  "# Steps\n\n- Open settings\n- Toggle dark mode",
  "1. Clone repo\n2. Run npm install\n3. Start server",
  "```\nTraceback (most recent call last):\n```",
  "## Root cause\n\nThe cache **never** expires.",
  "- [x] Reproduced locally\n- [ ] Confirmed on staging",
];

describe("detectFormat on 30 labelled samples", () => {
  it.each(JIRA_SAMPLES)("Jira sample %# reads as wiki", (sample) => {
    expect(detectFormat(sample).format).toBe("wiki");
  });

  it.each(GITHUB_SAMPLES)("GitHub sample %# reads as markdown", (sample) => {
    expect(detectFormat(sample).format).toBe("markdown");
  });
});

describe("detectFormat edge cases", () => {
  it("gives wiki for empty text", () => {
    expect(detectFormat("")).toEqual({ format: "wiki", signals: [] });
  });

  it("gives wiki for a plain sentence with no signal", () => {
    expect(detectFormat("plain sentence")).toEqual({ format: "wiki", signals: [] });
  });

  it("gives wiki when one wiki and one Markdown signal each match (a tie)", () => {
    const result = detectFormat("Run {{npm test}} first.\n\n**Then deploy**.");
    expect(result.format).toBe("wiki");
  });

  it("gives wiki (a list, not a heading) for three stacked # lines", () => {
    // Each "#" line neighbours another one, so the ambiguous case in the
    // ruling resolves to a Jira ordered list; a Markdown heading never
    // gets the chance to match, so the winning signal is the wiki list's,
    // not a heading's.
    expect(detectFormat("# one\n# two\n# three")).toEqual({ format: "wiki", signals: ["#"] });
  });

  it("gives markdown for a single # heading followed by a blank line", () => {
    expect(detectFormat("# Steps\n\nClick it").format).toBe("markdown");
  });

  it("returns h3., ||table||, {code} in that order for a heading+table+code sample", () => {
    const sample = "h3. Fields\n\n||Name||Type||\n|Id|number|\n\n{code}\nconst x = 1;\n{code}";
    expect(detectFormat(sample).signals).toEqual(["h3.", "||table||", "{code}"]);
  });
});

describe("parseRich", () => {
  it("resolves auto to whatever detectFormat picks", () => {
    expect(parseRich("h1. Title", "auto").format).toBe("wiki");
    expect(parseRich("# Title\n\nBody", "auto").format).toBe("markdown");
  });

  it("honours an explicit format regardless of content", () => {
    expect(parseRich("plain text", "markdown").format).toBe("markdown");
    expect(parseRich("plain text", "wiki").format).toBe("wiki");
  });
});
