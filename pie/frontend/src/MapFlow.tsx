import { useCallback, useMemo } from "react";
import type { query } from "../wailsjs/go/models";
import type { CanvasNode } from "./G6Canvas";
import { FIX_STATES, type FixState, fixState } from "./mapData";
import { LayeredFlow } from "./LayeredFlow";
import { projectLabel } from "./linked";

type Group = { baseline: string | null; projects: string[] };

const LAYERS = ["Baseline", "Project", "Fix state"];
// A fix state's box takes the colour of where its issues stand.
const MARKS: Partial<Record<string, Partial<CanvasNode>>> = {
  "s:missing": { lit: true },
  "s:behind": { status: "indeterminate" },
  "s:everywhere": { status: "done" },
};
const mark = (id: string) => MARKS[id] ?? {};

// MapFlow runs every issue in a linked set from its baseline to its project to
// its fix state (#473). The panel beside it gives the fix states of the box
// chosen, or of everything, and the issues still missing a fix.
export function MapFlow({
  sets,
  issues,
  groups,
  names,
  onOpen,
}: {
  sets: query.Set[];
  issues: Map<string, query.Issue>;
  groups: Group[];
  names: Map<string, string>;
  onOpen: (key: string) => void;
}) {
  const { paths, states } = useMemo(() => {
    const baseOf = new Map(groups.flatMap((g) => g.projects.map((p) => [p, `b:${g.baseline ?? ""}`] as const)));
    const states = new Map<string, FixState>();
    const paths = sets.flatMap((s) =>
      s.keys.map((key) => {
        const project = issues.get(key)?.project ?? "";
        const state = fixState(s, issues.get(key));
        states.set(key, state);
        return { key, steps: [baseOf.get(project) ?? "b:", `p:${project}`, `s:${state}`] };
      }),
    );
    return { paths, states };
  }, [sets, issues, groups]);
  const name = useCallback(
    (id: string) => {
      const rest = id.slice(2);
      if (id.startsWith("p:")) return projectLabel(rest, names.get(rest));
      if (id.startsWith("s:")) return FIX_STATES.find((f) => f.id === rest)?.label ?? rest;
      return rest === "" ? "No baseline" : `Baseline ${projectLabel(rest, names.get(rest))}`;
    },
    [names],
  );

  return (
    <LayeredFlow
      paths={paths}
      layers={LAYERS}
      name={name}
      noun="issue"
      label="Issues by baseline, project and fix state"
      mark={mark}
      details={(keys) => {
        const missing = keys.filter((k) => states.get(k) === "missing");
        return (
          <>
            <h4>Fix states</h4>
            <ul className="pie-graph-found" aria-label="Fix states">
              {FIX_STATES.map((f) => [f.label, keys.filter((k) => states.get(k) === f.id).length] as const)
                .filter(([, n]) => n > 0)
                .map(([l, n]) => (
                  <li key={l}>{`${l} ${n}`}</li>
                ))}
            </ul>
            <h4>Missing a fix</h4>
            {missing.length === 0 ? (
              <p className="muted">None.</p>
            ) : (
              <ul className="pie-graph-found" aria-label="Missing a fix">
                {missing.map((k) => (
                  <li key={k}>
                    <button type="button" className="pie-map-key" onClick={() => onOpen(k)}>
                      <span className="mono">{k}</span> {issues.get(k)?.status}
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </>
        );
      }}
    />
  );
}
