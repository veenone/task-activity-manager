import { useEffect, useState } from "react";
import {
  ListContainers,
  GetContainerBoard,
  SeedSampleContainers,
  CreateContainerAndAllocate,
  EditContainer,
  DeleteContainer,
  ExportPytest,
  errMsg,
} from "../api";
import type { Container, TestPlanBoard, Bucket } from "../api";
import { Menu } from "./Menu";

interface Props {
  profileId: string;
  refreshKey: number;
  onChanged: () => void;
}

const KINDS: Array<{ value: string; label: string }> = [
  { value: "testset", label: "Test Set" },
  { value: "testplan", label: "Test Plan" },
  { value: "testexec", label: "Test Execution" },
];

// ContainersView manages Test Sets / Plans / Executions (FR-13.7 + container
// CRUD): pick a kind and a container, see its member Tests with run status, and
// create / rename / delete. Computed from the local store; recomputes when the
// profile changes or a sync/commit bumps refreshKey.
export function ContainersView({ profileId, refreshKey, onChanged }: Props) {
  const [kind, setKind] = useState("testplan");
  const [containers, setContainers] = useState<Container[]>([]);
  const [selected, setSelected] = useState("");
  const [board, setBoard] = useState<TestPlanBoard | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [seeding, setSeeding] = useState(false);
  const [editingName, setEditingName] = useState(false);
  const [nameDraft, setNameDraft] = useState("");

  const kindLabel = KINDS.find((k) => k.value === kind)?.label ?? "container";
  const selectedContainer = containers.find((c) => c.key === selected) ?? null;

  // commitInlineRename saves the detail card's editable name (an inline path to
  // the same rename CRUD as the Actions menu).
  async function commitInlineRename() {
    setEditingName(false);
    const name = nameDraft.trim();
    if (!selectedContainer || !name || name === selectedContainer.summary) return;
    setError("");
    try {
      await EditContainer(profileId, selected, name);
      onChanged();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function seed() {
    setSeeding(true);
    setError("");
    try {
      await SeedSampleContainers(profileId);
      onChanged();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setSeeding(false);
    }
  }

  async function newContainer() {
    const name = window.prompt(`New ${kindLabel} name:`);
    if (!name || !name.trim()) return;
    setError("");
    try {
      await CreateContainerAndAllocate(profileId, kind, name.trim(), []);
      onChanged();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function renameContainer() {
    if (!selected) return;
    const cur = containers.find((c) => c.key === selected);
    const name = window.prompt(`Rename ${kindLabel} to:`, cur?.summary ?? "");
    if (name === null || !name.trim()) return;
    setError("");
    try {
      await EditContainer(profileId, selected, name.trim());
      onChanged();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function generatePytest() {
    if (!selected) return;
    setError("");
    try {
      const path = await ExportPytest(profileId, selected);
      if (path) window.alert(`pytest scaffold saved to:\n${path}`);
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function deleteContainer() {
    if (!selected) return;
    if (
      !window.confirm(
        `Delete this ${kindLabel}? Its test memberships are removed (committed on sync).`,
      )
    )
      return;
    setError("");
    try {
      await DeleteContainer(profileId, selected);
      setSelected("");
      onChanged();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  useEffect(() => {
    if (!profileId) return;
    let cancelled = false;
    setLoading(true);
    setError("");
    ListContainers(profileId, kind)
      .then((cs) => {
        if (cancelled) return;
        setContainers(cs ?? []);
        setSelected((cur) => {
          if (cur && (cs ?? []).some((c) => c.key === cur)) return cur;
          return cs && cs.length > 0 ? cs[0].key : "";
        });
      })
      .catch((e) => {
        if (!cancelled) setError(errMsg(e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [profileId, kind, refreshKey]);

  useEffect(() => {
    if (!profileId || !selected) {
      setBoard(null);
      return;
    }
    let cancelled = false;
    setError("");
    GetContainerBoard(profileId, selected)
      .then((b) => {
        if (!cancelled) setBoard(b);
      })
      .catch((e) => {
        if (!cancelled) setError(errMsg(e));
      });
    return () => {
      cancelled = true;
    };
  }, [profileId, selected, refreshKey]);

  return (
    <div className="board">
      <div className="board-head">
        <label className="board-picker">
          <span>Type</span>
          <select value={kind} onChange={(e) => setKind(e.target.value)}>
            {KINDS.map((k) => (
              <option key={k.value} value={k.value}>
                {k.label}
              </option>
            ))}
          </select>
        </label>
        <label className="board-picker">
          <span>{kindLabel}</span>
          {loading ? (
            <span className="muted">Loading…</span>
          ) : (
            <select
              value={selected}
              onChange={(e) => setSelected(e.target.value)}
              disabled={containers.length === 0}
            >
              {containers.length === 0 && <option value="">None</option>}
              {containers.map((c) => (
                <option key={c.key} value={c.key}>
                  {c.key} — {c.summary}
                </option>
              ))}
            </select>
          )}
        </label>
        <div className="board-head-actions">
          <button className="btn btn-primary" onClick={newContainer} title={`New ${kindLabel}`}>
            + New
          </button>
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
                onClick: generatePytest,
                disabled: !selected,
                title: "Generate a pytest scaffold from this container's tests",
              },
              {
                key: "delete",
                label: `Delete ${kindLabel}…`,
                onClick: deleteContainer,
                disabled: !selected,
                danger: true,
              },
              { key: "d", divider: true },
              {
                key: "seed",
                label: seeding ? "Generating…" : "Regenerate sample data",
                onClick: seed,
                disabled: seeding,
                title: "Regenerate sample sets / plans / executions from synced tests",
              },
            ]}
          />
        </div>
      </div>

      {error && <div className="error-text">{error}</div>}

      {!loading && containers.length === 0 && (
        <p className="muted">
          No {kindLabel}s yet. Create one, run a sync, or generate sample data.
        </p>
      )}

      {selectedContainer && (
        <div className="container-card">
          <div className="container-card-top">
            <span className={`kind-badge kind-${kind}`}>{kindLabel}</span>
            <span className="mono container-card-key">
              {selectedContainer.key}
            </span>
            {selectedContainer.status && (
              <span className="status-pill">{selectedContainer.status}</span>
            )}
            <span className="container-card-count">
              {(board?.rows.length ?? 0).toLocaleString()} test
              {(board?.rows.length ?? 0) === 1 ? "" : "s"}
            </span>
          </div>

          {editingName ? (
            <input
              className="container-card-name-edit"
              autoFocus
              value={nameDraft}
              onChange={(e) => setNameDraft(e.target.value)}
              onBlur={commitInlineRename}
              onKeyDown={(e) => {
                if (e.key === "Enter") commitInlineRename();
                if (e.key === "Escape") setEditingName(false);
              }}
            />
          ) : (
            <h2
              className="container-card-name"
              title="Click to rename"
              onClick={() => {
                setNameDraft(selectedContainer.summary);
                setEditingName(true);
              }}
            >
              {selectedContainer.summary || "(untitled)"}
            </h2>
          )}

          {board && board.runCounts.length > 0 && (
            <div className="container-card-runs">
              <RunBar counts={board.runCounts} />
              <div className="board-counts">
                {board.runCounts.map((b) => (
                  <RunBadge key={b.label} status={b.label} count={b.count} />
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {board && containers.length > 0 && (
        <table className="board-table">
          <thead>
            <tr>
              <th>Test</th>
              <th>Summary</th>
              <th>Status</th>
              <th>Execution</th>
            </tr>
          </thead>
          <tbody>
            {board.rows.length === 0 ? (
              <tr>
                <td colSpan={4} className="muted">
                  This {kindLabel.toLowerCase()} has no tests.
                </td>
              </tr>
            ) : (
              board.rows.map((r) => (
                <tr key={r.testKey}>
                  <td className="mono">{r.testKey}</td>
                  <td>{r.summary}</td>
                  <td>{r.status || "—"}</td>
                  <td>
                    {r.runStatus ? (
                      <span
                        className={`run-badge run-${r.runStatus.toLowerCase()}`}
                      >
                        {r.runStatus}
                      </span>
                    ) : (
                      <span className="muted">not run</span>
                    )}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      )}
    </div>
  );
}

// RunBar is a compact stacked bar of run-status proportions for the selected
// container — a glanceable view of how its tests are doing.
function RunBar({ counts }: { counts: Bucket[] }) {
  const sum = counts.reduce((a, b) => a + b.count, 0) || 1;
  return (
    <div className="run-bar" title="Run-status distribution">
      {counts.map((b) => (
        <span
          key={b.label}
          className={runSegClass(b.label)}
          style={{ width: `${(b.count / sum) * 100}%` }}
          title={`${b.label}: ${b.count}`}
        />
      ))}
    </div>
  );
}

function runSegClass(label: string): string {
  if (label === "(not run)") return "run-seg";
  return `run-seg run-${label.toLowerCase()}`;
}

function RunBadge({ status, count }: { status: string; count: number }) {
  const cls =
    status === "(not run)" ? "run-badge" : `run-badge run-${status.toLowerCase()}`;
  return (
    <span className={cls}>
      {status === "(not run)" ? "not run" : status} {count}
    </span>
  );
}
