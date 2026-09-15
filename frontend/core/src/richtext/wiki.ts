import type { Block, Inline } from "./ast";

// parseWiki turns Jira wiki markup into the shared AST. This file (Task 1a)
// only builds block structure: headings, paragraphs, lists, tables, code
// blocks, quotes, panels and rules. Every block's inline content is one
// placeholder text node for now; Task 1b replaces lineToInline with real
// mark, link, escape and macro parsing without touching the block walker
// above it.
//
// D3 (binding): scan character by character, no regex with nested
// quantifiers over user text. Every match* helper below is a handful of
// index/charAt comparisons with an early bail on the first mismatching
// character, so a run of non-matching lines costs O(1) each rather than
// O(line length), and the whole parse is linear in the input size.
export function parseWiki(text: string): Block[] {
  const lines = splitLines(text);
  return parseBlockLines(lines, 0, lines.length);
}

function splitLines(text: string): string[] {
  return text.split("\n").map((line) => (line.endsWith("\r") ? line.slice(0, -1) : line));
}

// lineToInline is the seam Task 1b replaces. It turns one block's raw text
// (a heading's or paragraph's content, a table cell, a quote line) into
// Inline[]; today that is always exactly one text node carrying the text
// unchanged. 1b swaps this for real inline parsing (marks, links, escapes,
// macros) without the block walker above needing to change, since every
// call site already threads a single string through it.
function lineToInline(text: string): Inline[] {
  return [{ t: "text", text }];
}

function matchHeading(line: string): { level: 1 | 2 | 3 | 4 | 5 | 6; content: string } | null {
  if (line.length < 3 || line[0] !== "h") return null;
  const level = line.charCodeAt(1) - 48; // '0' is 48
  if (level < 1 || level > 6) return null;
  if (line[2] !== ".") return null;
  const start = line[3] === " " ? 4 : 3;
  return { level: level as 1 | 2 | 3 | 4 | 5 | 6, content: line.slice(start) };
}

function matchBq(line: string): string | null {
  if (line.startsWith("bq. ")) return line.slice(4);
  if (line === "bq.") return "";
  return null;
}

// matchBraceTag recognises "{name}" or "{name:attrs}" as an opening tag,
// but only when the closing "}" is the line's last character; it returns
// the attrs segment ("" for the bare "{name}" form) or null otherwise.
// Shared by matchCodeOpen and matchPanelOpen, the two tags that carry
// attributes on their opening line.
function matchBraceTag(line: string, name: string): string | null {
  if (!line.startsWith(name)) return null;
  const rest = line.slice(name.length);
  if (rest === "}") return "";
  if (rest[0] !== ":") return null;
  const closeIdx = rest.indexOf("}");
  if (closeIdx === -1 || closeIdx !== rest.length - 1) return null;
  return rest.slice(1, closeIdx);
}

function matchCodeOpen(line: string): { lang: string } | null {
  const attrs = matchBraceTag(line, "{code");
  if (attrs === null) return null;
  const pipeIdx = attrs.indexOf("|");
  return { lang: pipeIdx === -1 ? attrs : attrs.slice(0, pipeIdx) };
}

function matchPanelOpen(line: string): { title: string } | null {
  const attrs = matchBraceTag(line, "{panel");
  if (attrs === null) return null;
  let title = "";
  for (const part of attrs.split("|")) {
    if (part.startsWith("title=")) title = part.slice(6);
  }
  return { title };
}

// matchListMarker recognises a leading run of "*"/"#" followed by exactly
// one space as a list item; the same run with no trailing space (e.g. the
// inline mark "**bold**") is left for the paragraph fallback, and its
// characters for 1b to read as marks.
function matchListMarker(line: string): { markers: string; content: string } | null {
  let i = 0;
  while (i < line.length && (line[i] === "*" || line[i] === "#")) i++;
  if (i === 0 || i >= line.length || line[i] !== " ") return null;
  return { markers: line.slice(0, i), content: line.slice(i + 1) };
}

function isBlockStart(line: string): boolean {
  return (
    matchHeading(line) !== null ||
    matchBq(line) !== null ||
    line === "----" ||
    matchCodeOpen(line) !== null ||
    line === "{noformat}" ||
    line === "{quote}" ||
    matchPanelOpen(line) !== null ||
    line[0] === "|" ||
    matchListMarker(line) !== null
  );
}

type ListBlock = Extract<Block, { t: "list" }>;

