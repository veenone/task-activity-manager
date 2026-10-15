import type { Bucket } from "../api";

interface Props {
  components: Bucket[];
  selected: string; // "" means "All tests"
  emptyLabel: string;
  onSelect: (name: string) => void;
}

// ComponentList is the browse sidebar when grouping by Component. It mirrors the
// folder tree / container list — an "All tests" entry plus one row per Jira
// component with its test count — and selecting one filters the grid to tests
// carrying that component.
export function ComponentList({
  components,
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
      {components.length === 0 ? (
        <div className="folder-item muted">{emptyLabel}</div>
      ) : (
        components.map((c) => (
          <button
            type="button"
            key={c.label}
            className={
              "folder-item folder-filter-button" +
              (selected === c.label ? " folder-selected" : "")
            }
            aria-pressed={selected === c.label}
            style={{ paddingLeft: 24 }}
            onClick={() => onSelect(c.label)}
            title={`${c.label}: ${c.count} test(s)`}
          >
            <span className="folder-name">{c.label}</span>
            <span className="folder-count">{c.count}</span>
          </button>
        ))
      )}
    </nav>
  );
}
