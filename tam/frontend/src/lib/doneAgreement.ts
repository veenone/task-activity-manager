import { NAMESPACES, parseXml } from "./storage/xml";

// The done agreement's items, read out of the document body the rituals
// machinery stores. This is the one parse of them, and it is here rather
// than in Go for two reasons. The XML reader it needs already exists in
// lib/storage, which the ritual editor round trips pages with; and the
// sprint report's own document, which shows these items with a count per
// issue, is built in TypeScript in lib/reportDocument, so a parse in Go
// would be answered by a second one on this side anyway. Go stores a tick
// against an item's text and never needs to know what an item is.
//
// An item is identified by its text, because an ac:task carries nothing else
// that survives an edit: its ac:task-id is Confluence's, absent until the
// page is published and not stable across a rewrite of the list. So the
// words are the key, which is why they are normalized here, once: ASCII
// whitespace collapsed, ends trimmed. A non-breaking space is left alone,
// the way lib/storage/xml treats it, because in a page it is a character
// somebody typed.
//
// The document's own ac:task-status is ignored. That box is the team's
// template, not a judgement about any one issue; a tick against an issue is
// a row in TAM's store.

const WHITESPACE = /[ \t\r\n]+/g;

// itemText is one task's own words. A task list nested under a task belongs
// to the items it holds, not to the words of the item holding them: reading
// it into the parent's text would change the parent's identity, and so orphan
// its ticks, every time a child was edited.
function itemText(taskBody: Element): string {
  let text = "";
  for (const child of Array.from(taskBody.childNodes)) {
    if (child.nodeType === Node.ELEMENT_NODE && (child as Element).localName === "task-list") continue;
    text += child.textContent ?? "";
  }
  return text.replace(WHITESPACE, " ").trim();
}

// unique keeps the first of a repeated item. The same words twice are one
// item: they would share the one row in the store, so drawing two boxes that
// moved together would say something untrue about them.
function unique(items: string[]): string[] {
  const seen = new Set<string>();
  return items.filter((i) => (seen.has(i) ? false : (seen.add(i), true)));
}

// agreementItems is every item one document states, in the order it states
// them. A body that is not well-formed, or is empty, states none: the panel
// draws nothing rather than failing, and the store is told to forget nothing.
export function agreementItems(body: string): string[] {
  const root = parseXml(body);
  if (!root) return [];
  const out: string[] = [];
  for (const task of Array.from(root.getElementsByTagNameNS(NAMESPACES.ac, "task"))) {
    const bodyEl = task.getElementsByTagNameNS(NAMESPACES.ac, "task-body")[0];
    if (!bodyEl) continue;
    const text = itemText(bodyEl);
    if (text) out.push(text);
  }
  return unique(out);
}

// effectiveAgreement is what one issue is held to: the board's standing
// items, then whatever its sprint added to them. An issue in no sprint is
// passed no additions and sees the board's items alone.
export function effectiveAgreement(boardBody: string, sprintBody = ""): string[] {
  return unique([...agreementItems(boardBody), ...agreementItems(sprintBody)]);
}

// One line of the panel's list. stated is whether the agreement still says
// this, which is the answer to a tick made against wording somebody has since
// edited: the tick is a record of what was checked, so it stays, marked as
// made against different words, rather than being dropped or moved onto
// whatever the item says now.
export interface AgreementRow {
  text: string;
  ticked: boolean;
  stated: boolean;
}

// agreementRows puts the store's ticks beside today's items: every item the
// agreement states, in its own order, and then the wording of any tick the
// agreement no longer states. The progress count reads against the stated
// rows alone, because an item the agreement dropped is not something this
// issue is still held to.
export function agreementRows(items: string[], ticked: string[]): AgreementRow[] {
  const stated = new Set(items);
  return [
    ...items.map((text) => ({ text, ticked: ticked.includes(text), stated: true })),
    ...ticked.filter((text) => !stated.has(text)).map((text) => ({ text, ticked: true, stated: false })),
  ];
}
