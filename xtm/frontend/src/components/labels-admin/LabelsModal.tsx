import { useId, useMemo, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Modal, useConfirm, usePrompt } from "@agile-suite/core";
import { useProfile } from "../../contexts/ProfileContext";
import { useLabels } from "../../queries/app";
import { keys } from "../../queries/keys";
import { DeleteLabel, RenameLabel, errMsg } from "../../api";
import { Pager } from "../Pager";
import { SortControl } from "../SortControl";

interface Props {
  // Called after a change is queued, so the pending list and tests reload.
  onChanged: () => void;
  onClose: () => void;
}

const PAGE_SIZES = [15, 25, 50, 100];

function changes(n: number) {
  return `${n} pending change${n === 1 ? "" : "s"}`;
}

// LabelsModal lists every label on the profile's synced tests and renames,
// merges or deletes one across all of them. Jira has no label admin API, so
// each action is a bulk edit of the tests carrying the label, queued like any
// other edit. Before queueing, the confirm says how many changes that is: a
// label on 2,000 tests is 2,000 issue updates on Commit.
export function LabelsModal({ onChanged, onClose }: Props) {
  const { activeId: profileId } = useProfile();
  const qc = useQueryClient();
  const titleId = useId();
  const labels = useLabels(profileId);
  const { confirm } = useConfirm();
  const { prompt } = usePrompt();
  const [filter, setFilter] = useState("");
  const [sortField, setSortField] = useState("label");
  const [sortDesc, setSortDesc] = useState(false);
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState(PAGE_SIZES[0]);
  const [status, setStatus] = useState("");
  const [error, setError] = useState("");

  const buckets = useMemo(() => labels.data ?? [], [labels.data]);
  const shown = useMemo(() => {
    const q = filter.trim().toLowerCase();
    const dir = sortDesc ? -1 : 1;
    return buckets
      .filter((b) => b.label.toLowerCase().includes(q))
      .sort((a, b) =>
        sortField === "tests"
          ? dir * (a.count - b.count)
          : dir * a.label.localeCompare(b.label),
      );
  }, [buckets, filter, sortField, sortDesc]);
  const pageItems = shown.slice(page * pageSize, (page + 1) * pageSize);

  async function queue(run: () => Promise<{ succeeded: string[]; failed: { testKey: string; error: string }[] }>) {
    setError("");
    setStatus("");
    try {
      const res = await run();
      setStatus(`Queued ${changes(res.succeeded.length)}. Commit to send them to Jira.`);
      if (res.failed.length > 0) {
        setError(res.failed.map((f) => `${f.testKey}: ${f.error}`).join("; "));
      }
    } catch (e) {
      setError(errMsg(e));
    } finally {
      await qc.invalidateQueries({ queryKey: keys.labels(profileId) });
      onChanged();
    }
  }

  async function rename(label: string, count: number) {
    const next = (
      await prompt({ title: `Rename ${label}`, defaultValue: label, submitLabel: "Next" })
    )?.trim();
    if (!next || next === label) return;
    const merge = buckets.some((b) => b.label === next);
    const ok = await confirm({
      title: merge ? `Merge "${label}" into "${next}"?` : `Rename "${label}" to "${next}"?`,
      message: `This queues ${changes(count)}, one per synced test carrying "${label}". Each reaches Jira as an issue update when you commit.`,
      confirmLabel: merge ? "Merge" : "Rename",
      danger: false,
    });
    if (ok) await queue(() => RenameLabel(profileId, label, next));
  }

  async function remove(label: string, count: number) {
    const ok = await confirm({
      title: `Delete "${label}"?`,
      message: `This queues ${changes(count)}, one per synced test carrying "${label}". Each reaches Jira as an issue update when you commit.`,
      confirmLabel: "Delete",
    });
    if (ok) await queue(() => DeleteLabel(profileId, label));
  }

  return (
    <Modal onClose={onClose} className="modal pending-modal" labelledBy={titleId}>
      <div className="pending-head">
        <h2 id={titleId}>Labels</h2>
        <button className="btn btn-ghost" onClick={onClose} title="Close" aria-label="Close">
          ✕
        </button>
      </div>
      <div className="bulk-body">
        <p className="muted">
          These are the labels on your synced tests. Tests you have not synced
          keep their labels. Changes are queued and reach Jira on Commit.
        </p>
        <div className="labels-admin-head">
          <input
            className="labels-admin-search"
            value={filter}
            placeholder="Filter labels…"
            aria-label="Filter labels"
            onChange={(e) => {
              setFilter(e.target.value);
              setPage(0);
            }}
          />
          <SortControl
            fields={[
              { value: "label", label: "Label" },
              { value: "tests", label: "Tests" },
            ]}
            field={sortField}
            desc={sortDesc}
            onChange={(f, d) => {
              setSortField(f);
              setSortDesc(d);
              setPage(0);
            }}
          />
        </div>
        {error && <div className="error-text">{error}</div>}
        {status && <p className="ok-text">{status}</p>}
        {labels.isLoading ? (
          <p className="muted">Loading labels…</p>
        ) : labels.error ? (
          <p className="error-text">{errMsg(labels.error)}</p>
        ) : buckets.length === 0 ? (
          <p className="muted">No synced test has a label.</p>
        ) : shown.length === 0 ? (
          <p className="muted">No label matches that filter.</p>
        ) : (
          <>
            <table aria-label="Labels" className="board-table">
              <thead>
                <tr>
                  <th scope="col">Label</th>
                  <th scope="col">Tests</th>
                  <th scope="col">
                    <span className="sr-only">Actions</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {pageItems.map((b) => (
                  <tr key={b.label}>
                    <td className="mono">{b.label}</td>
                    <td>{b.count}</td>
                    <td className="labels-admin-actions">
                      <button
                        className="btn btn-ghost"
                        aria-label={`Rename ${b.label}`}
                        onClick={() => rename(b.label, b.count)}
                      >
                        Rename…
                      </button>
                      <button
                        className="btn btn-ghost"
                        aria-label={`Delete ${b.label}`}
                        onClick={() => remove(b.label, b.count)}
                      >
                        Delete
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            <Pager
              page={page}
              pageSize={pageSize}
              total={shown.length}
              pageSizeOptions={PAGE_SIZES}
              onPage={setPage}
              onPageSize={(n) => {
                setPageSize(n);
                setPage(0);
              }}
            />
          </>
        )}
      </div>
    </Modal>
  );
}
