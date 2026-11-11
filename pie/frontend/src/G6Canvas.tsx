import { useEffect, useImperativeHandle, useRef, useState, type Ref } from "react";
import { Graph, NodeEvent, type GraphData, type IElementEvent, type LayoutOptions } from "@antv/g6";

// G6Canvas is the one place PIE talks to AntV G6 (#464, #465): it draws the
// nodes, edges and groups it is given and reports a click. Everything about
// what to draw lives in plain modules (graphModel, baselineData) that are
// tested without a canvas; jsdom has none, so this stays thin.

export type CanvasNode = {
  id: string;
  label: string;
  sub?: string;
  status?: string;
  dashed?: boolean;
  badge?: string;
  combo?: string;
  faded?: boolean;
  lit?: boolean;
  cluster?: boolean;
};
export type CanvasEdge = { id: string; source: string; target: string; type: string; directed: boolean; faded?: boolean; lit?: boolean };
export type CanvasCombo = { id: string; label: string; parent?: string; tone: "baseline" | "derived" | "external" };
export type CanvasApi = { fit: () => void; relayout: () => void };

// css reads a theme token when the graph draws, so it follows light and dark.
const css = (name: string) => getComputedStyle(document.documentElement).getPropertyValue(name).trim() || "#888";

const statusColour = (s?: string) => css(s === "done" ? "--ok-text" : s === "indeterminate" ? "--info-solid" : "--text-muted");

// edgeColour gives the common link types their own token and any other type
// a stable one from the rest, so a custom type keeps its colour across runs.
const OTHERS = ["--chart-added", "--ok-text", "--info-fg", "--chart-committed"];
export function edgeColour(type: string): string {
  const t = type.toLowerCase();
  if (t.includes("block")) return css("--danger-solid");
  if (t.includes("clon")) return css("--chart-carried");
  if (t.includes("duplic")) return css("--warn-fg");
  if (t.includes("relat")) return css("--text-muted");
  if (t.includes("parent")) return css("--text");
  let h = 0;
  for (const c of t) h = (h * 31 + c.charCodeAt(0)) >>> 0;
  return css(OTHERS[h % OTHERS.length]);
}

function data(nodes: CanvasNode[], edges: CanvasEdge[], combos: CanvasCombo[], focus: string | null): GraphData {
  return {
    nodes: nodes.map((n) => ({
      id: n.id,
      combo: n.combo,
      style: {
        size: (n.cluster ? [150, 30] : [132, 44]) as [number, number],
        radius: n.cluster ? 15 : 8,
        fill: css("--surface"),
        stroke: n.id === focus ? css("--accent") : n.lit ? css("--danger-solid") : statusColour(n.status),
        lineWidth: n.id === focus ? 3.5 : n.lit ? 2.5 : 1.5,
        lineDash: n.dashed ? [4, 3] : undefined,
        opacity: n.faded ? 0.4 : 1,
        labelText: n.sub ? `${n.label}\n${n.sub}` : n.label,
        labelPlacement: "center" as const,
        labelFill: css("--text"),
        labelFontSize: 12,
        labelWordWrap: true,
        labelMaxWidth: 124,
        labelMaxLines: 2,
        badge: !!n.badge,
        badges: n.badge ? [{ text: n.badge, placement: "right-top" as const }] : [],
        cursor: "pointer",
      },
    })),
    edges: edges.map((e) => ({
      id: e.id,
      source: e.source,
      target: e.target,
      style: {
        stroke: edgeColour(e.type),
        lineWidth: e.lit ? 3 : e.type.toLowerCase().includes("parent") ? 2.5 : 1.5,
        lineDash: e.directed ? undefined : [5, 4],
        endArrow: e.directed,
        opacity: e.faded ? 0.25 : 1,
      },
    })),
    combos: combos.map((c) => ({
      id: c.id,
      combo: c.parent,
      style: {
        labelText: c.label,
        labelPlacement: "top" as const,
        labelFill: css(c.tone === "external" ? "--text-muted" : "--accent"),
        fill: css(c.tone === "external" ? "--surface" : "--surface-2"),
        stroke: css(c.tone === "external" ? "--border-strong" : "--accent"),
        lineDash: c.tone === "baseline" ? [7, 5] : undefined,
        radius: 14,
        padding: 18,
      },
    })),
  };
}

// NO_COMBOS is one empty list, so a view without groups does not hand the
// canvas a new array, and a redraw, on every render.
const NO_COMBOS: CanvasCombo[] = [];
// still runs a layout without its animation: render() resolves before an
// animated layout ends, so a fit after it measured the old positions (#469).
const still = (l: LayoutOptions): LayoutOptions => ({ ...l, animation: false }) as LayoutOptions;
// COMBINED lays out groups and the nodes in them; a view without groups can
// pass its own layout, such as a radial one around a centre.
const COMBINED: LayoutOptions = { type: "combo-combined", spacing: 24, comboPadding: 24 };

