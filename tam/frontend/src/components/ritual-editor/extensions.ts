import { Extension, Node as TiptapNode, mergeAttributes } from "@tiptap/core";
import { ReactNodeViewRenderer } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import { Table, TableCell, TableHeader, TableRow } from "@tiptap/extension-table";
import { TaskItem, TaskList } from "@tiptap/extension-list";
import { OpaqueBlockView, OpaqueInlineView } from "./OpaqueViews";

// hidden declares an attribute lib/storage writes that the page's HTML never
// shows. keepOnSplit is off so a new paragraph or task made by pressing
// Enter does not inherit a bare flag, an attribute bag, or a task id.
const hidden = (fallback: unknown) => ({ default: fallback, rendered: false, keepOnSplit: false });

// StorageAttributes declares every attribute lib/storage/parse puts on a
// node or a mark. ProseMirror silently drops an attribute its schema does
// not declare, so without this a page would lose its colgroup, its task ids,
// and every attribute TAM does not model the first time anybody edited it.
const StorageAttributes = Extension.create({
  name: "storageAttributes",
  addGlobalAttributes() {
    return [
      { types: ["paragraph"], attributes: { bare: hidden(null) } },
      { types: ["paragraph", "heading", "bulletList", "orderedList", "listItem", "blockquote", "tableRow", "tableCell", "tableHeader", "taskList"], attributes: { extra: hidden(null) } },
      { types: ["table"], attributes: { extra: hidden(null), colgroup: hidden(""), tbody: hidden(true) } },
      // taskId defaults to null, not "": a task with no <ac:task-id> at all
      // parses with taskId left undefined, and the schema must fill that gap
      // with a value serialize.ts also reads as "no id", or the very first
      // schema round trip (opening the page in this editor) would write an
      // empty <ac:task-id/> onto every task that never had one.
      { types: ["taskItem"], attributes: { taskId: hidden(null), extraXml: hidden("") } },
      { types: ["bold", "italic", "strike"], attributes: { tag: hidden(null) } },
      { types: ["link"], attributes: { extra: hidden(null) } },
    ];
  },
});

const xmlAttributes = () => ({
  xml: { default: "", parseHTML: (el: HTMLElement) => el.getAttribute("data-xml") ?? "", renderHTML: (a: Record<string, unknown>) => ({ "data-xml": a.xml }) },
  label: { default: "", parseHTML: (el: HTMLElement) => el.getAttribute("data-label") ?? "", renderHTML: (a: Record<string, unknown>) => ({ "data-label": a.label }) },
});

// OpaqueBlock is content TAM does not model: a macro, a layout, anything.
// It can be moved and deleted whole, and never edited inside.
export const OpaqueBlock = TiptapNode.create({
  name: "opaqueBlock",
  group: "block",
  atom: true,
  selectable: true,
  draggable: true,
  addAttributes: xmlAttributes,
  parseHTML: () => [{ tag: "div[data-opaque-block]" }],
  renderHTML: ({ HTMLAttributes }) => ["div", mergeAttributes(HTMLAttributes, { "data-opaque-block": "" })],
  addNodeView: () => ReactNodeViewRenderer(OpaqueBlockView),
});

export const OpaqueInline = TiptapNode.create({
  name: "opaqueInline",
  group: "inline",
  inline: true,
  atom: true,
  selectable: true,
  addAttributes: xmlAttributes,
  parseHTML: () => [{ tag: "span[data-opaque-inline]" }],
  renderHTML: ({ HTMLAttributes }) => ["span", mergeAttributes(HTMLAttributes, { "data-opaque-inline": "" })],
  addNodeView: () => ReactNodeViewRenderer(OpaqueInlineView),
});

export function ritualExtensions() {
  return [
    // No code block (Confluence writes code as a macro, which stays opaque)
    // and no trailing node: that plugin appends a paragraph on load, which
    // would change a page nobody touched.
    StarterKit.configure({
      heading: { levels: [1, 2, 3, 4, 5, 6] },
      codeBlock: false,
      trailingNode: false,
      link: { openOnClick: false, autolink: false },
    }),
    Table.configure({ resizable: false }),
    TableRow,
    TableHeader,
    TableCell,
    TaskList,
    TaskItem.configure({ nested: true }),
    OpaqueBlock,
    OpaqueInline,
    StorageAttributes,
  ];
}
