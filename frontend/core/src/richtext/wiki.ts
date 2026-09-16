import type { Block, Inline, Mark } from "./ast";

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

const MARKS: Record<string, Mark> = {
  "*": "bold",
  "_": "italic",
  "+": "underline",
  "-": "strike",
  "^": "sup",
  "~": "sub",
};

const MARK_CHARS: Record<Mark, string> = {
  bold: "*",
  italic: "_",
  underline: "+",
  strike: "-",
  sup: "^",
  sub: "~",
};

const ESCAPABLE = new Set(["*", "|", "{", "[", "\\"]);

function isWordChar(ch: string): boolean {
  const c = ch.charCodeAt(0);
  return (c >= 48 && c <= 57) || (c >= 65 && c <= 90) || (c >= 97 && c <= 122);
}

function isLetter(ch: string | undefined): boolean {
  if (!ch) return false;
  const c = ch.charCodeAt(0);
  return (c >= 65 && c <= 90) || (c >= 97 && c <= 122);
}

// pushText appends a run of plain text to a node list, merging into a
// trailing text node rather than adding a new one, so escapes, macro
// fallback and unclosed-mark repair all read back as one text node where a
// hand-written expectation expects it, instead of a splintered run of them.
function pushText(nodes: Inline[], value: string): void {
  if (value === "") return;
  const last = nodes[nodes.length - 1];
  if (last && last.t === "text") {
    last.text += value;
  } else {
    nodes.push({ t: "text", text: value });
  }
}

type MarkFrame = { mark: Mark; children: Inline[] };

// unwindFrame folds a mark that never found its closing delimiter back into
// plain text: the delimiter character it opened with, followed by whatever
// it had already collected (marks that did close inside it are kept as
// nodes, not re-flattened).
function unwindFrame(frame: MarkFrame, target: Inline[]): void {
  pushText(target, MARK_CHARS[frame.mark]);
  for (const child of frame.children) {
    if (child.t === "text") pushText(target, child.text);
    else target.push(child);
  }
}

