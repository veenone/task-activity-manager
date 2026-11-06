import { useEffect } from "react";
import { useFocusWhen } from "@agile-suite/core";
import type { query } from "../wailsjs/go/models";
import { projectLabel } from "./linked";

// IssueDetail is the issue chosen in Results, with the client impact text that
// is too long for a column. Focus moves to its close button when it opens, and
// Escape closes it.
export function IssueDetail({ issue, onClose }: { issue: query.Issue; onClose: () => void }) {
  const close = useFocusWhen<HTMLButtonElement>(issue.key);
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);
  return (
    <aside className="pie-detail" aria-label={`${issue.key} details`}>
      <div className="pie-detail-head">
        <h2 className="mono">{issue.key}</h2>
        <button ref={close} type="button" className="btn btn-ghost" onClick={onClose}>
          Close details
        </button>
      </div>
      <p>{issue.summary}</p>
      <dl className="pie-detail-facts">
        <dt>Project</dt>
        <dd>{projectLabel(issue.project, issue.projectName)}</dd>
        <dt>Status</dt>
        <dd>{issue.status}</dd>
        <dt>Priority</dt>
        <dd>{issue.priority}</dd>
        <dt>Client visibility</dt>
        <dd>{issue.clientVisibility || "-"}</dd>
      </dl>
      <h3>Client impact</h3>
      <p className={issue.clientImpact ? "pie-detail-impact" : "muted"}>{issue.clientImpact || "No client impact recorded."}</p>
    </aside>
  );
}
