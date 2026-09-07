import type { EpicTreeData } from "../api";

// NO_EPIC_KEY stands in for the orphans group in the expanded set and the
// flat visible-item list: it is not a real issue key, so it can never
// collide with one.
export const NO_EPIC_KEY = "__no_epic__";
export const MAX_ORPHAN_ROWS = 200;

export type RowKind = "all" | "epic" | "noepic" | "child";

export interface Row {
  id: string;
  kind: RowKind;
  ownerKey: string;
}

// visibleRows flattens the tree into the order the mockup renders it in:
// All epics, then each epic with its children when expanded, then the
// orphans group with its children (capped) when expanded. It backs both the
// keyboard model (which item is next/previous, and a child's parent) and the
// data-tree-index each rendered row carries.
export function visibleRows(tree: EpicTreeData, expanded: Set<string>): Row[] {
  const rows: Row[] = [{ id: "", kind: "all", ownerKey: "" }];
  for (const node of tree.epics) {
    const key = node.issue.key;
    rows.push({ id: key, kind: "epic", ownerKey: key });
    if (expanded.has(key)) {
      for (const child of node.children) rows.push({ id: child.key, kind: "child", ownerKey: key });
    }
  }
  if (tree.orphans.length > 0) {
    rows.push({ id: NO_EPIC_KEY, kind: "noepic", ownerKey: NO_EPIC_KEY });
    if (expanded.has(NO_EPIC_KEY)) {
      for (const child of tree.orphans.slice(0, MAX_ORPHAN_ROWS)) {
        rows.push({ id: child.key, kind: "child", ownerKey: NO_EPIC_KEY });
      }
    }
  }
  return rows;
}

// ownerOf finds the epic (or the orphans group) a key belongs to, so a
// selection made elsewhere (search, a click deeper in the app) can open the
// branch that holds it.
export function ownerOf(tree: EpicTreeData, key: string): string {
  if (!key) return "";
  for (const node of tree.epics) {
    if (node.issue.key === key) return "";
    if (node.children.some((c) => c.key === key)) return node.issue.key;
  }
  if (tree.orphans.some((o) => o.key === key)) return NO_EPIC_KEY;
  return "";
}
