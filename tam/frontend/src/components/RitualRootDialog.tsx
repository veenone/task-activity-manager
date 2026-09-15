import { useId, useState } from "react";
import { Modal, errMsg } from "@agile-suite/core";
import type { RitualRootMissing, RitualRootResult } from "../api";
import {
  ROOT_AFTER_SENTENCE, ROOT_TITLE_EMPTY, rootForbiddenSentence, rootMissingSentence, rootPlacementSentence,
  rootTakenNestedSentence, rootTakenTopSentence,
} from "../lib/ritualText";

type Step =
  | { kind: "confirm" }
  | { kind: "working"; adopt: boolean }
  | { kind: "forbidden" }
  | { kind: "taken"; title: string; topLevel: boolean };

interface Props {
  missing: RitualRootMissing;
  // create is the root create (adopt false) or adoption (adopt true) with the
  // Sync Go runs after it. The caller runs it through SyncContext.runRitualRoot.
  create: (title: string, adopt: boolean) => Promise<RitualRootResult>;
  // onDone fires once a root was set and the caller has applied what came back.
  onDone: () => void;
  onOpenProfiles: () => void;
  onClose: () => void;
}

// RitualRootDialog is what a Sync that found its root page gone opens. It
// states where a new page goes before anything is written, offers the create
// only when the permission probe did not say no, and asks a second time
// before adopting a page somebody else wrote. It refuses Escape and the
// overlay while its call is in flight, the way the sprint dialogs do: the call
// holds the profile lock, and closing under it would leave its outcome with
// nowhere to land.
export function RitualRootDialog({ missing, create, onDone, onOpenProfiles, onClose }: Props) {
  const titleId = useId();
  const [title, setTitle] = useState(missing.suggestedTitle);
  const [step, setStep] = useState<Step>(missing.canCreate ? { kind: "confirm" } : { kind: "forbidden" });
  const [error, setError] = useState("");
  const working = step.kind === "working";
  const editing = step.kind === "confirm" || step.kind === "working";

  async function run(value: string, adopt: boolean) {
    const trimmed = value.trim();
    if (!trimmed) {
      setError(ROOT_TITLE_EMPTY);
      return;
    }
    setError("");
    setStep({ kind: "working", adopt });
    try {
      const result = await create(trimmed, adopt);
      switch (result.root.outcome) {
        case "created":
        case "adopted":
          onDone();
          return;
        case "forbidden":
          setStep({ kind: "forbidden" });
          return;
        case "titleTaken":
          setStep({ kind: "taken", title: trimmed, topLevel: result.root.topLevel });
          return;
      }
    } catch (e) {
      setError(errMsg(e));
      setStep({ kind: "confirm" });
    }
  }

  const openProfiles = () => {
    onClose();
    onOpenProfiles();
  };

  return (
    <Modal onClose={onClose} className="modal ritual-root-modal" labelledBy={titleId} closeOnEsc={!working} closeOnOverlayClick={!working}>
      <h2 id={titleId}>Rituals root page not found</h2>
      <p>{rootMissingSentence(missing.pageId)}</p>
      {editing && (
        <>
          <label className="ritual-root-title">
            Page title
            <input
              className="detail-input"
              value={title}
              disabled={working}
              spellCheck={false}
              onChange={(e) => setTitle(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !working) {
                  e.preventDefault();
                  void run(title, false);
                }
              }}
            />
          </label>
          <p className="muted small">{rootPlacementSentence(missing.spaceKey)}</p>
          <p className="muted small">{ROOT_AFTER_SENTENCE}</p>
        </>
      )}
      {step.kind === "forbidden" && <p className="warn-text">{rootForbiddenSentence(missing.spaceKey)}</p>}
      {step.kind === "taken" && (
        <p className="warn-text">
          {step.topLevel ? rootTakenTopSentence(step.title, missing.spaceKey) : rootTakenNestedSentence(step.title, missing.spaceKey)}
        </p>
      )}
      {error && <p className="error-text" role="alert">{error}</p>}
      <div className="form-actions form-actions-end">
        {step.kind === "taken" && <button className="btn" onClick={() => setStep({ kind: "confirm" })}>Back</button>}
        {(step.kind === "taken" || step.kind === "forbidden") && <button className="btn" onClick={openProfiles}>Open Profile settings</button>}
        <button className="btn" onClick={onClose} disabled={working}>Cancel</button>
        {editing && (
          <button className="btn btn-primary" onClick={() => void run(title, false)} disabled={working}>
            {step.kind === "working" ? (step.adopt ? "Using page…" : "Creating page…") : "Create page and sync"}
          </button>
        )}
        {step.kind === "taken" && step.topLevel && (
          <button className="btn btn-primary" onClick={() => void run(step.title, true)}>Use this page and sync</button>
        )}
      </div>
    </Modal>
  );
}
