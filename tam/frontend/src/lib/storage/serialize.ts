import type { JSONContent } from "@tiptap/core";
import { escapeAttr, escapeText } from "./xml";

type Mark = { type: string; attrs?: Record<string, unknown> };

// serializeStorage is parseStorage's inverse. Opaque nodes print the XML they
// were read from, unchanged; everything else is written in the shape
// Confluence writes itself.
export function serializeStorage(doc: JSONContent): string {
  return blocks(doc.content ?? []);
}

function attrs(extra: unknown, more: Record<string, string> = {}): string {
  const all: Record<string, string> = { ...((extra as Record<string, string> | null) ?? {}), ...more };
  return Object.entries(all).map(([k, v]) => ` ${k}="${escapeAttr(String(v))}"`).join("");
}

const isBare = (n: JSONContent | undefined) => n?.type === "paragraph" && n.attrs?.bare === true;

// A bare paragraph is written without <p> only when no other bare paragraph
// sits beside it. Parsing never puts two side by side; an edit that did
// would otherwise run their text together.
function blocks(nodes: JSONContent[]): string {
  return nodes.map((n, i) => (isBare(n) && !isBare(nodes[i - 1]) && !isBare(nodes[i + 1]) ? inline(n.content ?? []) : block(n))).join("");
}

function block(n: JSONContent): string {
  const a = n.attrs ?? {};
  const kids = n.content ?? [];
  switch (n.type) {
    case "paragraph": return `<p${attrs(a.extra)}>${inline(kids)}</p>`;
    case "heading": return `<h${a.level}${attrs(a.extra)}>${inline(kids)}</h${a.level}>`;
    case "bulletList": return `<ul${attrs(a.extra)}>${blocks(kids)}</ul>`;
    case "orderedList": return `<ol${attrs(a.extra)}>${blocks(kids)}</ol>`;
    case "listItem": return `<li${attrs(a.extra)}>${blocks(kids)}</li>`;
    case "blockquote": return `<blockquote${attrs(a.extra)}>${blocks(kids)}</blockquote>`;
    case "horizontalRule": return "<hr />";
    case "table": {
      const rows = blocks(kids);
      // A table made in the editor carries no tbody attribute; Confluence
      // writes one, so absent means yes.
      const tbody = a.tbody !== false;
      return `<table${attrs(a.extra)}>${a.colgroup ?? ""}${tbody ? `<tbody>${rows}</tbody>` : rows}</table>`;
    }
    case "tableRow": return `<tr${attrs(a.extra)}>${blocks(kids)}</tr>`;
    case "tableCell":
    case "tableHeader": {
      const tag = n.type === "tableHeader" ? "th" : "td";
      const spans: Record<string, string> = {};
      if (a.colspan && a.colspan !== 1) spans.colspan = String(a.colspan);
      if (a.rowspan && a.rowspan !== 1) spans.rowspan = String(a.rowspan);
      return `<${tag}${attrs(a.extra, spans)}>${blocks(kids)}</${tag}>`;
    }
    case "taskList": return `<ac:task-list${attrs(a.extra)}>${blocks(kids)}</ac:task-list>`;
    case "taskItem": {
      // taskId null or undefined means parsing never saw a task-id element
      // at all (the editor's schema fills the gap with null, its declared
      // default, once a page has been through it); "" means it saw one that
      // was genuinely empty (<ac:task-id/>) and must write one back rather
      // than silently dropping it. Testing the type, not just !== undefined,
      // is what keeps the schema's own default from being written back out.
      const id = typeof a.taskId === "string" ? `<ac:task-id>${escapeText(a.taskId)}</ac:task-id>` : "";
      return `<ac:task>${id}${a.extraXml ?? ""}<ac:task-status>${a.checked ? "complete" : "incomplete"}</ac:task-status><ac:task-body>${blocks(kids)}</ac:task-body></ac:task>`;
    }
    case "opaqueBlock": return String(a.xml ?? "");
  }
  return "";
}

function tagOf(m: Mark, fallback: string): string {
  const tag = m.attrs?.tag;
  return typeof tag === "string" && tag ? tag : fallback;
}

function openTag(m: Mark): string {
  switch (m.type) {
    case "bold": return `<${tagOf(m, "strong")}>`;
    case "italic": return `<${tagOf(m, "em")}>`;
    case "strike": return `<${tagOf(m, "s")}>`;
    case "underline": return "<u>";
    case "code": return "<code>";
    case "link": return `<a${attrs(m.attrs?.extra, { href: String(m.attrs?.href ?? "") })}>`;
  }
  return "";
}

function closeTag(m: Mark): string {
  switch (m.type) {
    case "bold": return `</${tagOf(m, "strong")}>`;
    case "italic": return `</${tagOf(m, "em")}>`;
    case "strike": return `</${tagOf(m, "s")}>`;
    case "underline": return "</u>";
    case "code": return "</code>";
    case "link": return "</a>";
  }
  return "";
}

// markKey compares only what a mark writes, so a link the editor decorated
// with its own target and rel still closes and reopens where the page did.
function markKey(m: Mark): string {
  return `${m.type}|${m.type === "link" ? `${m.attrs?.href}|${JSON.stringify(m.attrs?.extra ?? null)}` : tagOf(m, "")}`;
}

// inline keeps a stack of open marks and closes only the ones that end, so
// "<strong>b <em>c</em></strong>" comes back as it went in rather than as a
// strong per text node.
function inline(nodes: JSONContent[]): string {
  let out = "";
  let open: Mark[] = [];
  for (const n of nodes) {
    const marks = ((n.marks ?? []) as Mark[]).filter((m) => openTag(m) !== "");
    let common = 0;
    while (common < open.length && common < marks.length && markKey(open[common]) === markKey(marks[common])) common++;
    for (let i = open.length - 1; i >= common; i--) out += closeTag(open[i]);
    for (let i = common; i < marks.length; i++) out += openTag(marks[i]);
    open = marks;
    if (n.type === "text") out += escapeText(n.text ?? "");
    else if (n.type === "hardBreak") out += "<br />";
    else if (n.type === "opaqueInline") out += String(n.attrs?.xml ?? "");
  }
  for (let i = open.length - 1; i >= 0; i--) out += closeTag(open[i]);
  return out;
}
