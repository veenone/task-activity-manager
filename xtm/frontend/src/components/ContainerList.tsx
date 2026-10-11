import type { Container } from "../api";

interface Props {
  containers: Container[];
  selected: string; // "" means "All tests"
  emptyLabel: string;
  onSelect: (key: string) => void;
}

// ContainerList is the browse sidebar when grouping by Test Set or Test Plan
// (FR-11.6). It mirrors the folder tree's look — an "All tests" entry plus one
// row per container — and selecting one filters the grid to that container's
// members.
export function ContainerList({
  containers,
  selected,
  emptyLabel,
  onSelect,
}: Props) {
  return (
    <nav className="folder-tree">
      <button
        type="button"
        className={
          "folder-item folder-filter-button" +
          (selected === "" ? " folder-selected" : "")
        }
        aria-pressed={selected === ""}
        onClick={() => onSelect("")}
      >
        <span className="folder-caret" />
        <span className="folder-name">All tests</span>
      </button>
      {containers.length === 0 ? (
        <div className="folder-item muted">{emptyLabel}</div>
      ) : (
        containers.map((c) => (
          <button
            type="button"
            key={c.key}
            className={
              "folder-item folder-filter-button" +
              (selected === c.key ? " folder-selected" : "")
            }
            aria-pressed={selected === c.key}
            style={{ paddingLeft: 24 }}
            onClick={() => onSelect(c.key)}
            title={`${c.key} — ${c.summary}`}
          >
            <span className="folder-name">
              <span className="mono">{c.key}</span> {c.summary}
            </span>
          </button>
        ))
      )}
    </nav>
  );
}
