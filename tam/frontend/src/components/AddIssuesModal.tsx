import { useMemo, useState } from "react";
import { Modal, announce, errMsg, toPlainText } from "@agile-suite/core";
import type { Sprint } from "../api";
import { useIssues } from "../queries/issues";
import { useBoardSprintDetails } from "../queries/sprints";
import { useAddIssuesToBoard } from "../queries/boards";
import { useDebounced } from "../lib/useDebounced";
import { sprintOption } from "./BoardsToolbar";

const SEARCH_DELAY_MS = 250;
const RESULT_LIMIT = 25;
const BACKLOG_SCOPE = "backlog";

interface Props {
  profileId: string;
  boardId: number;
  boardName: string;
  // sprints are the board's own open ones, the same list the toolbar and
  // BoardSelectionBar offer: what the sync fetched membership for.
  sprints: Sprint[];
  onClose: () => void;
  onAdded: (line: string) => void;
}

// AddIssuesModal is the picker beside New sprint: it searches the issue
// cache the Backlog already searches (drafts included, since ListIssues
// already lists them ahead of the ranked rows), leaves out whatever this
// board already holds, and journals AddIssuesToBoard for the ones checked.
// Also a local write, like CreateDraftBoard: no lock, nothing in flight to
// guard the buttons against.
export function AddIssuesModal({ profileId, boardId, boardName, sprints, onClose, onAdded }: Props) {
  const [text, setText] = useState("");
  const [checked, setChecked] = useState<Set<string>>(new Set());
  const [into, setInto] = useState(BACKLOG_SCOPE);
  const [error, setError] = useState("");
  const search = useDebounced(text, SEARCH_DELAY_MS, boardId);
  const add = useAddIssuesToBoard(profileId);

  const query = useMemo(
    () => ({ text: search, types: [], sprintId: "", offset: 0, limit: RESULT_LIMIT, sort: "" as const, desc: false }),
    [search],
  );
  const issues = useIssues(profileId, query);
  const details = useBoardSprintDetails(profileId, boardId);

  // member is every key this board already holds, across its sprints and its
  // own backlog: the Sprints view's own read is the only one that already
  // spans the whole board, so this reuses it rather than asking Jira or the
  // cache a second way.
  const member = useMemo(() => {
    const keys = new Set<string>();
    for (const node of details.data ?? []) {
      for (const i of node.issues) keys.add(i.key);
    }
    return keys;
  }, [details.data]);

  const candidates = (issues.data?.issues ?? []).filter((i) => !member.has(i.key));

  function toggle(key: string) {
    setChecked((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key); else next.add(key);
      return next;
    });
  }

  function onAdd() {
    if (checked.size === 0) return;
    setError("");
    add.mutate(
      { keys: [...checked], boardId, scope: into },
      {
        onSuccess: () => {
          const destination = into === BACKLOG_SCOPE ? "the backlog" : sprints.find((s) => String(s.id) === into)?.name || "the sprint";
          const line = `${checked.size} issue${checked.size === 1 ? "" : "s"} queued onto ${boardName}'s ${destination}. Commit pushes it to Jira.`;
          announce(line);
          onAdded(line);
          onClose();
        },
        onError: (err) => setError(errMsg(err)),
      },
    );
  }

  return (
    <Modal onClose={onClose} className="modal pending-modal" labelledBy="add-issues-title">
      <div className="pending-head">
        <h2 id="add-issues-title">Add issues to {boardName}</h2>
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>

      <div className="bulk-body">
        <label className="edit-row" htmlFor="add-issues-search">
          <span className="muted small">Search</span>
          <input
            id="add-issues-search"
            className="detail-input"
            type="search"
            placeholder="key, summary, or assignee"
            value={text}
            onChange={(e) => setText(e.target.value)}
          />
        </label>

        <ul aria-label="Matching issues">
          {candidates.length === 0 && <li className="muted small">{issues.isPending ? "Searching…" : "No matching issues outside this board."}</li>}
          {candidates.map((i) => (
            <li key={i.key}>
              <label className="edit-row">
                <input type="checkbox" checked={checked.has(i.key)} onChange={() => toggle(i.key)} />
                <span>{i.key}</span>
                <span className="muted small">{toPlainText(i.summary, "summary")}</span>
              </label>
            </li>
          ))}
        </ul>

        <label className="edit-row" htmlFor="add-issues-into">
          <span className="muted small">Into</span>
          <select id="add-issues-into" className="detail-input" value={into} onChange={(e) => setInto(e.target.value)}>
            <option value={BACKLOG_SCOPE}>Backlog</option>
            {sprints.map((s) => (
              <option key={s.id} value={String(s.id)}>{sprintOption(s)}</option>
            ))}
          </select>
        </label>

        {error && <p className="error-text small" role="alert">{error}</p>}
      </div>

      <div className="pending-actions">
        <span className="new-issue-status">{checked.size > 0 && <span className="muted small">{checked.size} selected</span>}</span>
        <span className="pending-footer-buttons">
          <button type="button" className="btn" onClick={onClose}>Cancel</button>
          <button type="button" className="btn btn-primary" onClick={onAdd} disabled={checked.size === 0}>Add</button>
        </span>
      </div>
    </Modal>
  );
}
