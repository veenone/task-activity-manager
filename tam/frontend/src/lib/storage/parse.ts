import type { JSONContent } from "@tiptap/core";
import { isBlank, outerXml, parseXml } from "./xml";

export type ParseResult = { ok: true; doc: JSONContent } | { ok: false; reason: string };

type Mark = { type: string; attrs?: Record<string, unknown> };
type Attrs = Record<string, string> | null;

// Elements that always start a block of their own.
const BLOCK = new Set(["p", "h1", "h2", "h3", "h4", "h5", "h6", "ul", "ol", "table", "blockquote", "hr", "ac:task-list"]);
// Elements that always sit inside a line of text. Anything in neither set
// (a macro, an image, an element nobody anticipated) is placed by its
// neighbours: inline beside text, a block of its own otherwise.
const INLINE = new Set(["strong", "b", "em", "i", "u", "s", "del", "code", "a", "br", "span", "sub", "sup", "ac:link", "ac:emoticon", "time", "ac:inline-comment-marker"]);
const MARKS: Record<string, string> = { strong: "bold", b: "bold", em: "italic", i: "italic", u: "underline", s: "strike", del: "strike", code: "code" };
const TAGGED = new Set(["bold", "italic", "strike"]);

// parseStorage turns a page into the editor's document. Every element it
// does not model becomes an opaque node holding its raw XML, so nothing on a
// page is ever dropped: the fallback is the rule, not a list of known
// unknowns.
export function parseStorage(body: string): ParseResult {
  const root = parseXml(body);
  if (!root) return { ok: false, reason: "The page is not well-formed storage format." };
  return { ok: true, doc: { type: "doc", content: nonEmpty(blockChildren(root)) } };
}

function node(type: string, attrs?: Record<string, unknown> | null, content?: JSONContent[]): JSONContent {
  const n: JSONContent = { type };
  if (attrs) n.attrs = attrs;
  if (content && content.length) n.content = content;
  return n;
}

const isElement = (n: Node): n is Element => n.nodeType === Node.ELEMENT_NODE;
const isText = (n: Node) => n.nodeType === Node.TEXT_NODE;

function attrsOf(el: Element, skip: string[] = []): Attrs {
  const out: Record<string, string> = {};
  for (const a of Array.from(el.attributes)) if (!skip.includes(a.name)) out[a.name] = a.value;
  return Object.keys(out).length ? out : null;
}

function label(n: Node): string {
  if (!isElement(n)) return n.nodeType === Node.COMMENT_NODE ? "comment" : "content";
  if (n.nodeName === "ac:structured-macro") return n.getAttribute("ac:name") ?? "macro";
  return n.nodeName;
}

function opaque(type: "opaqueBlock" | "opaqueInline", n: Node): JSONContent {
  return node(type, { xml: outerXml(n), label: label(n) });
}

// A container whose blocks the editor requires at least one of.
const nonEmpty = (blocks: JSONContent[]) => (blocks.length ? blocks : [node("paragraph", { bare: true })]);
// A list item and a task item must open with a paragraph; a bare empty one
// writes back as nothing.
const paragraphFirst = (blocks: JSONContent[]) =>
  blocks[0]?.type === "paragraph" ? blocks : [node("paragraph", { bare: true }), ...blocks];

function definitelyInline(n: Node): boolean {
  if (isText(n)) return !isBlank(n.textContent ?? "");
  if (!isElement(n)) return false;
  return INLINE.has(n.nodeName) || n.nodeName.startsWith("ri:");
}

// blockChildren reads a container that holds blocks. A run of inline content
// between blocks becomes a bare paragraph, written back without <p>, which
// is how "<li>Item<ul>...</ul></li>" survives. A run holding no text is its
// elements, each a block of its own; blank text between blocks is dropped.
function blockChildren(parent: Element): JSONContent[] {
  const out: JSONContent[] = [];
  let run: Node[] = [];
  const flush = () => {
    if (run.some(definitelyInline)) out.push(node("paragraph", { bare: true }, inlineNodes(run, [])));
    else for (const n of run) if (isElement(n) || n.nodeType === Node.COMMENT_NODE || n.nodeType === Node.CDATA_SECTION_NODE) out.push(opaque("opaqueBlock", n));
    run = [];
  };
  for (const child of Array.from(parent.childNodes)) {
    if (isElement(child) && BLOCK.has(child.nodeName)) {
      flush();
      out.push(block(child));
    } else {
      run.push(child);
    }
  }
  flush();
  return out;
}

