import { useState } from "react";
import type { cases, query } from "../wailsjs/go/models";
import { CaseDetail } from "./CaseDetail";

// CasesPage lists the profile's cases and opens one. It works with no run on
// screen; a run, when there is one, supplies each case's summary.
export function CasesPage({
  profileId,
  cases: list,
  result,
  onChanged,
}: {
  profileId: string;
  cases: cases.Case[];
  result: query.Result | null;
  onChanged: () => void;
}) {
  const [openId, setOpenId] = useState<number | null>(null);
  const issues = new Map(result?.issues.map((i) => [i.key, i]));
  const open = list.find((c) => c.id === openId);

  if (list.length === 0) {
    return <p className="muted">No cases yet. Tick issues in Results and choose Open as cases.</p>;
  }
  return (
    <div className="pie-linked pie-trace">
      <div className="pie-trace-side">
        <ul className="pie-set-list" aria-label="Cases">
          {list.map((c) => (
            <li key={c.id}>
              <button
                type="button"
                className={`pie-set${c.id === openId ? " pie-set-open" : ""}`}
                aria-current={c.id === openId ? "true" : undefined}
                onClick={() => setOpenId(c.id)}
              >
                <span className="mono">{c.key}</span>
                <span className="muted">{issues.get(c.key)?.summary ?? "Not in the run on screen"}</span>
                <span className="muted">{`${c.assessed} assessed, ${c.affected} affected`}</span>
              </button>
            </li>
          ))}
        </ul>
      </div>
      {open && (
        <CaseDetail
          key={open.id}
          profileId={profileId}
          id={open.id}
          issue={issues.get(open.key)}
          result={result}
          onChanged={onChanged}
        />
      )}
    </div>
  );
}
