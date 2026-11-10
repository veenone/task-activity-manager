import { lazy, Suspense, useMemo, useRef, useState } from "react";
import type { cases, query } from "../wailsjs/go/models";
import { baselineEdges, fold, health, insideIssues, members, versionsOf, walk } from "./baselineData";
import { BaselineHealth } from "./BaselineHealth";
import { projectLabel, projectNames } from "./linked";
import type { CanvasApi, CanvasCombo, CanvasEdge, CanvasNode } from "./G6Canvas";

const G6Canvas = lazy(() => import("./G6Canvas"));

type Scope = "inside" | "crossing" | "both";
type Analysis = "blockers" | "impact" | "none";

// BaselineView puts one baseline in the middle (#465): its issues inside a
// boundary, each derived project nested in it, and the linked issues of other
// projects outside, grouped by project. A chosen issue's blockers or impact
// are traced along blocks links; the panel on the right counts the
// baseline's health for a version and compares it with another.
export function BaselineView({
  result,
  baselines,
  onOpen,
}: {
  result: query.Result;
  baselines: cases.Baseline[];
  onOpen: (key: string) => void;
}) {
  const [chosen, setChosen] = useState("");
  const [version, setVersion] = useState("");
  const [compareWith, setCompareWith] = useState("");
  const [scope, setScope] = useState<Scope>("both");
  const [hidden, setHidden] = useState<Set<string>>(new Set());
  const [analysis, setAnalysis] = useState<Analysis>("blockers");
  const [onlyLinkedOut, setOnlyLinkedOut] = useState(true);
  const [finished, setFinished] = useState(false);
  const [opened, setOpened] = useState<Set<string>>(new Set());
  const [selected, setSelected] = useState<string | null>(null);
  const canvas = useRef<CanvasApi>(null);

  const base = baselines.find((b) => b.key === chosen) ?? baselines[0];
  const run = useMemo(() => new Map(result.issues.map((i) => [i.key, i])), [result]);
  const label = useMemo(() => {
    const names = projectNames(result);
    return (p: string) => projectLabel(p, names.get(p));
  }, [result]);
  const projects = useMemo(() => (base ? members(base) : new Set<string>()), [base]);
  const versions = useMemo(() => versionsOf(result.issues, projects), [result, projects]);
  const v = versions.includes(version) ? version : "";
  const then = versions.includes(compareWith) && compareWith !== v ? compareWith : "";

  const view = useMemo(() => {
    const all = insideIssues(result.issues, projects, v);
    const edges = baselineEdges(all, run);
    const inside = all.filter((i) => finished || i.statusCategory !== "done" || i.key === selected);
    const traced = selected && analysis !== "none" ? walk(selected, edges, analysis) : null;
    const lit = new Set(traced ? [selected!, ...traced.keys] : []);
    const litEdges = new Set(traced?.edges.map((e) => e.id) ?? []);
    const { shown, hidden: folded } = onlyLinkedOut ? fold(inside, edges, lit, opened) : { shown: inside, hidden: new Map<string, number>() };
    const inKeys = new Set(shown.map((i) => i.key));
    const drawnEdges = edges.filter(
      (e) => !hidden.has(e.type) && (scope === "both" || (scope === "crossing") === e.crossing),
    );
    // Outside issues are drawn only where a drawn edge reaches them.
    const outside = new Set<string>();
    const keep = drawnEdges.filter((e) => {
      const ends = [e.source, e.target];
      if (!ends.every((k) => inKeys.has(k) || !projects.has(run.get(k)?.project ?? ""))) return false;
      ends.filter((k) => !inKeys.has(k)).forEach((k) => outside.add(k));
      return true;
    });
    const comboOf = (p: string) => (p === base?.key ? "base" : projects.has(p) ? `d:${p}` : `x:${p}`);
    const node = (is: query.Issue): CanvasNode => ({
      id: is.key,
      label: is.key,
      sub: is.summary.length > 22 ? `${is.summary.slice(0, 21)}…` : is.summary,
      status: is.statusCategory,
      combo: comboOf(is.project),
      lit: lit.has(is.key) && is.key !== selected,
      faded: lit.size > 0 && !lit.has(is.key),
    });
    const nodes: CanvasNode[] = [
      ...shown.map(node),
      ...[...outside].map((k) => node(run.get(k)!)),
      ...[...folded].map(([p, n]) => ({ id: `more:${p}`, label: `+ ${n} more`, combo: comboOf(p), cluster: true })),
    ];
    const canvasEdges: CanvasEdge[] = keep.map((e) => ({
      id: e.id,
      source: e.source,
      target: e.target,
      type: e.type,
      directed: e.directed,
      lit: litEdges.has(e.id),
      faded: lit.size > 0 && !litEdges.has(e.id),
    }));
    const external = [...new Set([...outside].map((k) => run.get(k)!.project))];
    const combos: CanvasCombo[] = base
      ? [
          { id: "base", label: `Baseline ${label(base.key)}${v ? ` · ${v}` : ""}`, tone: "baseline" },
          ...(base.descendants ?? []).map((p) => ({ id: `d:${p}`, label: `Derived: ${label(p)}`, parent: "base", tone: "derived" as const })),
          ...external.map((p) => ({ id: `x:${p}`, label: label(p), tone: "external" as const })),
        ]
      : [];
    return { all, edges, nodes, canvasEdges, combos, traced, inside: shown.length, outside: outside.size };
  }, [result, run, projects, base, v, finished, selected, analysis, onlyLinkedOut, opened, hidden, scope, label]);

  if (!base) return <p className="muted">Mark a project as a baseline under Baselines to see it here.</p>;
  const now = Date.now();
  const types = [...new Set(view.edges.map((e) => e.type))].sort();
  const thenHealth = then ? health(insideIssues(result.issues, projects, then), baselineEdges(insideIssues(result.issues, projects, then), run), run, result.sets, now) : null;

  return (
    <div className="pie-graph">
      <section className="pie-graph-side" aria-label="Baseline options">
        <h4>Baseline</h4>
        <select className="detail-input" aria-label="Baseline" value={base.key} onChange={(e) => setChosen(e.target.value)}>
          {baselines.map((b) => (
            <option key={b.key} value={b.key}>
              {label(b.key)}
            </option>
          ))}
        </select>
        <select className="detail-input" aria-label="Version" value={v} onChange={(e) => setVersion(e.target.value)}>
          <option value="">All versions</option>
          {versions.map((x) => (
            <option key={x}>{x}</option>
          ))}
        </select>
        <select className="detail-input" aria-label="Compared with" value={then} onChange={(e) => setCompareWith(e.target.value)}>
          <option value="">Compare with no version</option>
          {versions
            .filter((x) => x !== v)
            .map((x) => (
              <option key={x}>{x}</option>
            ))}
        </select>
        {versions.length === 0 && <p className="muted">No Fix Versions in this run. Run the query again to read them.</p>}
        <h4>Relationships</h4>
        {(
          [
            ["inside", "Inside the baseline"],
            ["crossing", "Crossing its boundary"],
            ["both", "Both"],
          ] as const
        ).map(([id, text]) => (
          <label key={id} className="pie-check">
            <input type="radio" name="pie-baseline-scope" checked={scope === id} onChange={() => setScope(id)} />
            {text}
          </label>
        ))}
        {types.map((t) => (
          <label key={t} className="pie-check">
            <input
              type="checkbox"
              checked={!hidden.has(t)}
              onChange={(e) => {
                const next = new Set(hidden);
                if (e.target.checked) next.delete(t);
                else next.add(t);
                setHidden(next);
              }}
            />
            {t}
          </label>
        ))}
        <h4>Analysis</h4>
        {(
          [
            ["blockers", "Blocker chain"],
            ["impact", "Impact"],
            ["none", "None"],
          ] as const
        ).map(([id, text]) => (
          <label key={id} className="pie-check">
            <input type="radio" name="pie-baseline-analysis" checked={analysis === id} onChange={() => setAnalysis(id)} />
            {text}
          </label>
        ))}
        <p className="muted">Choose an issue to trace what blocks it, or what it holds up.</p>
        <h4>Inside the boundary</h4>
        <label className="pie-check">
          <input type="checkbox" checked={onlyLinkedOut} onChange={(e) => setOnlyLinkedOut(e.target.checked)} />
          Only issues with links out
        </label>
        <label className="pie-check">
          <input type="checkbox" checked={finished} onChange={(e) => setFinished(e.target.checked)} />
          Finished issues
        </label>
      </section>
      <div className="pie-graph-main">
        <div className="pie-filters">
          <span className="muted" role="status">
            {`${view.inside} of ${view.all.length} issues inside drawn, ${view.outside} outside`}
          </span>
          <button type="button" className="btn" onClick={() => canvas.current?.fit()}>
            Fit
          </button>
          {selected && (
            <button type="button" className="btn btn-ghost" onClick={() => setSelected(null)}>
              Clear the trace
            </button>
          )}
        </div>
        <Suspense fallback={<p className="muted">Loading the graph...</p>}>
          <G6Canvas
            nodes={view.nodes}
            edges={view.canvasEdges}
            combos={view.combos}
            focus={selected}
            onSelect={(id) => {
              if (id.startsWith("more:")) setOpened(new Set([...opened, id.slice(5)]));
              else setSelected(id);
            }}
            label={`Baseline ${label(base.key)} and the issues linked to it`}
            apiRef={canvas}
          />
        </Suspense>
      </div>
      <BaselineHealth
        version={v}
        health={health(view.all, view.edges, run, result.sets, now)}
        compareWith={then}
        then={thenHealth}
        walkTitle={analysis === "impact" ? `What ${selected} holds up` : `What blocks ${selected}`}
        steps={view.traced?.edges ?? []}
        selected={selected}
        onTrace={() => selected && onOpen(selected)}
      />
    </div>
  );
}
