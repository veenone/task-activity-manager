import { useState } from "react";
import type { cases, query } from "../wailsjs/go/models";
import { CaseDetail } from "./CaseDetail";
import { CaseGrid as ReadGrid } from "../wailsjs/go/main/App";
import { CaseGrid } from "./CaseGrid";
import { ExportButton } from "./ExportButton";
import { caseSheet } from "./exportSheets";
import { projectLabel, projectNames } from "./linked";

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
  const [grid, setGrid] = useState(false);
  const issues = new Map(result?.issues.map((i) => [i.key, i]));
  const open = list.find((c) => c.id === openId);

  if (list.length === 0) {
    return (
      <p className="muted">
        No cases yet. Tick issues in Results and choose Open as cases.
      </p>
    );
  }
  const modes = (
    <div className="pie-filters" role="group" aria-label="Show cases as">
      {[
        { label: "List", on: !grid },
        { label: "Grid", on: grid },
      ].map((m) => (
        <button
          key={m.label}
          type="button"
          className={`btn${m.on ? " btn-primary" : ""}`}
          aria-pressed={m.on}
          onClick={() => setGrid(m.label === "Grid")}
        >
          {m.label}
        </button>
      ))}
      <ExportButton
        name="pie-cases"
        sheets={async () => {
          const names = result ? projectNames(result) : new Map<string, string>();
          return [caseSheet(await ReadGrid(profileId), (p) => projectLabel(p, names.get(p)))];
        }}
      />
    </div>
  );
  if (grid) {
    return (
      <>
        {modes}
        <CaseGrid
          profileId={profileId}
          result={result}
          onOpen={(id) => {
            setOpenId(id);
            setGrid(false);
          }}
        />
      </>
    );
  }
  return (
    <>
      {modes}
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
                  <span className="muted">
                    {issues.get(c.key)?.summary ?? "Not in the run on screen"}
                  </span>
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
    </>
  );
}
