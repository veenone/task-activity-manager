import { useEffect, useImperativeHandle, useRef, useState, type Ref } from "react";
import { EdgeEvent, Graph, NodeEvent, type EdgeData, type GraphData, type IElementEvent, type LayoutOptions } from "@antv/g6";

// G6Canvas is the one place the suite talks to AntV G6 (#464, #465): it draws
// the nodes, edges and groups it is given and reports a click. It moved here
// from PIE so TAM's Dependencies view draws through the same wrapper (#612).
// What to draw lives in plain modules (PIE's graphModel, TAM's
// dependencyGraph) tested without a canvas; jsdom has none, so this stays
// thin. Apps import it lazily by path rather than from the core index, so G6
// stays out of their first chunk.

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
// width and label, when given, size a line by what it counts and say the count.
export type CanvasEdge = {
  id: string;
  source: string;
  target: string;
  type: string;
  directed: boolean;
  faded?: boolean;
  lit?: boolean;
  width?: number;
  label?: string;
  // dashed draws a directed edge dashed, such as a link still pending a Commit.
  dashed?: boolean;
};
export type CanvasCombo = { id: string; label: string; parent?: string; tone: "baseline" | "derived" | "external" };
export type CanvasApi = { fit: () => void; relayout: () => void };
// CanvasLayout is a G6 layout, named here so an app need not import G6 itself.
export type CanvasLayout = LayoutOptions;

// css reads a theme token when the graph draws, so it follows light and dark.
const css = (name: string) => getComputedStyle(document.documentElement).getPropertyValue(name).trim() || "#888";

const statusColour = (s?: string) => css(s === "done" ? "--ok-text" : s === "indeterminate" ? "--info-solid" : "--text-muted");

// edgeColour gives the common link types their own token and any other type
// a stable one from the rest, so a custom type keeps its colour across runs.
const OTHERS = ["--chart-added", "--ok-text", "--info-fg", "--chart-committed"];
export function edgeColour(type: string): string {
  const t = type.toLowerCase();
  // A flow line with no type is plain.
  if (!t) return css("--border-strong");
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
        lineWidth: e.width ?? (e.lit ? 3 : e.type.toLowerCase().includes("parent") ? 2.5 : 1.5),
        labelText: e.label,
        labelFill: css("--text"),
        labelFontSize: 11,
        labelBackground: !!e.label,
        labelBackgroundFill: css("--surface"),
        labelBackgroundRadius: 4,
        labelPadding: [1, 4],
        // An empty dash, not undefined, so an edge that stops being pending
        // turns solid again: a redraw merges styles and keeps an old dash.
        lineDash: e.directed && !e.dashed ? [] : [5, 4],
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

// graphImage draws nodes and edges on a graph of its own, off screen, and
// answers with a PNG data URL of all of it. An export wants every node, not
// the part of the canvas the user panned to, and must not move their view
// (#612). The background is the surface, so the theme's text stays legible.
export async function graphImage(nodes: CanvasNode[], edges: CanvasEdge[], focus: string | null, layout: LayoutOptions): Promise<string> {
  const box = document.createElement("div");
  box.style.cssText = "position:fixed;left:-20000px;top:0;width:1600px;height:1000px";
  document.body.appendChild(box);
  const g = new Graph({
    container: box,
    animation: false,
    background: css("--surface"),
    node: { type: "rect" },
    layout: still(layout),
    transforms: ["process-parallel-edges"],
  });
  try {
    g.setData(data(nodes, edges, NO_COMBOS, focus));
    await g.render();
    return await g.toDataURL({ mode: "overall", type: "image/png" });
  } finally {
    g.destroy();
    box.remove();
  }
}

export default function G6Canvas({
  nodes,
  edges,
  combos = NO_COMBOS,
  focus,
  layout = COMBINED,
  follow = true,
  onSelect,
  onSelectEdge,
  onConnect,
  label,
  apiRef,
}: {
  nodes: CanvasNode[];
  edges: CanvasEdge[];
  combos?: CanvasCombo[];
  focus: string | null;
  layout?: LayoutOptions;
  // follow moves the camera to the focus; a flow keeps its layers in view.
  follow?: boolean;
  onSelect: (id: string) => void;
  // onSelectEdge reports a click on an edge by its id.
  onSelectEdge?: (id: string) => void;
  // onConnect turns dragging a node into drawing a link to another node,
  // reported as the pair; it replaces dragging the boxes around (#612).
  onConnect?: (source: string, target: string) => void;
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
  const pickEdge = useRef(onSelectEdge);
  pickEdge.current = onSelectEdge;
  const connect = useRef(onConnect);
  connect.current = onConnect;
  // Whether the graph made once links by drag, fixed like its first layout.
  const connectable = useRef(!!onConnect);
  // The colours come from tokens read at draw time, so a theme switch has to
  // draw again; the boxes stay where they are.
  const [theme, setTheme] = useState(() => document.documentElement.dataset.theme);
  useEffect(() => {
    const root = document.documentElement;
    const watch = new MutationObserver(() => setTheme(root.dataset.theme));
    watch.observe(root, { attributes: true, attributeFilter: ["data-theme"] });
    return () => watch.disconnect();
  }, []);

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
      behaviors: [
        "drag-canvas",
        "zoom-canvas",
        connectable.current
          ? {
              type: "create-edge",
              trigger: "drag",
              style: { stroke: css("--accent"), lineDash: [5, 4] },
              // The view journals the link and redraws; returning nothing
              // keeps G6 from adding an edge of its own.
              onCreate: (edge: EdgeData) => {
                connect.current?.(String(edge.source), String(edge.target));
                return undefined;
              },
            }
          : "drag-element",
      ],
      // Two link types between the same pair bend apart instead of overlapping.
      transforms: ["process-parallel-edges"],
      plugins: [{ type: "minimap", key: "minimap", size: [150, 96], position: "left-bottom" }],
    });
    g.on(NodeEvent.CLICK, (e: IElementEvent) => pick.current(e.target.id));
    g.on(EdgeEvent.CLICK, (e: IElementEvent) => pickEdge.current?.(e.target.id));
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
          // zoomTo does not keep the graph centred, so centre it again.
          .then(() => (live() && g.getZoom() > 1 ? g.zoomTo(1).then(() => g.fitCenter()) : undefined))
      : g.draw();
    drawn
      .then(() => {
        if (!live()) return;
        setFailed("");
        // After a new layout the fit shows everything; moving to the focus
        // then pushed the rest of a grown graph off the canvas (#473).
        if (follow && !moved && focus && g.hasNode(focus)) return g.focusElement(focus, { duration: 500, easing: "ease-in-out" });
      })
      // A render still running when its graph is destroyed fails; only a
      // failure of the live graph is worth showing.
      .catch((e: unknown) => {
        if (graph.current === g) setFailed(e instanceof Error ? e.message : String(e));
      });
  }, [nodes, edges, combos, focus, layout, follow, theme]);

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
      <div ref={box} className="g6-canvas" role="img" aria-label={label} />
      <div className="g6-zoom-controls" role="group" aria-label="Graph zoom">
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
