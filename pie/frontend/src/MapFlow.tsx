import type { query } from "../wailsjs/go/models";
import { FIX_STATES, FLOW_NODE, type FixState, fixState, sankey } from "./mapData";
import { projectLabel } from "./linked";
import { Zoom } from "./Zoom";

type Group = { baseline: string | null; projects: string[] };

const WIDTH = 860;
const HEIGHT = 360;
const LEFT = 240;
const RIGHT = 180;

// MapFlow runs every issue in a linked set from its baseline to its project to
// its fix state, one band per pair sized by issue count. The sets out of step
// are listed beside it, because a band is hard to aim at and has no keys.
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
  const label = (p: string) => projectLabel(p, names.get(p));
  const baseOf = new Map(groups.flatMap((g) => g.projects.map((p) => [p, `b:${g.baseline ?? ""}`] as const)));
  const pairs = new Map<string, { from: string; to: string; keys: string[] }>();
  const add = (from: string, to: string, key: string) => {
    const pair = pairs.get(`${from}|${to}`) ?? { from, to, keys: [] };
    if (!pair.keys.includes(key)) pair.keys.push(key);
    pairs.set(`${from}|${to}`, pair);
  };
  const states = new Map<FixState, number>();
  for (const s of sets) {
    for (const k of s.keys) {
      const project = issues.get(k)?.project ?? "";
      const state = fixState(s, issues.get(k));
      states.set(state, (states.get(state) ?? 0) + 1);
      add(baseOf.get(project) ?? "b:", `p:${project}`, k);
      add(`p:${project}`, `s:${state}`, k);
    }
  }
  const columns = [
    groups.map((g) => ({
      id: `b:${g.baseline ?? ""}`,
      label: g.baseline === null ? "No baseline" : `Baseline ${label(g.baseline)}`,
    })),
    groups.flatMap((g) => g.projects.map((p) => ({ id: `p:${p}`, label: label(p) }))),
    FIX_STATES.filter((f) => states.has(f.id)).map((f) => ({
      id: `s:${f.id}`,
      label: f.label,
    })),
  ];
  const name = new Map(columns.flat().map((c) => [c.id, c.label]));
  const { nodes, bands, height } = sankey(columns, [...pairs.values()], {
    width: WIDTH - LEFT - RIGHT,
    height: HEIGHT,
    gap: 14,
  });
  const tone = (id: string) => (id.startsWith("s:") ? `pie-flow-${id.slice(2)}` : "pie-flow-plain");
  const behind = sets.filter((s) => s.outOfStep);

  return (
    <div className="pie-flow">
      <Zoom label="Project flow">
        <svg
          className="pie-flow-svg"
          viewBox={`${-LEFT} -10 ${WIDTH} ${height + 20}`}
          role="img"
          aria-label="Issues by baseline, project and fix state. The table view lists the same sets."
        >
          {bands.map((b) => (
            <path key={`${b.from}|${b.to}`} className={`pie-flow-band ${tone(b.to)}`} d={b.path}>
              <title>{`${name.get(b.from)} to ${name.get(b.to)}: ${b.keys.length} (${b.keys.join(", ")})`}</title>
            </path>
          ))}
          {nodes.map((n) => (
            <g key={n.id}>
              <rect className={`pie-flow-node ${tone(n.id)}`} x={n.x} y={n.y} width={FLOW_NODE} height={Math.max(n.h, 2)} rx={2} />
              <text
                className="pie-flow-label"
                x={n.id.startsWith("b:") ? n.x - 6 : n.x + FLOW_NODE + 6}
                y={n.y + n.h / 2}
                textAnchor={n.id.startsWith("b:") ? "end" : "start"}
              >
                {n.id.startsWith("s:") ? `${n.label} ${n.value}` : n.label}
              </text>
            </g>
          ))}
        </svg>
      </Zoom>
      <section className="pie-flow-side">
        <h3>Sets out of step</h3>
        <ul aria-label="Sets out of step">
          {behind.map((s) => (
            <li key={s.keys[0]}>
              {s.keys.map((k) => (
                <button key={k} type="button" className="pie-map-key" onClick={() => onOpen(k)}>
                  <span className="mono">{k}</span> {issues.get(k)?.status}
                </button>
              ))}
            </li>
          ))}
        </ul>
        {behind.length === 0 && <p className="muted">Every set is in step.</p>}
      </section>
    </div>
  );
}