// buildList turns a run of consecutive list-marker lines into a nested list
// tree, and can return more than one root: a top-level marker whose type
// (bullet/ordered) differs from the list currently open at that depth pops
// the stack to empty, same as any other depth/type mismatch, and an empty
// stack starts a new sibling root rather than reusing the old one. That is
// what makes "* a\n# b" (or a nested list followed by a top-level type
// switch) two lists in order instead of the second silently replacing the
// first. Depth is the marker run's length; the marker character at that
// depth (the last one in the run, e.g. the "*" in "#*") decides that level's
// own ordered/unordered type, so "#*" is a bullet item nested inside an
// ordered item.
function buildList(entries: { markers: string; content: string }[]): Block[] {
  const stack: { depth: number; ordered: boolean; block: ListBlock }[] = [];
  const roots: ListBlock[] = [];

  for (const entry of entries) {
    const depth = entry.markers.length;
    const ordered = entry.markers[depth - 1] === "#";

    while (
      stack.length > 0 &&
      (stack[stack.length - 1].depth > depth ||
        (stack[stack.length - 1].depth === depth && stack[stack.length - 1].ordered !== ordered))
    ) {
      stack.pop();
    }

    if (stack.length === 0 || stack[stack.length - 1].depth < depth) {
      const block: ListBlock = { t: "list", ordered, items: [] };
      if (stack.length === 0) {
        roots.push(block);
      } else {
        const parent = stack[stack.length - 1];
        const parentItem = parent.block.items[parent.block.items.length - 1];
        parentItem.children.push(block);
      }
      stack.push({ depth, ordered, block });
    }

    const top = stack[stack.length - 1];
    top.block.items.push({ children: [{ t: "p", children: lineToInline(entry.content) }] });
  }

  return roots;
}

// parseTableRow splits one "|"-prefixed line into cells. A cell opens with
// "||" (header) or "|" (normal) and its content always ends at the next "|",
// whatever cell that pipe goes on to open; that single rule is what makes a
// row like "||h1||h2|c1|" resolve to two header cells then one normal cell
// without special-casing the transition.
//
// ponytail: a cell's content is read verbatim up to the next "|", so a "|"
// that is actually inside a {code} span or a [text|url] link splits the row
// early. Table cells are Task 1b's, alongside real inline parsing: 1b scans
// each cell's raw text for a balanced {code}...{code} or [...] span before
// this splitter cuts on "|", so both stay intact.
function parseTableRow(line: string): { header: boolean; children: Inline[] }[] {
  const cells: { header: boolean; children: Inline[] }[] = [];
  const n = line.length;
  let i = 0;

  while (i < n) {
    let header = false;
    if (line[i + 1] === "|") {
      header = true;
      i += 2;
    } else {
      i += 1;
    }
    const start = i;
    while (i < n && line[i] !== "|") i++;
    if (start === i && i === n) break; // trailing closer, not another cell
    cells.push({ header, children: lineToInline(line.slice(start, i)) });
  }

  return cells;
}

function parseBlockLines(lines: string[], start: number, end: number): Block[] {
  const blocks: Block[] = [];
  let i = start;

  while (i < end) {
    const line = lines[i];

    if (line.trim() === "") {
      i++;
      continue;
    }

    const heading = matchHeading(line);
    if (heading) {
      blocks.push({ t: "h", level: heading.level, children: lineToInline(heading.content) });
      i++;
      continue;
    }

    const bq = matchBq(line);
    if (bq !== null) {
      blocks.push({ t: "quote", children: [{ t: "p", children: lineToInline(bq) }] });
      i++;
      continue;
    }

    if (line === "----") {
      blocks.push({ t: "rule" });
      i++;
      continue;
    }

    const codeOpen = matchCodeOpen(line);
    if (codeOpen) {
      let j = i + 1;
      while (j < end && lines[j] !== "{code}") j++;
      blocks.push({ t: "codeblock", lang: codeOpen.lang, text: lines.slice(i + 1, j).join("\n") });
      i = j < end ? j + 1 : end;
      continue;
    }

    if (line === "{noformat}") {
      let j = i + 1;
      while (j < end && lines[j] !== "{noformat}") j++;
      blocks.push({ t: "codeblock", lang: "", text: lines.slice(i + 1, j).join("\n") });
      i = j < end ? j + 1 : end;
      continue;
    }

    if (line === "{quote}") {
      let j = i + 1;
      while (j < end && lines[j] !== "{quote}") j++;
      blocks.push({ t: "quote", children: parseBlockLines(lines, i + 1, j) });
      i = j < end ? j + 1 : end;
      continue;
    }

    const panelOpen = matchPanelOpen(line);
    if (panelOpen) {
      let j = i + 1;
      while (j < end && lines[j] !== "{panel}") j++;
      blocks.push({ t: "panel", title: panelOpen.title, children: parseBlockLines(lines, i + 1, j) });
      i = j < end ? j + 1 : end;
      continue;
    }

    if (line[0] === "|") {
      const rows: { header: boolean; children: Inline[] }[][] = [];
      let j = i;
      while (j < end && lines[j][0] === "|") {
        rows.push(parseTableRow(lines[j]));
        j++;
      }
      blocks.push({ t: "table", rows });
      i = j;
      continue;
    }

    if (matchListMarker(line)) {
      const entries: { markers: string; content: string }[] = [];
      let j = i;
      while (j < end) {
        const marker = matchListMarker(lines[j]);
        if (!marker) break;
        entries.push(marker);
        j++;
      }
      blocks.push(...buildList(entries));
      i = j;
      continue;
    }

    // Paragraph: everything else, consuming consecutive non-blank lines
    // that do not start another block, so a paragraph never crosses a
    // blank line or a line that reads as a heading, list, table, etc.
    const paraLines: string[] = [];
    let j = i;
    while (j < end && lines[j].trim() !== "" && !isBlockStart(lines[j])) {
      paraLines.push(lines[j]);
      j++;
    }
    blocks.push({ t: "p", children: lineToInline(paraLines.join("\n")) });
    i = j;
  }

  return blocks;
}
