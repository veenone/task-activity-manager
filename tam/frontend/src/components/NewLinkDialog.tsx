import { useId, useState } from "react";
import { DialogBody, DialogFooter, DialogHeader, Modal } from "@agile-suite/core";
import type { LinkType } from "../api";
import { isBlocks } from "../lib/dependencyGraph";

interface Props {
  source: string;
  target: string;
  types: LinkType[];
  busy: boolean;
  // onAdd journals the link; it resolves false when the dialog should stay.
  onAdd: (type: string, direction: "outward" | "inward") => Promise<boolean>;
  onClose: () => void;
}

// NewLinkDialog asks which link a drag from source to target makes (#612).
// It lists each type's outward and inward wording the way the panel's Add
// link form does, so "blocks" and "is blocked by" read as sentences about
// the two issues. Blocks outward comes first: dragging from the blocker to
// what it holds up is the common case.
export function NewLinkDialog({ source, target, types, busy, onAdd, onClose }: Props) {
  const titleId = useId();
  const options: { value: string; label: string }[] = [];
  const ordered = [...types.filter((t) => isBlocks(t.name)), ...types.filter((t) => !isBlocks(t.name))];
  for (const t of ordered) {
    options.push({ value: `${t.name}|outward`, label: t.outward });
    if (t.inward !== t.outward) options.push({ value: `${t.name}|inward`, label: t.inward });
  }
  const [choice, setChoice] = useState("");
  const selected = choice || options[0]?.value || "";
  const [type, direction] = selected.split("|") as [string, "outward" | "inward"];
  const wording = options.find((o) => o.value === selected)?.label ?? "";

  return (
    <Modal onClose={onClose} className="modal new-link-modal" labelledBy={titleId} closeOnEsc={!busy} closeOnOverlayClick={!busy}>
      <DialogHeader titleId={titleId} title="Link two issues" onClose={onClose} disabled={busy} />
      <DialogBody>
        <label className="edit-row" htmlFor="new-link-type">
          <span className="muted small">{source}</span>
          <select id="new-link-type" className="detail-input" value={selected} onChange={(e) => setChoice(e.target.value)} disabled={options.length === 0}>
            {options.map((o) => (
              <option key={o.value} value={o.value}>{o.label}</option>
            ))}
          </select>
          <span className="muted small">{target}</span>
        </label>
        {options.length === 0 ? (
          <p className="muted small">No link types are known yet. Sync, then try again.</p>
        ) : (
          <p className="small">{`${source} ${wording} ${target}. Journaled now, pushed on Commit.`}</p>
        )}
      </DialogBody>
      <DialogFooter>
        <button type="button" className="btn" onClick={onClose} disabled={busy}>Cancel</button>
        <button type="button" className="btn btn-primary" disabled={!selected || busy} onClick={() => void onAdd(type, direction)}>
          Add link
        </button>
      </DialogFooter>
    </Modal>
  );
}
