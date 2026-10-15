import { SyncContainers, errMsg } from "../../api";
import { SearchableSelect } from "../SearchableSelect";
import { Menu } from "../Menu";
import { KINDS } from "./constants";
import type { ContainerState } from "./useContainerState";
import type { ContainerActions, ContainerViewProps } from "./useContainerActions";

// ContainerPickerBar is the row that picks a container: kind, status and
// label filters, the container itself, and the create / sync / actions buttons.
export function ContainerPickerBar({ s, a, p }: { s: ContainerState; a: ContainerActions; p: ContainerViewProps }) {
  const {
    kind, setKind, cStatus, setCStatus, cLabel, setCLabel, selected, setSelected, setError,
    seeding, syncing, setSyncing, setShowJUnitNewExec, setDetailKey, profileId, loading, kindLabel,
    statusOptions, statusCounts, viewContainers, labelOptions,
  } = s;
  const { seed, cleanSample, newContainer, renameContainer, generatePytest, deleteContainer } = a;
  const { onChanged, isDemo } = p;
  return (
      <div className="board-head">
        <label className="board-picker board-picker--secondary" data-tour="plans-type">
          <span>Type</span>
          <select
            className="app-select container-type-select"
            value={kind}
            onChange={(e) => {
              // Switching type changes the member set, so close any open test
              // detail (it belongs to the previous type's selection).
              setKind(e.target.value);
              setDetailKey(null);
            }}
          >
            {KINDS.map((k) => (
              <option key={k.value} value={k.value}>
                {k.label}
              </option>
            ))}
          </select>
        </label>
        {/* Status filter: a compact dropdown between Type and the item picker
            (replaces the old pill row to save horizontal space). It reads as the
            tertiary control, recessed until a status is applied, when it adopts
            the accent so an active filter is obvious at a glance. */}
        <label className="board-picker board-picker--tertiary">
          <span>Status</span>
          <select
            className={`app-select container-filter-select${
              cStatus ? " is-filtering" : ""
            }`}
            value={cStatus}
            onChange={(e) => setCStatus(e.target.value)}
            title="Filter containers by status"
          >
            <option value="">All statuses ({statusCounts.get("") ?? 0})</option>
            {statusOptions.map((s) => (
              <option key={s} value={s}>
                {s} ({statusCounts.get(s) ?? 0})
              </option>
            ))}
          </select>
        </label>
        {labelOptions.length > 0 && (
          <label className="board-picker board-picker--tertiary">
            <span>Label</span>
            <select
              className={`app-select container-filter-select${
                cLabel ? " is-filtering" : ""
              }`}
              value={cLabel}
              onChange={(e) => setCLabel(e.target.value)}
              title="Filter containers by label"
            >
              <option value="">All labels</option>
              {labelOptions.map((l) => (
                <option key={l} value={l}>
                  {l}
                </option>
              ))}
            </select>
          </label>
        )}
        <label className="board-picker board-picker--primary" data-tour="plans-pick">
          <span>{kindLabel}</span>
          {loading ? (
            <span className="muted">Loading…</span>
          ) : (
            <SearchableSelect
              className="container-exec-filter"
              value={selected}
              onChange={setSelected}
              disabled={viewContainers.length === 0}
              title={`Pick a ${kindLabel} (type to filter)`}
              placeholder={
                viewContainers.length === 0 ? "None" : `Select a ${kindLabel}…`
              }
              options={viewContainers.map((c) => ({
                value: c.key,
                label: `${c.key} · ${c.summary}`,
                className: c.parentKey ? "is-subtask" : undefined,
              }))}
            />
          )}
        </label>

        <div className="board-head-actions">
          <button
            className="btn"
            onClick={async () => {
              setSyncing(true);
              setError("");
              try {
                await SyncContainers(profileId);
                onChanged();
              } catch (e) {
                setError(errMsg(e));
              } finally {
                setSyncing(false);
              }
            }}
            disabled={syncing}
            title="Refresh just the Test Sets / Plans / Executions from Jira (partial sync)"
          >
            {syncing ? "Syncing…" : "Sync"}
          </button>
          <button className="btn btn-primary" onClick={newContainer} data-tour="plans-new">
            New {kindLabel}
          </button>
          {kind === "testexec" && (
            <button
              className="btn"
              onClick={() => setShowJUnitNewExec(true)}
              title="Create a new Test Execution from a JUnit XML report"
            >
              Import JUnit
            </button>
          )}
          <Menu
            label="Actions"
            align="right"
            triggerClassName="btn"
            title={`${kindLabel} actions`}
            items={[
              {
                key: "rename",
                label: `Rename ${kindLabel}…`,
                onClick: renameContainer,
                disabled: !selected,
              },
              {
                key: "pytest",
                label: "Generate pytest…",
                onClick: () => generatePytest("function"),
                disabled: !selected,
                title:
                  "Plain pytest: one @pytest.mark.xray function per test",
              },
              {
                key: "pytest-unittest",
                label: "Generate pytest (unittest class)…",
                onClick: () => generatePytest("unittest"),
                disabled: !selected,
                title:
                  "unittest.TestCase subclass: one test method per test " +
                  "(runs under python -m unittest and pytest)",
              },
              {
                key: "delete",
                label: `Delete ${kindLabel}…`,
                onClick: deleteContainer,
                disabled: !selected,
                danger: true,
              },
              { key: "d", divider: true },
              // Regenerating sample data is a demo-only aid — hidden for real
              // projects so fake containers can't be seeded into them (FR-5).
              ...(isDemo
                ? [
                    {
                      key: "seed",
                      label: seeding
                        ? "Generating…"
                        : "Regenerate sample data",
                      onClick: seed,
                      disabled: seeding,
                      title:
                        "Regenerate sample sets / plans / executions from synced tests",
                    },
                  ]
                : []),
              {
                key: "clean",
                label: "Clean sample data…",
                onClick: cleanSample,
                danger: true,
                title:
                  "Remove sample containers created by 'Regenerate sample data' (real data untouched)",
              },
            ]}
          />
        </div>
      </div>
  );
}
