import type { EpicTreeData } from "../api";
import { drawnParents, familyPlace, visibleFamilyIssues } from "./issueFamilies";

export type RowKind = "all" | "epic" | "child";

export interface Row {
  id: string;
  kind: RowKind;
  ownerKey: string;
}

// visibleRows flattens the tree into the order it renders in: All epics,
// then each epic with its children when expanded. It backs both the keyboard
// model (which item is next or previous, and a child's parent) and the
// data-tree-index each rendered row carries.
//
// Issues under no epic are not in the tree. They are a flat list of their
// own, paged by the no-epic tab, which is what let the 200-row ceiling this
// function used to impose on them go (#341).
export function visibleRows(tree: EpicTreeData, expanded: Set<string>, collapsed: ReadonlySet<string> = new Set()): Row[] {
  const rows: Row[] = [{ id: "", kind: "all", ownerKey: "" }];
  for (const node of tree.epics) {
    const key = node.issue.key;
    rows.push({ id: key, kind: "epic", ownerKey: key });
    if (expanded.has(key)) {
      const parents = drawnParents(node.children);
      for (const child of visibleFamilyIssues(node.children, collapsed)) rows.push({ id: child.key, kind: "child", ownerKey: familyPlace(child, parents) === "child" ? child.parentKey : key });
    }
  }
  return rows;
}

// ownerOf finds the epic a key belongs to, so a selection made elsewhere
// (search, a click deeper in the app) can open the branch that holds it.
export function ownerOf(tree: EpicTreeData, key: string): string {
  if (!key) return "";
  for (const node of tree.epics) {
    if (node.issue.key === key) return "";
    if (node.children.some((c) => c.key === key)) return node.issue.key;
  }
  return "";
}
