import { useMemo, useState } from "react";
import type { Folder } from "../api";

interface Props {
  folders: Folder[];
  selected: string; // "" means "All tests"
  onSelect: (folderId: string) => void;
}

export function FolderTree({ folders, selected, onSelect }: Props) {
  // Index folders by parentId so each node can find its children in O(1).
  const childrenOf = useMemo(() => {
    const map = new Map<string, Folder[]>();
    for (const f of folders) {
      const arr = map.get(f.parentId);
      if (arr) arr.push(f);
      else map.set(f.parentId, [f]);
    }
    return map;
  }, [folders]);

  const roots = childrenOf.get("") ?? [];

  return (
    <nav className="folder-tree">
      <div
        className={
          "folder-item" + (selected === "" ? " folder-selected" : "")
        }
        onClick={() => onSelect("")}
      >
        <span className="folder-caret" />
        <span className="folder-name">All tests</span>
      </div>
      {roots.map((root) => (
        <FolderNode
          key={root.id}
          folder={root}
          childrenOf={childrenOf}
          selected={selected}
          onSelect={onSelect}
          depth={0}
        />
      ))}
    </nav>
  );
}

interface NodeProps {
  folder: Folder;
  childrenOf: Map<string, Folder[]>;
  selected: string;
  onSelect: (id: string) => void;
  depth: number;
}

function FolderNode({
  folder,
  childrenOf,
  selected,
  onSelect,
  depth,
}: NodeProps) {
  const [open, setOpen] = useState(true);
  const children = childrenOf.get(folder.id) ?? [];
  const hasChildren = children.length > 0;

  return (
    <div className="folder-node">
      <div
        className={
          "folder-item" + (selected === folder.id ? " folder-selected" : "")
        }
        style={{ paddingLeft: 10 + depth * 14 }}
        onClick={() => onSelect(folder.id)}
      >
        {hasChildren ? (
          <span
            className="folder-caret folder-caret-toggle"
            onClick={(e) => {
              e.stopPropagation();
              setOpen((o) => !o);
            }}
          >
            {open ? "▾" : "▸"}
          </span>
        ) : (
          <span className="folder-caret" />
        )}
        <span className="folder-name">{folder.name}</span>
      </div>
      {hasChildren &&
        open &&
        children.map((c) => (
          <FolderNode
            key={c.id}
            folder={c}
            childrenOf={childrenOf}
            selected={selected}
            onSelect={onSelect}
            depth={depth + 1}
          />
        ))}
    </div>
  );
}
