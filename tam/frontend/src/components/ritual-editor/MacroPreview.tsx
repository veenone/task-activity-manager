import { useContext, useEffect, useState } from "react";
import { toPlainText } from "@agile-suite/core";
import { RitualMacroIssues } from "../../api";
import type { RitualMacroPreview } from "../../api";
import { MACRO_CAVEAT } from "../../lib/ritualText";
import { parseXml } from "../../lib/storage/xml";
import { RitualEditorContext } from "./context";

export const PREVIEW_LIMIT = 50;

export function macroJql(xml: string): string {
  const macro = parseXml(xml)?.firstElementChild;
  if (!macro) return "";
  for (const p of Array.from(macro.children)) {
    if (p.nodeName === "ac:parameter" && p.getAttribute("ac:name") === "jqlQuery") return (p.textContent ?? "").trim();
  }
  return "";
}

// MacroPreview draws a Jira Issues macro from TAM's cache, so a ritual page
// shows its issues offline. Only the three forms the templates write are
// answered; the caveat says the cache's "done" is not Jira's statusCategory.
export function MacroPreview({ xml }: { xml: string }) {
  const { profileId } = useContext(RitualEditorContext);
  const jql = macroJql(xml);
  const [preview, setPreview] = useState<RitualMacroPreview | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let live = true;
    setPreview(null);
    setError("");
    if (!profileId || !jql) return;
    RitualMacroIssues(profileId, jql)
      .then((p) => { if (live) setPreview(p); })
      .catch((e) => { if (live) setError(String(e)); });
    return () => { live = false; };
  }, [profileId, jql]);

  if (!jql) return <span className="muted small">Rendered in Confluence</span>;
  if (error) return <span className="warn-text small">{error}</span>;
  if (!preview) return <span className="muted small">Reading the cache</span>;
  if (!preview.supported) {
    return <span className="small"><code>{jql}</code> <span className="muted">Rendered in Confluence</span></span>;
  }
  const shown = preview.issues.slice(0, PREVIEW_LIMIT);
  return (
    <div className="ritual-macro-preview">
      <code className="small">{jql}</code>
      {shown.length === 0 ? (
        <p className="muted small">No cached issues match.</p>
      ) : (
        <table className="ritual-macro-table">
          <thead><tr><th>Key</th><th>Summary</th><th>Status</th><th>Assignee</th></tr></thead>
          <tbody>
            {shown.map((i) => <tr key={i.key}><td>{i.key}</td><td>{toPlainText(i.summary, "summary")}</td><td>{i.status}</td><td>{i.assignee}</td></tr>)}
          </tbody>
        </table>
      )}
      <p className="muted small">{MACRO_CAVEAT}</p>
    </div>
  );
}
