import { useEffect, useState } from "react";
import {
  ListContainers,
  GetTestPlanBoard,
  SeedSampleContainers,
  CreateContainerAndAllocate,
  EditContainer,
  DeleteContainer,
  errMsg,
} from "../api";
import type { Container, TestPlanBoard } from "../api";

interface Props {
  profileId: string;
  refreshKey: number;
  onSeeded: () => void;
}

// TestPlanBoardView is the read-only Test Plan board (FR-13.7): pick a plan and
// see each member Test with its consolidated execution status, computed from
// the local store. Recomputes when the profile changes or a sync/commit bumps
// refreshKey.
export function TestPlanBoardView({ profileId, refreshKey, onSeeded }: Props) {
  const [plans, setPlans] = useState<Container[]>([]);
  const [selected, setSelected] = useState("");
  const [board, setBoard] = useState<TestPlanBoard | null>(null);
  const [loadingPlans, setLoadingPlans] = useState(true);
  const [error, setError] = useState("");
  const [seeding, setSeeding] = useState(false);

  async function seed() {
    setSeeding(true);
    setError("");
    try {
      await SeedSampleContainers(profileId);
      onSeeded();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setSeeding(false);
    }
  }

  async function newPlan() {
    const name = window.prompt("New Test Plan name:");
    if (!name || !name.trim()) return;
    setError("");
    try {
      await CreateContainerAndAllocate(profileId, "testplan", name.trim(), []);
      onSeeded();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function renamePlan() {
    if (!selected) return;
    const cur = plans.find((p) => p.key === selected);
    const name = window.prompt("Rename Test Plan to:", cur?.summary ?? "");
    if (name === null || !name.trim()) return;
    setError("");
    try {
      await EditContainer(profileId, selected, name.trim());
      onSeeded();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function deletePlan() {
    if (!selected) return;
    if (
      !window.confirm(
        "Delete this Test Plan? Its test memberships are removed (committed on sync).",
      )
    )
      return;
    setError("");
    try {
      await DeleteContainer(profileId, selected);
      setSelected("");
      onSeeded();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  useEffect(() => {
    if (!profileId) return;
    let cancelled = false;
    setLoadingPlans(true);
    setError("");
    ListContainers(profileId, "testplan")
      .then((ps) => {
        if (cancelled) return;
        setPlans(ps ?? []);
        setSelected((cur) => {
          if (cur && (ps ?? []).some((p) => p.key === cur)) return cur;
          return ps && ps.length > 0 ? ps[0].key : "";
        });
      })
      .catch((e) => {
        if (!cancelled) setError(errMsg(e));
      })
      .finally(() => {
        if (!cancelled) setLoadingPlans(false);
      });
    return () => {
      cancelled = true;
    };
  }, [profileId, refreshKey]);

  useEffect(() => {
    if (!profileId || !selected) {
      setBoard(null);
      return;
    }
    let cancelled = false;
    setError("");
    GetTestPlanBoard(profileId, selected)
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

  if (loadingPlans && plans.length === 0) {
    return <div className="board muted">Loading…</div>;
  }
  if (plans.length === 0) {
    return (
      <div className="board">
        <p className="muted">
          No Test Plans cached yet. Create one, run a sync, or generate sample
          data to try it out.
        </p>
        <div className="board-head-actions">
          <button className="btn btn-primary" onClick={newPlan}>
            New Test Plan
          </button>
          <button className="btn" onClick={seed} disabled={seeding}>
            {seeding ? "Generating…" : "Generate sample data"}
          </button>
        </div>
        {error && <div className="error-text">{error}</div>}
      </div>
    );
  }

  return (
    <div className="board">
      <div className="board-head">
        <label className="board-picker">
          <span>Test Plan</span>
          <select
            value={selected}
            onChange={(e) => setSelected(e.target.value)}
          >
            {plans.map((p) => (
              <option key={p.key} value={p.key}>
                {p.key} — {p.summary}
              </option>
            ))}
          </select>
        </label>
        {board && board.runCounts.length > 0 && (
          <div className="board-counts">
            {board.runCounts.map((b) => (
              <RunBadge key={b.label} status={b.label} count={b.count} />
            ))}
          </div>
        )}
        <div className="board-head-actions">
          <button className="btn" onClick={newPlan} title="Create a new Test Plan">
            New
          </button>
          <button
            className="btn"
            onClick={renamePlan}
            disabled={!selected}
            title="Rename the selected Test Plan"
          >
            Rename
          </button>
          <button
            className="btn"
            onClick={deletePlan}
            disabled={!selected}
            title="Delete the selected Test Plan"
          >
            Delete
          </button>
        </div>
        <button
          className="link-btn board-seed"
          onClick={seed}
          disabled={seeding}
          title="Regenerate sample sets / plans / executions from synced tests"
        >
          {seeding ? "Generating…" : "Regenerate sample data"}
        </button>
      </div>

      {error && <div className="error-text">{error}</div>}

      {board && (
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
                  This plan has no tests.
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

function RunBadge({ status, count }: { status: string; count: number }) {
  const cls =
    status === "(not run)" ? "run-badge" : `run-badge run-${status.toLowerCase()}`;
  return (
    <span className={cls}>
      {status === "(not run)" ? "not run" : status} {count}
    </span>
  );
}
