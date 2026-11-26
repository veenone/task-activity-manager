import { useState } from "react";
import type { FormEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Modal, MultiSelect, call, errMsg } from "@agile-suite/core";
import { SetIssuesCatalog } from "../api";
import type { CatalogField } from "../api";
import { PERM_EDIT_ISSUES, PERM_RESOLVE_ISSUES } from "../api/permissions";
import { useComponents, useVersions } from "../queries/catalog";
import { useDenial } from "../queries/permissions";
import { invalidateWrites } from "../queries/invalidate";
import { catalogOptions } from "../lib/catalog";
import { plural } from "../lib/format";

interface Props {
  profileId: string;
  // keys are the checked issues, in the order the grid draws them.
  keys: string[];
  onClear: () => void;
}

// What each action is called on its button and dialog, and what the picker
// inside is named.
const ACTIONS: Record<CatalogField, { title: string; picker: string }> = {
  fixVersions: { title: "Set release", picker: "Releases" },
  components: { title: "Set components", picker: "Components" },
};

// CatalogBulkBar is what the Backlog's checked issues can be done with: set
// a release or components on all of them at once. It is the board's
// selection bar, class for class. Each action journals one edit per issue,
// so nothing reaches Jira until Commit and a discard undoes it per issue.
//
// The denial is checked against the profile's project, which is where the
// catalog belongs; an issue from another project is skipped by the backend
// and named afterwards rather than refused up front.
export function CatalogBulkBar({ profileId, keys, onClear }: Props) {
  const [open, setOpen] = useState<CatalogField | "">("");
  const [skipped, setSkipped] = useState<string[]>([]);
  const denial = useDenial(profileId);
  const editDenied = denial(PERM_EDIT_ISSUES);
  const denied = (field: CatalogField) => editDenied || (field === "fixVersions" ? denial(PERM_RESOLVE_ISSUES) : "");

  return (
    <div className="bulk-toolbar" role="group" aria-label="Checked issues">
      <span className="bulk-count">{plural(keys.length, "issue", "issues")} selected</span>
      {(Object.keys(ACTIONS) as CatalogField[]).map((field) => (
        <button key={field} type="button" className="btn" disabled={!!denied(field)} title={denied(field) || undefined} onClick={() => setOpen(field)}>
          {ACTIONS[field].title}
        </button>
      ))}
      <button type="button" className="btn" onClick={onClear}>Clear</button>
      {skipped.length > 0 && (
        <span className="muted small" role="status">{`Skipped ${skipped.join(", ")}: not in this profile's project.`}</span>
      )}
      {open && (
        <SetCatalogDialog
          profileId={profileId}
          field={open}
          keys={keys}
          onClose={() => setOpen("")}
          onDone={(left) => {
            setOpen("");
            setSkipped(left);
            onClear();
          }}
        />
      )}
    </div>
  );
}

interface DialogProps {
  profileId: string;
  field: CatalogField;
  keys: string[];
  onClose: () => void;
  onDone: (skipped: string[]) => void;
}

// SetCatalogDialog picks the values and whether they add to what each issue
// has or replace it. Adding is the default because it is the one that
// cannot take anything off an issue.
function SetCatalogDialog({ profileId, field, keys, onClose, onDone }: DialogProps) {
  const qc = useQueryClient();
  const versions = useVersions(profileId);
  const components = useComponents(profileId);
  const [picked, setPicked] = useState<string[]>([]);
  const [how, setHow] = useState<"add" | "replace">("add");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const rows = (field === "fixVersions" ? versions.data : components.data) ?? [];
  const { title, picker } = ACTIONS[field];
  const titleId = `set-${field}-title`;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (saving) return;
    setSaving(true);
    setError("");
    try {
      const res = await call(() => SetIssuesCatalog(profileId, keys, field, picked.map(Number), how === "add"));
      invalidateWrites(qc, profileId);
      onDone(res.skipped ?? []);
    } catch (err) {
      setError(errMsg(err));
      setSaving(false);
    }
  }

  return (
    <Modal onClose={onClose} className="modal pending-modal set-catalog-modal" labelledBy={titleId} closeOnOverlayClick={false} closeOnEsc={!saving}>
      <div className="pending-head">
        <h2 id={titleId}>{title}</h2>
        <p className="muted small">{`One pending edit per issue, on ${plural(keys.length, "issue", "issues")}. Commit sends them to Jira.`}</p>
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>
      <form id={`set-${field}-form`} className="bulk-body edit-form" onSubmit={(e) => void onSubmit(e)}>
        <div className="edit-row">
          <label className="muted small" htmlFor={`set-${field}-values`}>{picker}</label>
          <MultiSelect id={`set-${field}-values`} label={picker} options={catalogOptions(rows, [])} selected={picked} onChange={setPicked} allLabel="None" />
        </div>
        <div className="edit-row">
          <label className="muted small" htmlFor={`set-${field}-how`}>How</label>
          <select id={`set-${field}-how`} className="detail-input" value={how} onChange={(e) => setHow(e.target.value === "replace" ? "replace" : "add")}>
            <option value="add">Add to what each issue has</option>
            <option value="replace">Replace what each issue has</option>
          </select>
        </div>
      </form>
      <div className="pending-actions">
        <span className="new-issue-status">
          {error && <span className="error-text small" role="alert">{error}</span>}
        </span>
        <span className="pending-footer-buttons">
          <button type="button" className="btn" onClick={onClose} disabled={saving}>Cancel</button>
          <button type="submit" form={`set-${field}-form`} className="btn btn-primary" disabled={saving}>
            {saving ? "Saving" : `Set on ${plural(keys.length, "issue", "issues")}`}
          </button>
        </span>
      </div>
    </Modal>
  );
}
