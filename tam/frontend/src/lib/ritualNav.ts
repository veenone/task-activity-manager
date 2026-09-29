import type { RitualDocument } from "../api";
import { AGREEMENT_ADDITIONS_LABEL, DONE_AGREEMENT, RITUAL_LABEL, RITUAL_ORDER } from "./ritualText";

// The Rituals view's document list, kept out of the component so it can be
// read and tested as data.
//
// A document is identified here by its sprint as well as its type, which the
// five ritual pages never needed: a board's done agreement and a sprint's
// additions to it are the same type with different sprint ids, and the nav
// shows both at once. Everything else in the view already takes the board,
// sprint and type from the document it was handed, so this is the only place
// that had to learn the difference.

export interface RitualNavItem {
  key: string;
  label: string;
  doc: RitualDocument;
}

type Identified = Pick<RitualDocument, "sprintId" | "ritualType">;

export function navKey(doc: Identified): string {
  return `${doc.sprintId}:${doc.ritualType}`;
}

// isBoardAgreement is the board's standing agreement rather than a sprint's
// additions: the one document in the list that belongs to no sprint.
export function isBoardAgreement(doc: Identified): boolean {
  return doc.ritualType === DONE_AGREEMENT && doc.sprintId === 0;
}

// navItems is the five ritual pages in their own order, then the board's
// agreement, then the sprint's additions. The agreements come last because
// they are the documents a team writes once and reads, not the ones it opens
// every sprint.
export function navItems(docs: RitualDocument[]): RitualNavItem[] {
  const items: RitualNavItem[] = [];
  for (const type of RITUAL_ORDER) {
    const doc = docs.find((d) => d.ritualType === type);
    if (doc) items.push({ key: navKey(doc), label: RITUAL_LABEL[type], doc });
  }
  for (const doc of docs.filter(isBoardAgreement)) {
    items.push({ key: navKey(doc), label: RITUAL_LABEL[DONE_AGREEMENT], doc });
  }
  for (const doc of docs.filter((d) => d.ritualType === DONE_AGREEMENT && !isBoardAgreement(d))) {
    items.push({ key: navKey(doc), label: AGREEMENT_ADDITIONS_LABEL, doc });
  }
  return items;
}

// selectedKey is which item the view draws. The selection is not reset when
// the sprint changes, and the sprint switched to may not hold the same
// documents (a closed sprint gets no backfill, and Remove deletes one), so an
// unknown key falls back to Planning and then to whatever is first rather
// than leaving the nav and the page pane both blank.
export function selectedKey(items: RitualNavItem[], selected: string): string {
  if (items.some((i) => i.key === selected)) return selected;
  return items.find((i) => i.doc.ritualType === "planning")?.key ?? items[0]?.key ?? selected;
}