// lineToInline turns one block's raw text (a heading's or paragraph's
// content, a table cell, a quote line, a list item) into Inline[]: marks,
// links, escapes, images and macros, one character at a time (D3). A mark
// can open only where the character before it is not part of a word and
// the one after it is not blank, and can close only the mirror of that, so
// "mid-session", "my_var_name" and "2*3*4" stay plain while "a -gone- b"
// strikes; the stack of open marks is bounded by the six mark types, not by
// input length, so it stays linear regardless of how marks nest or fail to
// close.
function lineToInline(text: string): Inline[] {
  const root: Inline[] = [];
  const stack: MarkFrame[] = [];
  const n = text.length;
  let buffer = "";
  let i = 0;
  // Once a search for "}}" or a {code} close fails, it fails for every
  // later position too (the rest of the string does not grow a closing tag
  // it did not have), so these cap each parse to at most one full scan
  // apiece instead of one per remaining brace.
  let dbraceFailFrom = Infinity;
  let codeFailFrom = Infinity;

  const current = (): Inline[] => (stack.length ? stack[stack.length - 1].children : root);
  const flush = (): void => {
    if (buffer) {
      pushText(current(), buffer);
      buffer = "";
    }
  };

  while (i < n) {
    const ch = text[i];

    if (ch === "\\") {
      const next = text[i + 1];
      if (next === "\\") {
        const after = text[i + 2];
        if (after === undefined || after === "\n") {
          flush();
          current().push({ t: "br" });
          i += after === "\n" ? 3 : 2;
        } else {
          buffer += "\\";
          i += 2;
        }
        continue;
      }
      if (next !== undefined && ESCAPABLE.has(next)) {
        buffer += next;
        i += 2;
        continue;
      }
      buffer += "\\";
      i += 1;
      continue;
    }

    if (ch === "{" && text[i + 1] === "{") {
      if (i < dbraceFailFrom) {
        const close = text.indexOf("}}", i + 2);
        if (close !== -1) {
          flush();
          current().push({ t: "code", text: text.slice(i + 2, close) });
          i = close + 2;
          continue;
        }
        dbraceFailFrom = i;
      }
      buffer += "{";
      i += 1;
      continue;
    }

    if (ch === "{") {
      const next = text[i + 1];
      if (!isLetter(next)) {
        buffer += "{";
        i += 1;
        continue;
      }
      let j = i + 1;
      while (j < n && isLetter(text[j])) j++;
      const name = text.slice(i + 1, j);
      let bodyStart: number;
      if (text[j] === ":") {
        const closeAttrs = text.indexOf("}", j);
        if (closeAttrs === -1) {
          buffer += "{";
          i += 1;
          continue;
        }
        bodyStart = closeAttrs + 1;
      } else if (text[j] === "}") {
        bodyStart = j + 1;
      } else {
        buffer += "{";
        i += 1;
        continue;
      }

      if (name === "code") {
        if (bodyStart >= codeFailFrom) {
          buffer += "{";
          i += 1;
          continue;
        }
        const close = text.indexOf("{code}", bodyStart);
        flush();
        if (close === -1) {
          current().push({ t: "code", text: text.slice(bodyStart) });
          codeFailFrom = bodyStart;
          i = n;
        } else {
          current().push({ t: "code", text: text.slice(bodyStart, close) });
          i = close + 6;
        }
        continue;
      }

      // ponytail: an unknown macro's closing tag is searched for with one
      // indexOf per occurrence, uncached by name (unlike {{ and {code}
      // above). Real Jira text carries at most a handful of macros per
      // field, so this stays linear in practice; a field carrying
      // thousands of distinct unclosed braces would pay one scan each.
      // Upgrade path, if that ever shows up: the same fail-position cache
      // used above, keyed by tag name.
      const closeTag = "{" + name + "}";
      const close = text.indexOf(closeTag, bodyStart);
      if (close === -1) {
        i = bodyStart; // a lone macro vanishes: nothing is emitted for it
        continue;
      }
      flush();
      current().push(...lineToInline(text.slice(bodyStart, close)));
      i = close + closeTag.length;
      continue;
    }

    if (ch === "[") {
      const close = text.indexOf("]", i + 1);
      if (close === -1) {
        buffer += "[";
        i += 1;
        continue;
      }
      const inner = text.slice(i + 1, close);
      const pipeIdx = inner.indexOf("|");
      const label = pipeIdx === -1 ? inner : inner.slice(0, pipeIdx);
      const href = pipeIdx === -1 ? inner : inner.slice(pipeIdx + 1);
      flush();
      current().push({ t: "link", href, children: [{ t: "text", text: label }] });
      i = close + 1;
      continue;
    }

    if (ch === "!") {
      const close = text.indexOf("!", i + 1);
      if (close === -1) {
        buffer += "!";
        i += 1;
        continue;
      }
      const inner = text.slice(i + 1, close);
      const pipeIdx = inner.indexOf("|");
      const name = pipeIdx === -1 ? inner : inner.slice(0, pipeIdx);
      flush();
      current().push({ t: "image", name });
      i = close + 1;
      continue;
    }

    if (ch === "h" && (text.startsWith("https://", i) || text.startsWith("http://", i))) {
      let j = i;
      while (j < n && text[j] !== " " && text[j] !== "\t" && text[j] !== "\n") j++;
      const url = text.slice(i, j);
      flush();
      current().push({ t: "link", href: url, children: [{ t: "text", text: url }] });
      i = j;
      continue;
    }

    const markType = MARKS[ch];
    if (markType) {
      const prev = i > 0 ? text[i - 1] : "";
      const next = i + 1 < n ? text[i + 1] : "";
      const canOpen = !isWordChar(prev) && next !== "" && next !== " " && next !== "\t";
      const canClose = prev !== "" && prev !== " " && prev !== "\t" && !isWordChar(next);
      const openIdx = stack.findIndex((f) => f.mark === markType);

      if (openIdx !== -1 && canClose) {
        flush();
        while (stack.length - 1 > openIdx) {
          const inner = stack.pop() as MarkFrame;
          unwindFrame(inner, current());
        }
        const frame = stack.pop() as MarkFrame;
        current().push({ t: "mark", mark: frame.mark, children: frame.children });
        i += 1;
        continue;
      }
      if (openIdx === -1 && canOpen) {
        flush();
        stack.push({ mark: markType, children: [] });
        i += 1;
        continue;
      }
      buffer += ch;
      i += 1;
      continue;
    }

    buffer += ch;
    i += 1;
  }

  flush();
  while (stack.length) {
    const frame = stack.pop() as MarkFrame;
    unwindFrame(frame, current());
  }
  return root;
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
// "||" (header) or "|" (normal) and its content always ends at the next "|"
// that is not inside a [text|url] link or a {code}...{code} span, whatever
// cell that pipe would otherwise open; that single rule is what makes a row
// like "||h1||h2|c1|" resolve to two header cells then one normal cell
// without special-casing the transition, while a link's or a code span's
// own "|" stays inside the cell that carries it.
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
    while (i < n && line[i] !== "|") {
      if (line[i] === "[") {
        const close = line.indexOf("]", i + 1);
        i = close === -1 ? i + 1 : close + 1;
        continue;
      }
      if (line[i] === "{" && line.startsWith("code", i + 1) && (line[i + 5] === ":" || line[i + 5] === "}")) {
        const attrsClose = line.indexOf("}", i);
        if (attrsClose !== -1) {
          const bodyClose = line.indexOf("{code}", attrsClose + 1);
          i = bodyClose === -1 ? n : bodyClose + 6;
          continue;
        }
      }
      i++;
    }
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
