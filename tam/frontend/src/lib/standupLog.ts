import type { Node as PMNode } from "@tiptap/pm/model";

export const DAILY_LOG_HEADING = "Daily log";

export type LogPlace = { kind: "insert"; pos: number } | { kind: "exists" } | { kind: "missing" };

// findTodaysEntry answers where "Add today's entry" puts a day: straight
// under the Daily log heading, so the newest day reads first. It refuses a
// second entry for the same day, and a page whose heading is gone, since a
// guessed place in a page somebody reorganised lands wherever the guess did.
export function findTodaysEntry(doc: PMNode, dayHeading: string): LogPlace {
  let insertAt = -1;
  let inLog = false;
  let exists = false;
  doc.forEach((child, offset) => {
    if (child.type.name !== "heading") return;
    const level = Number(child.attrs.level);
    const text = child.textContent.trim();
    if (level <= 2) {
      inLog = level === 2 && text === DAILY_LOG_HEADING;
      if (inLog && insertAt < 0) insertAt = offset + child.nodeSize;
      return;
    }
    if (inLog && level === 3 && text === dayHeading) exists = true;
  });
  if (insertAt < 0) return { kind: "missing" };
  return exists ? { kind: "exists" } : { kind: "insert", pos: insertAt };
}

export function localDay(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}
