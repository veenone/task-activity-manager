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
    rowNodes.push(node("tableRow", { extra: attrsOf(r) }, cells.map((c) => node(c.nodeName === "th" ? "tableHeader" : "tableCell", {
      colspan: Number(c.getAttribute("colspan") ?? 1),
      rowspan: Number(c.getAttribute("rowspan") ?? 1),
      extra: attrsOf(c, ["colspan", "rowspan"]),
    }, nonEmpty(blockChildren(c))))));
  }
  return node("table", { extra: attrsOf(el), colgroup, tbody }, rowNodes);
}

// taskList reads ac:task-list. A task's own children are an optional task-id,
// any elements Confluence adds before the status (kept as raw XML in their
// place), the status, and the body. Any other shape stays opaque.
function taskList(el: Element): JSONContent {
  const tasks = elementChildren(el);
  if (!tasks || tasks.length === 0 || tasks.some((t) => t.nodeName !== "ac:task")) return opaque("opaqueBlock", el);
  const items: JSONContent[] = [];
  for (const t of tasks) {
    const parts = elementChildren(t);
    if (!parts) return opaque("opaqueBlock", el);
    let taskId = "";
    let checked = false;
    let extraXml = "";
    let sawStatus = false;
    let body: Element | null = null;
    for (const p of parts) {
      if (p.nodeName === "ac:task-id" && !sawStatus && !body) taskId = p.textContent ?? "";
      else if (p.nodeName === "ac:task-status" && !sawStatus && !body) {
        sawStatus = true;
        checked = (p.textContent ?? "").trim() === "complete";
      } else if (p.nodeName === "ac:task-body" && sawStatus && !body) body = p;
      else if (!sawStatus && !body) extraXml += outerXml(p);
      else return opaque("opaqueBlock", el);
    }
    if (!body) return opaque("opaqueBlock", el);
    items.push(node("taskItem", { checked, taskId, extraXml }, paragraphFirst(nonEmpty(blockChildren(body)))));
  }
  return node("taskList", { extra: attrsOf(el) }, items);
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
    if (name === "a" && n.hasAttribute("href")) {
      out.push(...inlineNodes(Array.from(n.childNodes), [...marks, { type: "link", attrs: { href: n.getAttribute("href"), extra: attrsOf(n, ["href"]) } }]));
      continue;
    }
    const mark = MARKS[name];
    if (mark && n.attributes.length === 0) {
      out.push(...inlineNodes(Array.from(n.childNodes), [...marks, TAGGED.has(mark) ? { type: mark, attrs: { tag: name } } : { type: mark }]));
      continue;
    }
    out.push(withMarks(opaque("opaqueInline", n)));
  }
  return out;
}