export default function G6Canvas({
  nodes,
  edges,
  combos = NO_COMBOS,
  focus,
  layout = COMBINED,
  onSelect,
  label,
  apiRef,
}: {
  nodes: CanvasNode[];
  edges: CanvasEdge[];
  combos?: CanvasCombo[];
  focus: string | null;
  layout?: LayoutOptions;
  onSelect: (id: string) => void;
  label: string;
  apiRef?: Ref<CanvasApi>;
}) {
  const box = useRef<HTMLDivElement>(null);
  const graph = useRef<Graph | null>(null);
  // What the graph was last laid out from: the issues, links and groups
  // drawn, and the layout. An option that changes them lays it out again
  // (#469). A change of style only, such as the focus or a faded trace, is
  // redrawn in place, so the boxes stay where the analyst panned to.
  const laidOut = useRef<{ shape: string; layout: LayoutOptions } | null>(null);
  // Each drawing's number: a slower earlier one must not fit or focus over it.
  const drawing = useRef(0);
  const [failed, setFailed] = useState("");
  const firstLayout = useRef(layout);
  // The latest handler, so the graph made once still calls the current one.
  const pick = useRef(onSelect);
  pick.current = onSelect;

  useEffect(() => {
    if (!box.current) return;
    // A new graph has laid nothing out yet. StrictMode makes a second one on
    // mount, and it drew without a layout, every node at 0,0, while this ref
    // still held the first one's (#469).
    laidOut.current = null;
    const g = new Graph({
      container: box.current,
      animation: true,
      node: { type: "rect" },
      combo: { type: "rect" },
      layout: still(firstLayout.current),
      behaviors: ["drag-canvas", "zoom-canvas", "drag-element"],
      plugins: [{ type: "minimap", key: "minimap", size: [150, 96], position: "left-bottom" }],
    });
    g.on(NodeEvent.CLICK, (e: IElementEvent) => pick.current(e.target.id));
    graph.current = g;
    return () => {
      graph.current = null;
      g.destroy();
    };
  }, []);

  useEffect(() => {
    const g = graph.current;
    if (!g) return;
    g.setData(data(nodes, edges, combos, focus));
    const shape = [
      ...nodes.map((n) => `${n.id}@${n.combo ?? ""}`),
      ...edges.map((e) => e.id),
      ...combos.map((c) => `#${c.id}<${c.parent ?? ""}`),
    ].join(",");
    const before = laidOut.current;
    const moved = !before || before.shape !== shape || before.layout !== layout;
    laidOut.current = { shape, layout };
    if (before && before.layout !== layout) g.setLayout(still(layout));
    const mine = ++drawing.current;
    const live = () => graph.current === g && drawing.current === mine;
    // Fit shrinks a graph that overflows either way and centres it, but never
    // enlarges a small one past its real size. G6's own "overflow" fit waits
    // for the graph to overflow both ways.
    const drawn = moved
      ? g
          .render()
          .then(() => (live() ? g.fitView() : undefined))
          .then(() => (live() && g.getZoom() > 1 ? g.zoomTo(1) : undefined))
      : g.draw();
    drawn
      .then(() => {
        if (!live()) return;
        setFailed("");
        if (focus && g.hasNode(focus)) return g.focusElement(focus, { duration: 500, easing: "ease-in-out" });
      })
      // A render still running when its graph is destroyed fails; only a
      // failure of the live graph is worth showing.
      .catch((e: unknown) => {
        if (graph.current === g) setFailed(e instanceof Error ? e.message : String(e));
      });
  }, [nodes, edges, combos, focus, layout]);

  useImperativeHandle(apiRef, () => ({
    fit: () => void graph.current?.fitView({ when: "always" }, { duration: 400 }),
    relayout: () => void graph.current?.layout(),
  }));

  const zoom = (by: number) => void graph.current?.zoomBy(by, { duration: 200 });
  return (
    <>
      {failed && (
        <p role="alert" className="error-text">
          {`The graph could not be drawn: ${failed}`}
        </p>
      )}
      <div ref={box} className="pie-g6" role="img" aria-label={label} />
      <div className="pie-zoom-controls" role="group" aria-label="Graph zoom">
        <button type="button" className="btn btn-ghost" aria-label="Zoom in" onClick={() => zoom(1.25)}>
          +
        </button>
        <button type="button" className="btn btn-ghost" aria-label="Zoom out" onClick={() => zoom(0.8)}>
          −
        </button>
      </div>
    </>
  );
}
