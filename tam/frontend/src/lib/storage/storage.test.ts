import { describe, expect, it } from "vitest";
import type { JSONContent } from "@tiptap/core";
import { parseStorage } from "./parse";
import { serializeStorage } from "./serialize";
import { normalizeStorage } from "./normalize";
import { corpus, handWritten, templates } from "./corpus";

function roundTrip(body: string): string {
  const parsed = parseStorage(body);
  if (!parsed.ok) throw new Error(parsed.reason);
  return serializeStorage(parsed.doc);
}

function findAll(node: JSONContent, type: string, out: JSONContent[] = []): JSONContent[] {
  if (node.type === type) out.push(node);
  for (const child of node.content ?? []) findAll(child, type, out);
  return out;
}

describe("storage round trip", () => {
  for (const [name, body] of Object.entries(corpus)) {
    it(`round-trips ${name}`, () => {
      expect(normalizeStorage(roundTrip(body))).toBe(normalizeStorage(body));
    });
  }

  it("keeps opaque XML byte for byte", () => {
    const out = roundTrip(handWritten.codeMacro);
    expect(out).toContain('<ac:structured-macro ac:name="code" ac:schema-version="1" ac:macro-id="abc">');
    expect(out).toContain("<![CDATA[if a < b && c > d { return }]]>");
    expect(out).not.toContain("xmlns");
  });

  it("labels a macro by its name and keeps a layout whole", () => {
    const parsed = parseStorage(templates.standup);
    if (!parsed.ok) throw new Error(parsed.reason);
    expect(findAll(parsed.doc, "opaqueBlock").map((n) => n.attrs?.label)).toContain("jira");
    const layout = parseStorage(handWritten.layout);
    if (!layout.ok) throw new Error(layout.reason);
    expect(layout.doc.content).toHaveLength(1);
    expect(layout.doc.content?.[0].attrs?.label).toBe("ac:layout");
  });

  it("maps marks and links onto editable text", () => {
    const parsed = parseStorage(`<p>a <strong>b <em>c</em></strong></p>`);
    if (!parsed.ok) throw new Error(parsed.reason);
    expect(parsed.doc.content?.[0]).toEqual({
      type: "paragraph",
      attrs: { extra: null },
      content: [
        { type: "text", text: "a " },
        { type: "text", text: "b ", marks: [{ type: "bold", attrs: { tag: "strong" } }] },
        { type: "text", text: "c", marks: [{ type: "bold", attrs: { tag: "strong" } }, { type: "italic", attrs: { tag: "em" } }] },
      ],
    });
  });

  it("reads a task's status, id and body", () => {
    const parsed = parseStorage(handWritten.nestedTasks);
    if (!parsed.ok) throw new Error(parsed.reason);
    const [parent, child] = findAll(parsed.doc, "taskItem");
    expect(parent.attrs).toEqual({ checked: true, taskId: "1", extraXml: "" });
    expect(child.attrs).toEqual({ checked: false, taskId: "2", extraXml: "" });
  });

  it("reports a page it cannot read instead of guessing", () => {
    expect(parseStorage("<p>unclosed").ok).toBe(false);
    expect(parseStorage("<p>&bogus;</p>").ok).toBe(false);
  });

  it("writes two bare paragraphs an edit put side by side as real paragraphs", () => {
    const doc: JSONContent = { type: "doc", content: [
      { type: "paragraph", attrs: { bare: true }, content: [{ type: "text", text: "a" }] },
      { type: "paragraph", attrs: { bare: true }, content: [{ type: "text", text: "b" }] },
    ] };
    expect(serializeStorage(doc)).toBe("<p>a</p><p>b</p>");
  });

  it("writes what the editor creates in the shapes Confluence expects", () => {
    const doc: JSONContent = { type: "doc", content: [
      { type: "taskList", content: [{ type: "taskItem", attrs: { checked: false }, content: [{ type: "paragraph", content: [{ type: "text", text: "call infra" }] }] }] },
      { type: "table", content: [{ type: "tableRow", content: [{ type: "tableHeader", attrs: { colspan: 1, rowspan: 1 }, content: [{ type: "paragraph" }] }] }] },
    ] };
    expect(serializeStorage(doc)).toBe(
      "<ac:task-list><ac:task><ac:task-status>incomplete</ac:task-status><ac:task-body><p>call infra</p></ac:task-body></ac:task></ac:task-list>" +
      "<table><tbody><tr><th><p></p></th></tr></tbody></table>",
    );
  });
});
