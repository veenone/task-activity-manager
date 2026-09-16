import type { Block, RichFormat } from "./ast";
import { parseWiki } from "./wiki";
import { parseMarkdown } from "./markdown";

// detectFormat scores each format by the count of DISTINCT signal kinds it
// matches, not occurrences (a ruling: thirty "- " bullets must not outvote
// one "{code}"), so each kind is recorded at most once, keyed by the label
// of the first line that satisfied it. Markdown wins only on a strict
// majority; a tie (including 0-0, empty text or a plain sentence) reads as
// Jira markup, which is also the safer default when a field turns out to
// hold neither.
export function detectFormat(text: string): { format: RichFormat; signals: string[] } {
  const lines = text.split("\n");
  const wiki = scanWiki(lines);
  const markdown = scanMarkdown(lines);
  if (markdown.size > wiki.size) return { format: "markdown", signals: [...markdown.values()] };
  return { format: "wiki", signals: [...wiki.values()] };
}

export function parseRich(text: string, format: RichFormat | "auto"): { format: RichFormat; blocks: Block[] } {
  const resolved = format === "auto" ? detectFormat(text).format : format;
  return { format: resolved, blocks: resolved === "markdown" ? parseMarkdown(text) : parseWiki(text) };
}

// A line beginning with "#" is ambiguous on its own: Jira's own ordered
// list marker and a Markdown heading use the identical character. The
// ruling: two or more STACKED "#" lines read as a Jira list, an isolated
// one (blank or absent neighbour on both sides) reads as a heading
// candidate instead. Both scanners below call this so a single "#" line
// can never register as both a wiki list item and a Markdown heading.
function hashIsStacked(lines: string[], index: number): boolean {
  const prev = index > 0 ? lines[index - 1] : "";
  const next = index < lines.length - 1 ? lines[index + 1] : "";
  return prev.startsWith("#") || next.startsWith("#");
}

function scanWiki(lines: string[]): Map<string, string> {
  const found = new Map<string, string>();
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    if (!found.has("heading")) {
      const h = matchWikiHeading(line);
      if (h) found.set("heading", h);
    }
    if (!found.has("table") && line.startsWith("||")) found.set("table", "||table||");
    if (!found.has("code") && (line === "{code}" || line.startsWith("{code:") || line.startsWith("{code|"))) {
      found.set("code", "{code}");
    }
    if (!found.has("noformat") && line === "{noformat}") found.set("noformat", "{noformat}");
    if (!found.has("panel") && line.startsWith("{panel")) found.set("panel", "{panel}");
    if (!found.has("quote") && (line === "{quote}" || line.startsWith("bq. "))) {
      found.set("quote", line.startsWith("bq.") ? "bq." : "{quote}");
    }
    if (!found.has("rule") && line === "----") found.set("rule", "----");
    if (!found.has("list")) {
      const marker = matchWikiList(lines, i);
      if (marker) found.set("list", marker);
    }
    if (!found.has("link") && matchWikiLink(line)) found.set("link", "[x|y]");
    if (!found.has("mono") && line.includes("{{") && line.includes("}}")) found.set("mono", "{{mono}}");
  }
  return found;
}

function matchWikiHeading(line: string): string | null {
  if (line.length < 3 || line[0] !== "h") return null;
  const level = line.charCodeAt(1) - 48;
  if (level < 1 || level > 6) return null;
  if (line[2] !== ".") return null;
  if (line.length > 3 && line[3] !== " ") return null;
  return `h${level}.`;
}

function matchWikiList(lines: string[], index: number): string | null {
  const line = lines[index];
  let i = 0;
  while (i < line.length && (line[i] === "*" || line[i] === "#")) i++;
  if (i === 0 || line[i] !== " ") return null;
  if (line[0] === "#" && !hashIsStacked(lines, index)) return null; // read as a heading candidate instead
  return line[0];
}

function matchWikiLink(line: string): boolean {
  const open = line.indexOf("[");
  if (open === -1) return false;
  const close = line.indexOf("]", open + 1);
  return close !== -1 && line.slice(open + 1, close).includes("|");
}

const MD_HEADING = /^#{1,6}(?: |$)/;
const MD_TASK = /^[-*+] \[[ xX]\] /;
const MD_BULLET = /^[-+] /;
const MD_ORDERED = /^\d+\. /;
const MD_TABLE_SEP = /^[\s|:-]+$/;
const MD_LINK = /\[[^\]]+\]\([^)]+\)/;
const MD_IMAGE = /!\[[^\]]*\]\([^)]+\)/;

function scanMarkdown(lines: string[]): Map<string, string> {
  const found = new Map<string, string>();
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    if (!found.has("heading") && MD_HEADING.test(line) && !hashIsStacked(lines, i)) {
      found.set("heading", "#");
    }
    if (!found.has("bold") && line.includes("**")) found.set("bold", "**bold**");
    if (!found.has("strike") && line.includes("~~")) found.set("strike", "~~strike~~");
    if (!found.has("fence") && line.startsWith("```")) found.set("fence", "```");
    if (!found.has("table") && line.includes("|") && MD_TABLE_SEP.test(line)) found.set("table", "|---|");
    if (!found.has("task") && MD_TASK.test(line)) found.set("task", "- [ ]");
    if (!found.has("bullet") && MD_BULLET.test(line)) found.set("bullet", "- ");
    if (!found.has("ordered") && MD_ORDERED.test(line)) found.set("ordered", "1.");
    if (!found.has("quote") && line.startsWith("> ")) found.set("quote", "> ");
    if (!found.has("image") && MD_IMAGE.test(line)) found.set("image", "![]()");
    if (!found.has("link") && MD_LINK.test(line)) found.set("link", "[]()");
  }
  return found;
}
