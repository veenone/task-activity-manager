import type { GNode } from "./graphModel";
import { PARENT } from "./graphModel";

// IssueDetails is the issue graph's right-hand panel: the chosen issue's
// fields, its links by type, and what can be done with it (#464).
export function IssueDetails({
  node,
  label,
  unloaded,
  busy,
  canOpenInJira,
  centred,
  onExpand,
  onCentre,
  onTrace,
  onJira,
  onPick,
}: {
  node: GNode;
  label: (project: string) => string;
  unloaded: number;
  busy: boolean;
  canOpenInJira: boolean;
  centred: boolean;
  onExpand: () => void;
  onCentre: () => void;
  onTrace: () => void;
  onJira: () => void;
  onPick: (key: string) => void;
}) {
  const is = node.issue;
  // Links worded from this issue's side, then parent and sub-tasks.
  const rows: [string, string[]][] = [];
  const add = (words: string, key: string) => {
    const row = rows.find(([w]) => w === words);
    if (row) row[1].push(key);
    else rows.push([words, [key]]);
  };
  for (const l of is.links ?? []) add(l.words || l.type, l.key);
  if (is.parent) add("sub-task of", is.parent);
  for (const s of is.subtasks ?? []) add("parent of", s);
  const day = (iso: string) => (iso ? iso.slice(0, 10) : "-");
  return (
    <aside className="pie-graph-details" aria-label={`Issue ${is.key}`}>
      <span className="mono muted">{`${is.key} · ${is.type || "Issue"}`}</span>
      <h3>{is.summary || is.key}</h3>
      <dl className="pie-graph-facts">
        <dt>Status</dt>
        <dd>{is.status || "-"}</dd>
        <dt>Priority</dt>
        <dd>{is.priority || "-"}</dd>
        <dt>Project</dt>
        <dd>{label(is.project)}</dd>
        <dt>Created</dt>
        <dd>{day(is.created)}</dd>
        <dt>Updated</dt>
        <dd>{day(is.updated)}</dd>
        <dt>Resolved</dt>
        <dd>{day(is.resolved)}</dd>
        <dt>In the run</dt>
        <dd>{node.inRun ? "Yes" : "No"}</dd>
        {node.fetchedAt && (
          <>
            <dt>Fetched</dt>
            <dd>{node.fetchedAt.slice(0, 16).replace("T", " ")}</dd>
          </>
        )}
      </dl>
      <h4>Links</h4>
      {rows.length === 0 ? (
        <p className="muted">No links.</p>
      ) : (
        <ul className="pie-graph-links">
          {rows.map(([words, keys]) => (
            <li key={words}>
              <span className="muted">{words === PARENT ? "parent of" : words}</span>
              <span>
                {keys.map((k) => (
                  <button key={k} type="button" className="pie-map-key mono" onClick={() => onPick(k)}>
                    {k}
                  </button>
                ))}
              </span>
            </li>
          ))}
        </ul>
      )}
      <div className="pie-graph-actions">
        <button type="button" className="btn btn-primary" disabled={busy || unloaded === 0} onClick={onExpand}>
          {unloaded > 0 ? `Expand links (${unloaded})` : "All links shown"}
        </button>
        <button type="button" className="btn" disabled={centred} onClick={onCentre}>
          Centre here
        </button>
        <button type="button" className="btn" disabled={!node.inRun} onClick={onTrace} title={node.inRun ? "" : "Only issues in the run open in Trace"}>
          Open in Trace
        </button>
        {canOpenInJira && (
          <button type="button" className="btn" onClick={onJira}>
            Open in Jira
          </button>
        )}
      </div>
    </aside>
  );
}