function elementChildren(el: Element): Element[] | null {
  const out: Element[] = [];
  for (const c of Array.from(el.childNodes)) {
    if (isElement(c)) out.push(c);
    else if (isText(c) && isBlank(c.textContent ?? "")) continue;
    else return null;
  }
  return out;
}

function block(el: Element): JSONContent {
  const name = el.nodeName;
  if (name === "p") return node("paragraph", { extra: attrsOf(el) }, inlineNodes(Array.from(el.childNodes), []));
  if (/^h[1-6]$/.test(name)) return node("heading", { level: Number(name[1]), extra: attrsOf(el) }, inlineNodes(Array.from(el.childNodes), []));
  if (name === "ul" || name === "ol") return list(el);
  if (name === "blockquote") return node("blockquote", { extra: attrsOf(el) }, nonEmpty(blockChildren(el)));
  if (name === "hr") return el.attributes.length || el.childNodes.length ? opaque("opaqueBlock", el) : node("horizontalRule");
  if (name === "table") return table(el);
  if (name === "ac:task-list") return taskList(el);
  return opaque("opaqueBlock", el);
}

function list(el: Element): JSONContent {
  const items = elementChildren(el);
  if (!items || items.length === 0 || items.some((i) => i.nodeName !== "li")) return opaque("opaqueBlock", el);
  return node(el.nodeName === "ul" ? "bulletList" : "orderedList", { extra: attrsOf(el) },
    items.map((li) => node("listItem", { extra: attrsOf(li) }, paragraphFirst(nonEmpty(blockChildren(li))))));
}

// table models the shape Confluence writes: an optional colgroup, then rows
// directly or inside one tbody, cells of td and th. Anything else (thead, a
// caption, a second tbody) keeps the whole table opaque rather than
// rearranging it.
function table(el: Element): JSONContent {
  const children = elementChildren(el);
  if (!children) return opaque("opaqueBlock", el);
  let colgroup = "";
  let tbody = false;
  let rows: Element[] = [];
  for (const c of children) {
    if (c.nodeName === "colgroup" && !colgroup && !tbody && rows.length === 0) {
      colgroup = outerXml(c);
    } else if (c.nodeName === "tbody" && !tbody && rows.length === 0 && c.attributes.length === 0) {
      const inner = elementChildren(c);
      if (!inner) return opaque("opaqueBlock", el);
      tbody = true;
      rows = inner;
    } else if (c.nodeName === "tr" && !tbody) {
      rows.push(c);
    } else {
      return opaque("opaqueBlock", el);
    }
  }
  if (rows.length === 0 || rows.some((r) => r.nodeName !== "tr")) return opaque("opaqueBlock", el);
  const rowNodes: JSONContent[] = [];
  for (const r of rows) {
    const cells = elementChildren(r);
    if (!cells || cells.length === 0 || cells.some((c) => c.nodeName !== "td" && c.nodeName !== "th")) return opaque("opaqueBlock", el);
    rowNodes.push(node("tableRow", { extra: attrsOf(r) }, cells.map((c) => {
      const colspan = cellSpan(c.getAttribute("colspan"));
      const rowspan = cellSpan(c.getAttribute("rowspan"));
      const skip: string[] = [];
      if (colspan.lifted) skip.push("colspan");
      if (rowspan.lifted) skip.push("rowspan");
      return node(c.nodeName === "th" ? "tableHeader" : "tableCell", {
        colspan: colspan.value,
        rowspan: rowspan.value,
        extra: attrsOf(c, skip),
      }, nonEmpty(blockChildren(c)));
    })));
  }
  return node("table", { extra: attrsOf(el), colgroup, tbody }, rowNodes);
}

// cellSpan only lifts a colspan/rowspan value into the numeric attr the
// editor reads when it is exactly a plain positive integer greater than 1: a
// raw "1" is redundant but authored, "0" and "abc" are not integers at all,
// and " 2" carries whitespace no editor wrote. Anything not lifted stays in
// extra exactly as written rather than being rounded, defaulted away, or
// silently rewritten.
function cellSpan(raw: string | null): { value: number; lifted: boolean } {
  if (raw !== null && raw !== "1" && /^[1-9]\d*$/.test(raw)) return { value: Number(raw), lifted: true };
  return { value: 1, lifted: false };
}

