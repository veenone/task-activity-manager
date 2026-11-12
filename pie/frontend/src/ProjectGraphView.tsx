import { lazy, Suspense, useMemo, useRef, useState } from "react";
import type { query } from "../wailsjs/go/models";
import type { CanvasApi } from "./G6Canvas";
import { back, countWidth, forward, type History, radialAround, visit } from "./graphModel";
import {
  busiest,
  type Edge,
  type LinkType,
  neighbours,
  projectOf,
} from "./linkData";
import { projectGraph } from "./projectGraph";
import { ProjectInput } from "./ProjectInput";

// G6 is loaded with the view, not with the app.
const G6Canvas = lazy(() => import("./G6Canvas"));

const short = (s: string) => (s.length > 20 ? `${s.slice(0, 19)}…` : s);
const plural = (n: number, one: string) => `${n} ${one}${n === 1 ? "" : "s"}`;

// ProjectGraphView is the Link map's network of projects (#473), in the same
// shell as the issue graph: options on the left, the projects in rings around
// a centre in the middle, the chosen project's links on the right.
export function ProjectGraphView({
  edges,
  issues,
  types,
  label,
  name,
  onIssues,
  onGrid,
}: {
  edges: Edge[];
  issues: Map<string, query.Issue>;
  types: LinkType[];
  label: (project: string) => string;
  name: (project: string) => string;
  onIssues: (project: string) => void;
  onGrid: (projects: string[]) => void;
}) {
  const first = useMemo(() => busiest(edges, issues), [edges, issues]);
  const [history, setHistory] = useState<History>(() => ({
    keys: [first],
    at: 0,
  }));
  const [selected, setSelected] = useState<string | null>(null);
  const [hidden, setHidden] = useState<Set<string>>(new Set());
  // The box keeps what is typed, so clearing it to type 2 does not make 12.
  const [minText, setMinText] = useState("1");
  const min = Math.max(1, Math.floor(Number(minText)) || 1);
  const [secondRing, setSecondRing] = useState(false);
  const canvas = useRef<CanvasApi>(null);
  const go = (p: string) => {
    setHistory((h) => visit(h, p));
    setSelected(null);
  };

  const projects = useMemo(
    () =>
      [
        ...new Set(
          edges.flatMap((e) => [
            projectOf(e.from, issues),
            projectOf(e.to, issues),
          ]),
        ),
      ].sort(),
    [edges, issues],
  );
  // A centre the links no longer reach, after a filter or a refresh, gives
  // way to the busiest project rather than standing alone.
  const visited = history.keys[history.at];
  const centre = projects.includes(visited) ? visited : first;
  // Links per type, inside one project left out, for the filter and legend.
  const counts = useMemo(() => {
    const c = new Map<string, number>();
    for (const e of edges)
      if (projectOf(e.from, issues) !== projectOf(e.to, issues))
        c.set(e.type, (c.get(e.type) ?? 0) + 1);
    return [...c].sort(([a], [b]) => a.localeCompare(b));
  }, [edges, issues]);
  const { nodes, lines } = useMemo(() => {
    const g = projectGraph(edges, issues, centre, { hidden, min, secondRing });
    const undirected = new Set(
      types.filter((t) => t.outward === t.inward).map((t) => t.name),
    );
    return {
      nodes: g.nodes.map((n) => ({
        id: n.project,
        label: `${n.project} · ${plural(n.issues, "issue")}`,
        sub: short(name(n.project)),
        dashed: n.ring === 2,
      })),
      lines: g.edges.map((e) => ({
        id: e.id,
        source: e.source,
        target: e.target,
        type: e.type,
        directed: !undirected.has(e.type),
        width: countWidth(e.count),
        label: String(e.count),
      })),
    };
  }, [edges, issues, centre, hidden, min, secondRing, types, name]);
  const layout = useMemo(() => radialAround(centre), [centre]);

  if (!centre)
    return <p className="muted">No links between projects to show.</p>;
  // A project a filter took off the canvas is no longer the one shown.
  const chosen =
    selected && nodes.some((n) => n.id === selected) ? selected : centre;
  const around = neighbours(chosen, edges, issues);
  const [out, inn] = around.reduce(
    ([o, i], n) => [
      o + n.types.reduce((a, s) => a + s.out, 0),
      i + n.types.reduce((a, s) => a + s.in, 0),
    ],
    [0, 0],
  );
  const inRun = [...issues.values()].filter((i) => i.project === chosen).length;

  return (
    <div className="pie-graph">
      <section className="pie-graph-side" aria-label="Project network options">
        <h4>Centre</h4>
        <ProjectInput
          label="Centre on a project"
          allText="Busiest project"
          projects={projects}
          projectLabel={label}
          value={centre}
          onChange={(p) => p && p !== centre && go(p)}
        />
        <h4>Relationships</h4>
        {counts.map(([type, n]) => (
          <label key={type} className="pie-check">
            <input
              type="checkbox"
              checked={!hidden.has(type)}
              onChange={(e) => {
                const next = new Set(hidden);
                if (e.target.checked) next.delete(type);
                else next.add(type);
                setHidden(next);
              }}
            />
            {type}
            <span className="muted">{n}</span>
          </label>
        ))}
        <h4>Show</h4>
        <label className="pie-check">
          <input
            type="checkbox"
            checked={secondRing}
            onChange={(e) => setSecondRing(e.target.checked)}
          />
          Two rings
        </label>
        <label className="pie-check">
          Hide lines with fewer links than
          <input
            type="number"
            className="detail-input pie-graph-min"
            min={1}
            value={minText}
            onChange={(e) => setMinText(e.target.value)}
          />
        </label>
        <p className="muted">
          A line&apos;s width and number count its links. The second ring is
          dashed.
        </p>
      </section>
      <div className="pie-graph-main">
        <div className="pie-filters">
          <button
            type="button"
            className="btn"
            disabled={history.at <= 0}
            onClick={() => (setHistory(back), setSelected(null))}
          >
            ‹ Back
          </button>
          <button
            type="button"
            className="btn"
            disabled={history.at >= history.keys.length - 1}
            onClick={() => (setHistory(forward), setSelected(null))}
          >
            Forward ›
          </button>
          <nav aria-label="Projects centred" className="pie-graph-crumbs">
            <span className="mono">
              {history.keys
                .slice(0, history.at + 1)
                .slice(-5)
                .join(" › ")}
            </span>
          </nav>
          <button
            type="button"
            className="btn"
            onClick={() => canvas.current?.fit()}
          >
            Fit
          </button>
          <button
            type="button"
            className="btn"
            onClick={() => canvas.current?.relayout()}
          >
            Re-layout
          </button>
          <span
            className="muted"
            role="status"
          >{`${plural(nodes.length, "project")}, ${plural(
            lines.reduce((a, l) => a + Number(l.label), 0),
            "link",
          )}`}</span>
        </div>
        <Suspense fallback={<p className="muted">Loading the graph...</p>}>
          <G6Canvas
            nodes={nodes}
            edges={lines}
            focus={chosen}
            layout={layout}
            onSelect={setSelected}
            label={`Projects linked to ${centre}`}
            apiRef={canvas}
          />
        </Suspense>
      </div>
      <aside className="pie-graph-details" aria-label={`Project ${chosen}`}>
        <span className="mono muted">{`${chosen} · Project`}</span>
        <h3>{name(chosen) || chosen}</h3>
        <dl className="pie-graph-facts">
          <dt>Issues in the run</dt>
          <dd>{inRun}</dd>
          <dt>Links out</dt>
          <dd>{out}</dd>
          <dt>Links in</dt>
          <dd>{inn}</dd>
        </dl>
        <h4>Linked projects</h4>
        <dl className="pie-graph-facts" aria-label="Linked projects">
          {around.map((n) => [
            <dt key={`${n.project}-t`} className="mono">
              {n.project}
            </dt>,
            <dd key={`${n.project}-d`}>
              {n.types
                .map((s) =>
                  [s.type, s.out && `${s.out} out`, s.in && `${s.in} in`]
                    .filter(Boolean)
                    .join(" "),
                )
                .join(" · ")}
            </dd>,
          ])}
        </dl>
        <div className="pie-graph-actions">
          <button
            type="button"
            className="btn btn-primary"
            disabled={chosen === centre}
            onClick={() => go(chosen)}
          >
            Centre here
          </button>
          <button
            type="button"
            className="btn"
            disabled={inRun === 0}
            title={inRun === 0 ? "None of its issues are in the run" : ""}
            onClick={() => onIssues(chosen)}
          >
            Show its issues
          </button>
          <button
            type="button"
            className="btn"
            onClick={() => onGrid([chosen, ...around.map((n) => n.project)])}
          >
            Open in Grid
          </button>
        </div>
      </aside>
    </div>
  );
}
