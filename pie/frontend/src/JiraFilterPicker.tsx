import { useEffect, useState } from "react";
import { Modal } from "@agile-suite/core";
import { JiraFilter, JiraFilters } from "../wailsjs/go/main/App";
import type { jira } from "../wailsjs/go/models";
import { errorText } from "./AddProfileDialog";

// JiraFilterPicker starts a query from a saved Jira filter: one the account
// starred, or any it can see by the id in the filter's Jira address, because
// Data Center lists starred filters only. onPick gets the filter's id.
export function JiraFilterPicker({
  profileId,
  disabled,
  onPick,
}: {
  profileId: string;
  disabled: boolean;
  onPick: (id: string) => void;
}) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button type="button" className="btn btn-ghost" disabled={disabled} onClick={() => setOpen(true)}>
        From a Jira filter
      </button>
      {open && (
        <FilterDialog
          profileId={profileId}
          onClose={() => setOpen(false)}
          onPick={(id) => {
            setOpen(false);
            onPick(id);
          }}
        />
      )}
    </>
  );
}

function FilterDialog({ profileId, onClose, onPick }: { profileId: string; onClose: () => void; onPick: (id: string) => void }) {
  const [starred, setStarred] = useState<jira.RawFilter[] | null>(null);
  const [id, setId] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    let current = true;
    JiraFilters(profileId).then(
      (list) => current && setStarred(list),
      (e) => current && setError(errorText(e)),
    );
    return () => {
      current = false;
    };
  }, [profileId]);

  async function openById() {
    setError("");
    try {
      onPick((await JiraFilter(profileId, id)).id);
    } catch (e) {
      setError(errorText(e));
    }
  }

  return (
    <Modal onClose={onClose} labelledBy="pie-jira-filter">
      <div className="profile-form">
        <h2 id="pie-jira-filter">Start from a Jira filter</h2>
        {starred === null && !error && <p className="muted">Reading your starred filters...</p>}
        {starred?.length === 0 && <p className="muted">You have starred no filters in Jira. Enter a filter's id below.</p>}
        {starred && starred.length > 0 && (
          <ul className="pie-filter-list" aria-label="Starred filters">
            {starred.map((f) => (
              <li key={f.id}>
                <button type="button" className="pie-map-key" onClick={() => onPick(f.id)}>
                  {f.name}
                </button>
                <span className="muted mono">{f.jql}</span>
              </li>
            ))}
          </ul>
        )}
        {/* No form here: the picker sits inside the query form, and a nested
            form's submit would run the query too. */}
        <div className="pie-filter-id">
          <label>
            Filter id
            <input
              inputMode="numeric"
              value={id}
              onChange={(e) => setId(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  if (id.trim() !== "") void openById();
                }
              }}
            />
          </label>
          <button type="button" className="btn" disabled={id.trim() === ""} onClick={() => void openById()}>
            Open
          </button>
        </div>
        {error && <p className="error-text">{error}</p>}
        <div className="form-actions form-actions-end">
          <button type="button" className="btn" onClick={onClose}>
            Cancel
          </button>
        </div>
      </div>
    </Modal>
  );
}
