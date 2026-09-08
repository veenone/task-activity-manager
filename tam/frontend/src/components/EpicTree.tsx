import { useEffect, useRef, useState } from "react";
import type { CSSProperties, KeyboardEvent } from "react";
import type { EpicNode, EpicTreeData, Issue } from "../api";
import { MAX_ORPHAN_ROWS, NO_EPIC_KEY, ownerOf, visibleRows } from "../lib/epicTreeItems";
import type { Row } from "../lib/epicTreeItems";
import { EpicChildRow, EpicRow } from "./EpicRow";
import { keyColumnWidth } from "../lib/keyColumn";
import { MOVED_FLASH_MS } from "../lib/flash";

interface Props {
  tree: EpicTreeData;
  subtaskLabel?: string;
  selectedKey: string;
  onSelect: (key: string) => void;
  expanded: Set<string>;
  onExpandedChange: (updater: (prev: Set<string>) => Set<string>) => void;
  movedKey: string;
}

export function EpicTree({ tree, subtaskLabel, selectedKey, onSelect, expanded, onExpandedChange, movedKey }: Props) {
  const rootRef = useRef<HTMLElement>(null);
  const seenRef = useRef<Set<string>>(new Set());
  const [flashKey, setFlashKey] = useState("");
  const [focusKey, setFocusKey] = useState(selectedKey);

  // A newly seen epic (or the orphans group) opens by default, and so does
  // the branch holding the selected key, so nothing loads, or gets
  // selected, hidden behind a collapsed row.
  useEffect(() => {
    const toOpen: string[] = [];
    for (const node of tree.epics) {
      if (!seenRef.current.has(node.issue.key)) {
        seenRef.current.add(node.issue.key);
        toOpen.push(node.issue.key);
      }
    }
    if (tree.orphans.length > 0 && !seenRef.current.has(NO_EPIC_KEY)) {
      seenRef.current.add(NO_EPIC_KEY);
      toOpen.push(NO_EPIC_KEY);
    }
    const owner = ownerOf(tree, selectedKey);
    if (owner && !seenRef.current.has(owner)) {
      seenRef.current.add(owner);
      toOpen.push(owner);
    }
    if (toOpen.length > 0) {
      onExpandedChange((prev) => {
        const next = new Set(prev);
        for (const k of toOpen) next.add(k);
        return next;
      });
    }
  }, [tree, selectedKey, onExpandedChange]);

  // A moved row scrolls into view and flashes for two seconds.
  useEffect(() => {
    if (!movedKey) return;
    setFlashKey(movedKey);
    rootRef.current?.querySelector<HTMLElement>(`[data-tree-key="${movedKey}"]`)?.scrollIntoView({ block: "nearest" });
    const t = setTimeout(() => setFlashKey(""), MOVED_FLASH_MS);
    return () => clearTimeout(t);
  }, [movedKey]);

  const rows = visibleRows(tree, expanded);
  // One width for every row: each .epic-row is its own grid container, so a
  // per-row max-content track would size each row to its own key and the
  // columns would stop lining up. "All epics" is in the list because it sits
  // in the same column.
  const keyWidth = keyColumnWidth([
    "All epics",
    ...tree.epics.flatMap((n) => [n.issue.key, ...n.children.map((c) => c.key)]),
    ...tree.orphans.map((o) => o.key),
  ]);
  const indexOf = new Map(rows.map((r, i) => [r.id, i] as const));

  // Exactly one row keeps tabIndex 0: it defaults to the selection, and
  // falls back to the first visible row whenever a collapse, a filter, or a
  // selection change hides whichever row currently owns it.
  useEffect(() => {
    setFocusKey((prev) => {
      if (indexOf.has(prev)) return prev;
      if (indexOf.has(selectedKey)) return selectedKey;
      return rows[0]?.id ?? "";
    });
  }, [tree, expanded, selectedKey]);

  function moveFocus(index: number) {
    const target = rows[index];
    if (!target) return;
    setFocusKey(target.id);
    rootRef.current?.querySelector<HTMLElement>(`[data-tree-index="${index}"]`)?.focus();
  }

  function selectAndFocus(key: string) {
    onSelect(key);
    setFocusKey(key);
  }

  function toggle(key: string, focus?: boolean) {
    onExpandedChange((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
    if (focus) setFocusKey(key);
  }

  function onKeyDown(e: KeyboardEvent, row: Row) {
    const index = indexOf.get(row.id) ?? 0;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      moveFocus(Math.min(rows.length - 1, index + 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      moveFocus(Math.max(0, index - 1));
    } else if (e.key === "ArrowRight") {
      e.preventDefault();
      if (row.kind === "epic" || row.kind === "noepic") onExpandedChange((prev) => new Set(prev).add(row.id));
    } else if (e.key === "ArrowLeft") {
      e.preventDefault();
      if (row.kind === "epic" || row.kind === "noepic") {
        onExpandedChange((prev) => {
          const next = new Set(prev);
          next.delete(row.id);
          return next;
        });
      } else if (row.kind === "child") {
        moveFocus(indexOf.get(row.ownerKey) ?? index);
      }
    } else if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      if (row.kind === "all") selectAndFocus("");
      else if (row.kind === "epic" || row.kind === "child") selectAndFocus(row.id);
      else toggle(row.id, true);
    }
  }

  function renderChildren(children: Issue[], ownerKey: string) {
    return children.map((child) => (
      <EpicChildRow
        key={child.key}
        child={child}
        subtaskLabel={subtaskLabel}
        ownerKey={ownerKey}
        index={indexOf.get(child.key)}
        selected={child.key === selectedKey}
        focused={child.key === focusKey}
        flashed={flashKey === child.key}
        onActivate={selectAndFocus}
        onKeyDown={onKeyDown}
      />
    ));
  }

  // A branch is one epic (or the orphans group) plus its children when
  // open: both render the same header-and-group shape, so they share it.
  function branch(kind: "epic" | "noepic", rowKey: string, open: boolean, children: Issue[], node?: EpicNode, count?: number) {
    return (
      <div className="folder-node" key={rowKey}>
        <EpicRow
          kind={kind}
          subtaskLabel={subtaskLabel}
          rowKey={rowKey}
          node={node}
          count={count}
          index={indexOf.get(rowKey)}
          open={open}
          selected={rowKey === selectedKey}
          focused={rowKey === focusKey}
          flashed={flashKey === rowKey}
          onActivate={() => (kind === "epic" ? selectAndFocus(rowKey) : toggle(rowKey, true))}
          onToggle={() => toggle(rowKey)}
          onKeyDown={(e) => onKeyDown(e, { id: rowKey, kind, ownerKey: rowKey })}
        />
        {open && (
          <div className="folder-children" role="group">
            {renderChildren(children, rowKey)}
            {kind === "noepic" && tree.orphans.length > MAX_ORPHAN_ROWS && (
              <p className="muted small">{`and ${tree.orphans.length - MAX_ORPHAN_ROWS} more. Use the search to narrow.`}</p>
            )}
          </div>
        )}
      </div>
    );
  }

  return (
    <nav
      className="folder-tree"
      aria-label="Epics"
      role="tree"
      ref={rootRef}
      style={{ "--epic-key-w": keyWidth } as CSSProperties}
    >
      <div
        role="treeitem"
        aria-selected={selectedKey === ""}
        tabIndex={focusKey === "" ? 0 : -1}
        data-tree-index={0}
        className={`folder-item epic-row${selectedKey === "" ? " folder-selected" : ""}`}
        onClick={() => selectAndFocus("")}
        onKeyDown={(e) => onKeyDown(e, { id: "", kind: "all", ownerKey: "" })}
      >
        <span className="folder-caret" />
        <span />
        <span className="epic-cell epic-cell-key">All epics</span>
        <span className="epic-cell epic-cell-summary" />
        <span className="epic-cell folder-count epic-cell-progress">{tree.epics.length}</span>
      </div>

      {tree.epics.map((node) => branch("epic", node.issue.key, expanded.has(node.issue.key), node.children, node))}

      {tree.orphans.length > 0 &&
        branch("noepic", NO_EPIC_KEY, expanded.has(NO_EPIC_KEY), tree.orphans.slice(0, MAX_ORPHAN_ROWS), undefined, tree.orphans.length)}
    </nav>
  );
}
