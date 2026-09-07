import { useEffect, useRef, useState } from "react";
import type { KeyboardEvent } from "react";
import type { EpicTreeData, Issue } from "../api";
import { TypeChip } from "./TypeChip";
import { statusClass } from "../lib/statusClass";
import { MAX_ORPHAN_ROWS, NO_EPIC_KEY, ownerOf, visibleRows } from "../lib/epicTreeItems";
import type { Row } from "../lib/epicTreeItems";

const MOVED_FLASH_MS = 2000;

function progressText(done: number, total: number, points: number): string {
  return `${done} of ${total} done` + (points > 0 ? `, ${points} pts` : "");
}

interface Props {
  tree: EpicTreeData;
  selectedKey: string;
  onSelect: (key: string) => void;
  expanded: Set<string>;
  onExpandedChange: (updater: (prev: Set<string>) => Set<string>) => void;
  movedKey: string;
}

export function EpicTree({ tree, selectedKey, onSelect, expanded, onExpandedChange, movedKey }: Props) {
  const rootRef = useRef<HTMLElement>(null);
  const seenRef = useRef<Set<string>>(new Set());
  const [flashKey, setFlashKey] = useState("");

  // A newly seen epic (or the orphans group) opens by default, and so does
  // the branch holding whichever key is selected, so a freshly loaded tree
  // (or a selection made elsewhere) never hides what it should show.
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
  const indexOf = new Map(rows.map((r, i) => [r.id, i] as const));

  function focusIndex(index: number) {
    rootRef.current?.querySelector<HTMLElement>(`[data-tree-index="${index}"]`)?.focus();
  }

  function toggle(key: string) {
    onExpandedChange((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  function onKeyDown(e: KeyboardEvent, row: Row) {
    const index = indexOf.get(row.id) ?? 0;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      focusIndex(Math.min(rows.length - 1, index + 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      focusIndex(Math.max(0, index - 1));
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
        focusIndex(indexOf.get(row.ownerKey) ?? index);
      }
    } else if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      if (row.kind === "all") onSelect("");
      else if (row.kind === "epic" || row.kind === "child") onSelect(row.id);
      else toggle(row.id);
    }
  }

  function childRow(child: Issue, ownerKey: string) {
    const row: Row = { id: child.key, kind: "child", ownerKey };
    const selected = child.key === selectedKey;
    return (
      <div
        key={child.key}
        role="treeitem"
        aria-selected={selected}
        tabIndex={selected ? 0 : -1}
        data-tree-index={indexOf.get(child.key)}
        data-tree-key={child.key}
        className={`folder-item epic-row${selected ? " folder-selected" : ""}${flashKey === child.key ? " epic-row-moved" : ""}`}
        onClick={() => onSelect(child.key)}
        onKeyDown={(e) => onKeyDown(e, row)}
      >
        <span className="folder-caret" />
        <TypeChip type={child.type} />
        <span>{child.key}</span>
        <span>{child.summary}</span>
        <span className={`chip chip-status chip-status-${statusClass(child.status)}`}>{child.status}</span>
        <span>{child.storyPoints ?? "-"}</span>
        {child.pending && <span className="pending-dot" role="img" aria-label="Pending changes" />}
      </div>
    );
  }

  return (
    <nav className="folder-tree" aria-label="Epics" role="tree" ref={rootRef}>
      <div
        role="treeitem"
        aria-selected={selectedKey === ""}
        tabIndex={selectedKey === "" ? 0 : -1}
        data-tree-index={0}
        className={`folder-item epic-row${selectedKey === "" ? " folder-selected" : ""}`}
        onClick={() => onSelect("")}
        onKeyDown={(e) => onKeyDown(e, { id: "", kind: "all", ownerKey: "" })}
      >
        <span className="folder-caret" />
        <span />
        <span className="folder-name">All epics</span>
        <span className="folder-count">{tree.epics.length}</span>
      </div>

      {tree.epics.map((node) => {
        const key = node.issue.key;
        const open = expanded.has(key);
        const selected = key === selectedKey;
        const row: Row = { id: key, kind: "epic", ownerKey: key };
        return (
          <div className="folder-node" key={key}>
            <div
              role="treeitem"
              aria-expanded={open}
              aria-selected={selected}
              tabIndex={selected ? 0 : -1}
              aria-label={`${key} ${node.issue.summary}`}
              data-tree-index={indexOf.get(key)}
              data-tree-key={key}
              className={`folder-item epic-row${selected ? " folder-selected" : ""}${flashKey === key ? " epic-row-moved" : ""}`}
              onClick={() => onSelect(key)}
              onKeyDown={(e) => onKeyDown(e, row)}
            >
              <span className="folder-caret folder-caret-toggle" onClick={(e) => { e.stopPropagation(); toggle(key); }}>
                {open ? "▾" : "▸"}
              </span>
              <TypeChip type={node.issue.type} />
              <span className="folder-name">
                <span className="accent-text">{key}</span> {node.issue.summary}
              </span>
              <span className="folder-count">{progressText(node.done, node.total, node.points)}</span>
              {node.issue.pending && <span className="pending-dot" role="img" aria-label="Pending changes" />}
            </div>
            {open && (
              <div className="folder-children" role="group">
                {node.children.map((child) => childRow(child, key))}
              </div>
            )}
          </div>
        );
      })}

      {tree.orphans.length > 0 && (
        <div className="folder-node">
          <div
            role="treeitem"
            aria-expanded={expanded.has(NO_EPIC_KEY)}
            tabIndex={NO_EPIC_KEY === selectedKey ? 0 : -1}
            data-tree-index={indexOf.get(NO_EPIC_KEY)}
            className="folder-item epic-row"
            onClick={() => toggle(NO_EPIC_KEY)}
            onKeyDown={(e) => onKeyDown(e, { id: NO_EPIC_KEY, kind: "noepic", ownerKey: NO_EPIC_KEY })}
          >
            <span className="folder-caret folder-caret-toggle" onClick={(e) => { e.stopPropagation(); toggle(NO_EPIC_KEY); }}>
              {expanded.has(NO_EPIC_KEY) ? "▾" : "▸"}
            </span>
            <span />
            <span className="folder-name">No epic</span>
            <span className="folder-count">{tree.orphans.length}</span>
          </div>
          {expanded.has(NO_EPIC_KEY) && (
            <div className="folder-children" role="group">
              {tree.orphans.slice(0, MAX_ORPHAN_ROWS).map((child) => childRow(child, NO_EPIC_KEY))}
              {tree.orphans.length > MAX_ORPHAN_ROWS && (
                <p className="muted small">{`and ${tree.orphans.length - MAX_ORPHAN_ROWS} more. Use the search to narrow.`}</p>
              )}
            </div>
          )}
        </div>
      )}
    </nav>
  );
}
