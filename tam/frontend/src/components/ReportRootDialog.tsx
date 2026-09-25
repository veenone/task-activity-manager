import { useId, useState } from "react";
import { Modal, errMsg } from "@agile-suite/core";
import type { RitualRoot } from "../api";

interface Props {
  // The space the page is made in, named in every sentence here so nobody
  // creates a page in a space they did not mean.
  spaceKey: string;
  suggestedTitle: string;
  // create is the Go call: a create (adopt false) or an adoption of the page
  // already holding the title (adopt true).
  create: (title: string, adopt: boolean) => Promise<RitualRoot>;
  // onChosen fires with the root that was made or adopted. Nothing is stored
  // here; the profile form saves the id with the rest of the profile.
  onChosen: (root: RitualRoot) => void;
  onClose: () => void;
}

type Step = { kind: "confirm" } | { kind: "working" } | { kind: "forbidden" } | { kind: "taken"; topLevel: boolean };

// ReportRootDialog picks the page a profile's sprint reports hang under, the
// way the rituals root is picked: a title, a page created at the top of the
// space, and the offer to take a page that already carries the title when it
// is top level. A root is a page, so it is chosen as one rather than typed in
// as an id copied out of a Confluence address.
export function ReportRootDialog({ spaceKey, suggestedTitle, create, onChosen, onClose }: Props) {
  const titleId = useId();
  const fieldId = useId();
  const [title, setTitle] = useState(suggestedTitle);
  const [step, setStep] = useState<Step>({ kind: "confirm" });
  const [error, setError] = useState("");
  const working = step.kind === "working";

  async function run(adopt: boolean) {
    if (working) return;
    const trimmed = title.trim();
    if (!trimmed) {
      setError("The root page needs a title");
      return;
    }
    setError("");
    setStep({ kind: "working" });
    try {
      const root = await create(trimmed, adopt);
      switch (root.outcome) {
        case "created":
        case "adopted":
          onChosen(root);
          return;
        case "forbidden":
          setStep({ kind: "forbidden" });
          return;
        case "titleTaken":
          setStep({ kind: "taken", topLevel: root.topLevel });
          return;
      }
    } catch (e) {
      setError(errMsg(e));
      setStep({ kind: "confirm" });
    }
  }

  return (
    <Modal onClose={onClose} className="modal" labelledBy={titleId} closeOnEsc={!working} closeOnOverlayClick={!working}>
      <h2 id={titleId}>Choose a reports root page</h2>
      <label htmlFor={fieldId}>Page title</label>
      <input
        id={fieldId}
        className="detail-input"
        value={title}
        disabled={working}
        spellCheck={false}
        onChange={(e) => setTitle(e.target.value)}
      />
      <p className="muted small">
        The page is created at the top of {spaceKey}, and every sprint report published from TAM becomes a page
        under it. It is made now; saving the profile is what keeps it as the reports root.
      </p>
      {step.kind === "forbidden" && (
        <p className="warn-text" role="status">
          This token may not create pages in {spaceKey}. Ask for permission there, or name a space you can write in.
        </p>
      )}
      {step.kind === "taken" && (
        <p className="warn-text" role="status">
          {step.topLevel
            ? `A page titled "${title.trim()}" is already at the top of ${spaceKey}. You can use that page as the reports root.`
            : `A page titled "${title.trim()}" is already in ${spaceKey}, below another page. Give the root a different title.`}
        </p>
      )}
      {error && <p className="error-text" role="alert">{error}</p>}
      <div className="form-actions form-actions-end">
        {step.kind === "taken" && (
          <button type="button" className="btn" onClick={() => setStep({ kind: "confirm" })}>Back</button>
        )}
        <button type="button" className="btn" onClick={onClose} disabled={working}>Cancel</button>
        {step.kind === "taken" && step.topLevel ? (
          <button type="button" className="btn btn-primary" onClick={() => void run(true)}>Use this page</button>
        ) : (
          <button
            type="button"
            className="btn btn-primary"
            onClick={() => void run(false)}
            disabled={step.kind === "forbidden"}
            aria-disabled={working || undefined}
          >
            {working ? "Working..." : "Create page"}
          </button>
        )}
      </div>
    </Modal>
  );
}