// taskList reads ac:task-list. A task's own children are an optional task-id
// (nothing before it), any elements Confluence adds between the id and the
// status (kept as raw XML in their place), the status, and the body. An
// attribute anywhere in the four elements, a task-id or task-status holding
// anything but a single text node (an empty task-id is the one exception), a
// status that is not exactly "complete" or "incomplete", or an unknown
// element ahead of task-id are all shapes this cannot hold exactly, so the
// whole list stays opaque rather than dropping or rewriting a byte of it.
function taskList(el: Element): JSONContent {
  const tasks = elementChildren(el);
  if (!tasks || tasks.length === 0 || tasks.some((t) => t.nodeName !== "ac:task")) return opaque("opaqueBlock", el);
  const items: JSONContent[] = [];
  for (const t of tasks) {
    const item = taskItem(t);
    if (!item) return opaque("opaqueBlock", el);
    items.push(item);
  }
  return node("taskList", { extra: attrsOf(el) }, items);
}

// singleTextChild reads an element meant to hold nothing but text: null
// means the element cannot be read exactly (an unexpected child, whether
// that is a comment, another element, or more than one text node).
// allowEmpty lets task-id, and only task-id, be genuinely empty.
function singleTextChild(el: Element, allowEmpty: boolean): string | null {
  const kids = Array.from(el.childNodes);
  if (kids.length === 0) return allowEmpty ? "" : null;
  return kids.length === 1 && kids[0].nodeType === Node.TEXT_NODE ? kids[0].textContent ?? "" : null;
}

function taskItem(t: Element): JSONContent | null {
  if (t.attributes.length > 0) return null;
  const parts = elementChildren(t);
  if (!parts) return null;
  const idIndex = parts.findIndex((p) => p.nodeName === "ac:task-id");
  if (idIndex > 0) return null;
  let taskId: string | undefined;
  let i = 0;
  if (idIndex === 0) {
    const idEl = parts[0];
    if (idEl.attributes.length > 0) return null;
    const text = singleTextChild(idEl, true);
    if (text === null) return null;
    taskId = text;
    i = 1;
  }
  let extraXml = "";
  while (i < parts.length && parts[i].nodeName !== "ac:task-status") extraXml += outerXml(parts[i++]);
  const statusEl = parts[i];
  if (!statusEl || statusEl.attributes.length > 0) return null;
  const statusText = singleTextChild(statusEl, false);
  let checked: boolean;
  if (statusText === "complete") checked = true;
  else if (statusText === "incomplete") checked = false;
  else return null;
  const bodyEl = parts[i + 1];
  if (!bodyEl || bodyEl.nodeName !== "ac:task-body" || bodyEl.attributes.length > 0 || i + 2 !== parts.length) return null;
  return node("taskItem", { checked, taskId, extraXml }, paragraphFirst(nonEmpty(blockChildren(bodyEl))));
}

function inlineNodes(nodes: Node[], marks: Mark[]): JSONContent[] {
  const out: JSONContent[] = [];
  const withMarks = (n: JSONContent): JSONContent => (marks.length ? { ...n, marks } : n);
  for (const n of nodes) {
    if (isText(n)) {
      const text = n.textContent ?? "";
      if (text) out.push(withMarks({ type: "text", text }));
      continue;
    }
    if (!isElement(n)) {
      out.push(withMarks(opaque("opaqueInline", n)));
      continue;
    }
    const name = n.nodeName;
    if (name === "br" && n.attributes.length === 0 && n.childNodes.length === 0) {
      out.push(withMarks({ type: "hardBreak" }));
      continue;
    }
    // An empty mark or link has no text to carry the mark on, so there is
    // nothing to map it onto: keep it opaque rather than dropping it (and,
    // with it, whichever side of a and b in "a<strong></strong>b" it sat
    // between).
    if (name === "a" && n.hasAttribute("href") && n.childNodes.length > 0) {
      out.push(...inlineNodes(Array.from(n.childNodes), [...marks, { type: "link", attrs: { href: n.getAttribute("href"), extra: attrsOf(n, ["href"]) } }]));
      continue;
    }
    const mark = MARKS[name];
    if (mark && n.attributes.length === 0 && n.childNodes.length > 0) {
      out.push(...inlineNodes(Array.from(n.childNodes), [...marks, TAGGED.has(mark) ? { type: mark, attrs: { tag: name } } : { type: mark }]));
      continue;
    }
    out.push(withMarks(opaque("opaqueInline", n)));
  }
  return out;
}
