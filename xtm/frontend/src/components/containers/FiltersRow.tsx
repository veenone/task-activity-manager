import { ContainerFilterBar } from "./ContainerFilterBar";
import { MemberFilters } from "./MemberFilters";
import type { ContainerState } from "./useContainerState";
import type { ContainerActions, ContainerViewProps } from "./useContainerActions";

type Chip = { text: string; clear: () => void };

// FiltersRow is the one row of filters above the member table (#456). Closed,
// it is a Filters button with the number in force and a chip for each, so a
// filter never hides out of sight; open, it holds the container list's
// filters and the member filters: fix version, result, label and JQL.
export function FiltersRow({ s, a, p }: { s: ContainerState; a: ContainerActions; p: ContainerViewProps }) {
  const {
    filtersOpen, setFiltersOpen, kind, cExecType, setCExecType, cEnv, setCEnv, memberFvFilter,
    setMemberFvFilter, memberRunFilter, setMemberRunFilter, memberLabel, setMemberLabel, memberJqlKeys,
    setMemberJqlKeys, memberLabelOptions, board, selected, selectedContainer, setBoardPage, profileId, caps,
  } = s;
  const { isDemo } = p;
  // Each member filter starts the table again from its first page.
  const member = (set: (v: string) => void) => (v: string) => {
    set(v);
    setBoardPage(0);
  };
  const chips = (
    [
      cExecType && { text: `Execution: ${cExecType === "subtask" ? "Sub-task" : "Standalone"}`, clear: () => setCExecType("") },
      cEnv && { text: `Environment: ${cEnv}`, clear: () => setCEnv("") },
      memberFvFilter && { text: `Fix version: ${memberFvFilter}`, clear: () => member(setMemberFvFilter)("") },
      memberRunFilter && { text: `Result: ${memberRunFilter}`, clear: () => member(setMemberRunFilter)("") },
      memberLabel && { text: `Label: ${memberLabel}`, clear: () => member(setMemberLabel)("") },
      memberJqlKeys && { text: "JQL", clear: () => setMemberJqlKeys(null) },
    ] as (Chip | "" | null)[]
  ).filter((c): c is Chip => !!c);
  const fixVersions = kind === "testexec" ? (selectedContainer?.fixVersions ?? []) : [];

  return (
    <div className="filters-row">
      <div className="filters-row-head">
        <button
          type="button"
          className="btn"
          aria-expanded={filtersOpen}
          onClick={() => setFiltersOpen(!filtersOpen)}
          title={filtersOpen ? "Hide the filters" : "Show the filters"}
        >
          {chips.length > 0 ? `Filters (${chips.length})` : "Filters"}
          <span className="menu-caret" aria-hidden="true">
            ▾
          </span>
        </button>
        {chips.map((c) => (
          <span key={c.text} className="env-chip filter-chip">
            {c.text}
            <button type="button" className="env-chip-remove" aria-label={`Remove filter ${c.text}`} title="Remove this filter" onClick={c.clear}>
              ✕
            </button>
          </span>
        ))}
      </div>
      {filtersOpen && (
        <div className="filters-row-panel">
          <ContainerFilterBar s={s} a={a} p={p} />
          {board && selected && (
            <div className="member-filters">
              {fixVersions.length > 0 && (
                <label className="member-filter">
                  Fix version
                  <select
                    className="container-status-filter app-select"
                    value={memberFvFilter}
                    onChange={(e) => member(setMemberFvFilter)(e.target.value)}
                  >
                    <option value="">All fix versions</option>
                    {fixVersions.map((fv) => (
                      <option key={fv} value={fv}>
                        {fv}
                      </option>
                    ))}
                  </select>
                </label>
              )}
              {board.runCounts.length > 0 && (
                <label className="member-filter">
                  Result
                  <select
                    className="container-status-filter app-select"
                    value={memberRunFilter}
                    onChange={(e) => member(setMemberRunFilter)(e.target.value)}
                  >
                    <option value="">All results</option>
                    {board.runCounts.map((b) => (
                      <option key={b.label} value={b.label}>
                        {b.label}
                      </option>
                    ))}
                  </select>
                </label>
              )}
              <MemberFilters
                profileId={profileId}
                containerKey={selected}
                labels={memberLabelOptions}
                label={memberLabel}
                onLabel={member(setMemberLabel)}
                jqlOn={memberJqlKeys !== null}
                jqlAvailable={caps.supportsJqlScope && !isDemo}
                onJqlKeys={(k) => {
                  setMemberJqlKeys(k);
                  setBoardPage(0);
                }}
              />
            </div>
          )}
        </div>
      )}
    </div>
  );
}
