import type { CoverageState } from "./useCoverageState";
import type { CoverageActions } from "./useCoverageActions";

// RequirementList is the left side: add a functional requirement, and the
// list to pick or delete one.
export function RequirementList({ s, a }: { s: CoverageState; a: CoverageActions }) {
  const { selected, setSelected, busy, newName, setNewName, canon } = s;
  const { createCanonical, deleteCanonical } = a;
  return (
    <>
      <aside className="cov-list">
        <div className="cov-list-head" data-tour="coverage-tools">
          <input
            className="cov-input"
            placeholder="New functional requirement…"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") void createCanonical();
            }}
          />
          <button className="btn" disabled={busy || !newName.trim()} onClick={() => void createCanonical()}>
            Add
          </button>
        </div>
        {canon.length === 0 && <p className="cov-empty">No functional requirements yet. Add one, then import its parameter template.</p>}
        <ul className="cov-canon-list">
          {canon.map((c) => (
            <li
              key={c.id}
              className={`cov-canon row-stretch${selected === c.id ? " cov-canon-active" : ""}`}
            >
              <button
                className="row-select cov-canon-name"
                aria-current={selected === c.id ? "true" : undefined}
                onClick={() => setSelected(c.id)}
              >
                {c.name}
              </button>
              <span className="cov-canon-meta">{c.memberCount} proj</span>
              <button
                className="cov-x"
                title="Delete"
                onClick={(e) => {
                  e.stopPropagation();
                  void deleteCanonical(c);
                }}
                aria-label="Delete"
              >
                ×
              </button>
            </li>
          ))}
        </ul>
      </aside>
    </>
  );
}
