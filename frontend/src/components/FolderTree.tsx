import { useMemo, useState } from "react";
import type { Folder } from "../api";

interface Props {
  folders: Folder[];
  selected: string; // "" means "All tests"
  onSelect: (folderId: string) => void;
  onCreate: (parentPath: string) => void;
  onRename: (path: string, currentName: string) => void;
  onDelete: (path: string) => void;
}

export function FolderTree({
  folders,
  selected,
  onSelect,
  onCreate,
  onRename,
  onDelete,
}: Props) {
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
        <button
          className="folder-action"
          title="New top-level folder"
          onClick={(e) => {
            e.stopPropagation();
            onCreate("");
          }}
        >
          ＋
        </button>
      </div>
      {roots.map((root) => (
        <FolderNode
          key={root.id}
          folder={root}
          childrenOf={childrenOf}
          selected={selected}
          onSelect={onSelect}
          onCreate={onCreate}
          onRename={onRename}
          onDelete={onDelete}
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
  onCreate: (parentPath: string) => void;
  onRename: (path: string, currentName: string) => void;
  onDelete: (path: string) => void;
  depth: number;
}

function FolderNode({
  folder,
  childrenOf,
  selected,
  onSelect,
  onCreate,
  onRename,
  onDelete,
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
        <span className="folder-actions">
          <button
            className="folder-action"
            title="New subfolder"
            onClick={(e) => {
              e.stopPropagation();
              onCreate(folder.id);
            }}
          >
            ＋
          </button>
          <button
            className="folder-action"
            title="Rename folder"
            onClick={(e) => {
              e.stopPropagation();
              onRename(folder.id, folder.name);
            }}
          >
            ✎
          </button>
          <button
            className="folder-action"
            title="Delete folder"
            onClick={(e) => {
              e.stopPropagation();
              onDelete(folder.id);
            }}
          >
            ✕
          </button>
        </span>
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
            onCreate={onCreate}
            onRename={onRename}
            onDelete={onDelete}
            depth={depth + 1}
          />
        ))}
    </div>
  );
}
