import { useId, useState } from "react";
import { CreateReportRoot } from "../../api";
import type { Profile } from "../../api";
import { ReportRootDialog } from "../ReportRootDialog";
import type { TamProfileExtras } from "./useTamProfileExtras";

interface Props {
  x: TamProfileExtras;
  profile?: Profile;
  projectKey: string;
  demo: boolean;
}

// ConfluenceFieldset is the optional Confluence Rituals section: where rituals
// are published, the token, and where sprint reports go.
export function ConfluenceFieldset({ x, profile, projectKey, demo }: Props) {
  const isEdit = !!profile;
  const rootErrorId = useId();
  const reportsRootId = useId();
  const [pickingReportsRoot, setPickingReportsRoot] = useState(false);
  return (
    <>
      <details className="profile-form-advanced">
        <summary>Confluence Rituals (optional)</summary>
        <label>
          Confluence base URL
          <input value={x.confluenceURL} onChange={(e) => x.setConfluenceURL(e.target.value)} placeholder="https://confluence.example.com" spellCheck={false} />
        </label>
        <label>
          Space key
          <input value={x.confluenceSpace} onChange={(e) => x.setConfluenceSpace(e.target.value)} placeholder="TEAM" spellCheck={false} />
        </label>
        <label>
          Root page ID (optional)
          <input
            value={x.confluenceRootPageID}
            onChange={(e) => x.setConfluenceRootPageID(e.target.value)}
            onBlur={() => {
              if (x.rootInput.id && !x.rootInput.error) x.setConfluenceRootPageID(x.rootInput.id);
            }}
            placeholder="123456, or paste the page's address"
            spellCheck={false}
            aria-invalid={x.rootInput.error ? true : undefined}
            aria-describedby={x.rootInput.error ? rootErrorId : undefined}
          />
          {x.rootInput.error && <span id={rootErrorId} className="field-error">{x.rootInput.error}</span>}
        </label>
        <label>
          Confluence personal access token
          <input type="password" value={x.confluenceToken} onChange={(e) => x.setConfluenceToken(e.target.value)} placeholder={isEdit ? "Leave blank to keep the current token" : "Stored in Windows Credential Manager"} autoComplete="off" />
        </label>
        {/* Where sprint reports go. A report is not a ritual and may belong in
            another space or under another page, so it has its own pair. Both
            left blank publishes where the profile always did. */}
        <label>
          Reports space key (optional)
          <input
            value={x.reportsSpace}
            onChange={(e) => x.setReportsSpace(e.target.value)}
            placeholder="The rituals space above"
            spellCheck={false}
          />
        </label>
        <label htmlFor={reportsRootId}>Reports root page (optional)</label>
        <div className="reports-root-field">
          <input
            id={reportsRootId}
            value={x.reportsRootPageID}
            readOnly
            spellCheck={false}
            placeholder="The sprint's own ritual page"
          />
          <button
            type="button"
            className="btn"
            disabled={!isEdit || !x.confluenceLoaded}
            title={isEdit ? "Create or take the page reports hang under" : "Save the profile first, then choose the page"}
            onClick={() => setPickingReportsRoot(true)}
          >
            Choose page...
          </button>
          {x.reportsRootPageID && (
            <button type="button" className="btn" onClick={() => x.setReportsRootPageID("")}>Clear</button>
          )}
        </div>
        <span className="field-hint">
          Leave both blank and a report is published where it always was: the rituals space, under the sprint's
          own page or the rituals root.
        </span>
        {demo && <span className="field-hint">Demo profile sample: use <code>demo</code> as the Confluence URL. It is a local configuration example and does not contact a server.</span>}
      </details>
      {pickingReportsRoot && profile && (
        <ReportRootDialog
          spaceKey={x.reportsSpace.trim() || x.confluenceSpace.trim()}
          suggestedTitle={`${projectKey.trim().toUpperCase()} Reports`.trim()}
          create={(title, adopt) => CreateReportRoot(profile.id, x.reportsSpace.trim(), title, adopt)}
          onChosen={(root) => {
            x.setReportsRootPageID(root.pageId);
            setPickingReportsRoot(false);
          }}
          onClose={() => setPickingReportsRoot(false)}
        />
      )}
    </>
  );
}
