import { useState } from "react";
import type { FormEvent } from "react";
import { Modal, announce, errMsg, useProfile } from "@agile-suite/core";
import type { Profile, Settings } from "../api";
import { useCreateDraftBoard } from "../queries/boards";

interface Props {
  profileId: string;
  onClose: () => void;
  // onCreated hands back the board's negative placeholder id, so the picker
  // can switch to it at once, and the sentence the banner reports.
  onCreated: (boardId: number, line: string) => void;
}

// NewBoardModal drafts a new board, modelled on CreateSprintModal. Unlike
// that dialog, CreateDraftBoard makes no network call at all: it is a local
// write, so there is no lock to hold, no in-flight state to disable the
// buttons for, and no JQL validation call to make, in a feature whose whole
// point is working with no connection. A bad JQL fails at Commit with
// Jira's own message.
export function NewBoardModal({ profileId, onClose, onCreated }: Props) {
  const { activeProfile } = useProfile<Profile, Settings>();
  const create = useCreateDraftBoard(profileId);
  const [name, setName] = useState("");
  const [type, setType] = useState("scrum");
  const [filterName, setFilterName] = useState("");
  const [jql, setJql] = useState("");
  const [error, setError] = useState("");

  const filterPlaceholder = `Filter for ${name.trim() || "board"}`;
  const jqlPlaceholder = `project = ${activeProfile?.projectKey ?? "KEY"} ORDER BY Rank`;

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    const trimmed = name.trim();
    if (!trimmed) {
      setError("The board needs a name.");
      return;
    }
    create.mutate(
      {
        name: trimmed,
        type,
        filterName: filterName.trim() || `Filter for ${trimmed}`,
        jql: jql.trim() || jqlPlaceholder,
      },
      {
        onSuccess: (boardId) => {
          const line = `${trimmed} was drafted. Commit creates it in Jira.`;
          announce(line);
          onCreated(boardId, line);
          onClose();
        },
        onError: (err) => setError(errMsg(err)),
      },
    );
  }

  return (
    <Modal onClose={onClose} className="modal pending-modal" labelledBy="new-board-title">
      <div className="pending-head">
        <h2 id="new-board-title">New board</h2>
        <p className="muted small">Drafted locally. Commit creates it in Jira.</p>
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>

      <form id="new-board-form" className="bulk-body edit-form" onSubmit={onSubmit}>
        <label className="edit-row" htmlFor="new-board-name">
          <span className="muted small">Name</span>
          <input
            id="new-board-name"
            className="detail-input"
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </label>
        <label className="edit-row" htmlFor="new-board-type">
          <span className="muted small">Type</span>
          <select id="new-board-type" className="detail-input" value={type} onChange={(e) => setType(e.target.value)}>
            <option value="scrum">Scrum</option>
            <option value="kanban">Kanban</option>
          </select>
        </label>
        <label className="edit-row" htmlFor="new-board-filter">
          <span className="muted small">Filter name</span>
          <input
            id="new-board-filter"
            className="detail-input"
            type="text"
            placeholder={filterPlaceholder}
            value={filterName}
            onChange={(e) => setFilterName(e.target.value)}
          />
        </label>
        <label className="edit-row" htmlFor="new-board-jql">
          <span className="muted small">JQL</span>
          <input
            id="new-board-jql"
            className="detail-input"
            type="text"
            placeholder={jqlPlaceholder}
            value={jql}
            onChange={(e) => setJql(e.target.value)}
          />
        </label>
      </form>

      <div className="pending-actions">
        <span className="new-issue-status">
          {error && <span className="error-text small" role="alert">{error}</span>}
        </span>
        <span className="pending-footer-buttons">
          <button type="button" className="btn" onClick={onClose}>Cancel</button>
          <button type="submit" form="new-board-form" className="btn btn-primary">Create board</button>
        </span>
      </div>
    </Modal>
  );
}
