import {
  lazy,
  type ReactNode,
  Suspense,
  useMemo,
  useRef,
  useState,
} from "react";
import type { LayoutOptions } from "@antv/g6";
import type { CanvasApi, CanvasNode } from "./G6Canvas";
import { countWidth } from "./graphModel";
import { type FlowPath, layered } from "./flowLayers";

// G6 is loaded with the view, not with the app.
const G6Canvas = lazy(() => import("./G6Canvas"));

// Layers run left to right, a box's paths into and out of it straight across.
const LAYERS: LayoutOptions = {
  type: "antv-dagre",
  rankdir: "LR",
  nodesep: 14,
  ranksep: 140,
};
const plural = (n: number, one: string) => `${n} ${one}${n === 1 ? "" : "s"}`;

// LayeredFlow draws paths through layers of boxes with G6 (#473), in the
// same shell as the networks: options left, the flow in the middle, the
// chosen box on the right. A line between two boxes stands for the keys that
// pass both, its width and number giving the count. Choosing a box lights
// every path through it and fades the rest.
export function LayeredFlow({
  paths,
  layers,
  name,
  noun,
  label,
  mark,
  options,
  details,
}: {
  paths: FlowPath[];
  layers: string[];
  name: (box: string) => string;
  noun: string;
  label: string;
  mark?: (box: string) => Partial<CanvasNode>;
  options?: ReactNode;
  details?: (keys: string[]) => ReactNode;
}) {
  const [hidden, setHidden] = useState<Set<number>>(new Set());
  const [minText, setMinText] = useState("1");
  const min = Math.max(1, Math.floor(Number(minText)) || 1);
  const [selected, setSelected] = useState<string | null>(null);
  const canvas = useRef<CanvasApi>(null);
  const { boxes, lines } = useMemo(
    () => layered(paths, { hidden, min }),
    [paths, hidden, min],
  );
  // A box a filter took away is no longer the one chosen.
  const box = boxes.find((b) => b.id === selected) ?? null;
  const nodes = useMemo(() => {
    const lit = new Set(box?.keys);
    return boxes.map((b) => ({
      id: b.id,
      label: name(b.id),
      sub: plural(b.keys.length, noun),
      faded: !!box && !b.keys.some((k) => lit.has(k)),
      ...mark?.(b.id),
    }));
  }, [boxes, box, name, noun, mark]);
  const edges = useMemo(() => {
    const lit = new Set(box?.keys);
    return lines.map((l) => ({
      id: l.id,
      source: l.source,
      target: l.target,
      type: l.type,
      directed: true,
      width: countWidth(l.keys.length),
      label: String(l.keys.length),
      faded: !!box && !l.keys.some((k) => lit.has(k)),
    }));
  }, [lines, box]);
  if (paths.length === 0)
    return <p className="muted">{`No ${noun}s to show.`}</p>;

  // The boxes on the other end of a box's lines, each with the keys it shares.
  const ends = (end: "source" | "target") => {
    const other = end === "source" ? "target" : "source";
    const by = new Map<string, Set<string>>();
    for (const l of lines)
      if (l[other] === box?.id)
        by.set(l[end], new Set([...(by.get(l[end]) ?? []), ...l.keys]));
    return [...by].map(([id, keys]) => ({ id, text: `${name(id)} ${keys.size}` }));
  };
  const keys = box ? box.keys : [...new Set(paths.map((p) => p.key))];
  const title = box ? `Box ${name(box.id)}` : "Everything shown";
  return (
    <div className="pie-graph">
      <section className="pie-graph-side" aria-label="Flow options">
        {options}
        <h4>Layers</h4>
        {layers.map((l, i) => (
          <label key={l} className="pie-check">
            <input
              type="checkbox"
              aria-label={`${l} layer`}
              checked={!hidden.has(i)}
              // Two layers are the fewest a line can join.
              disabled={!hidden.has(i) && layers.length - hidden.size <= 2}
              onChange={() => {
                const next = new Set(hidden);
                if (next.has(i)) next.delete(i);
                else next.add(i);
                setHidden(next);
              }}
            />
            {l}
          </label>
        ))}
        <h4>Show</h4>
        <label className="pie-check">
          {`Hide lines with fewer ${noun}s than`}
          <input
            type="number"
            className="detail-input pie-graph-min"
            min={1}
            value={minText}
            onChange={(e) => setMinText(e.target.value)}
          />
        </label>
        <p className="muted">{`A line's width and number count its ${noun}s. Choose a box to light the paths through it.`}</p>
      </section>
      <div className="pie-graph-main">
        <div className="pie-filters">
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
          >{`${plural(boxes.length, "box")}, ${plural(
            new Set(lines.flatMap((l) => l.keys)).size,
            noun,
          )}`}</span>
        </div>
        <Suspense fallback={<p className="muted">Loading the graph...</p>}>
          <G6Canvas
            nodes={nodes}
            edges={edges}
            focus={box?.id ?? null}
            layout={LAYERS}
            follow={false}
            onSelect={setSelected}
            label={label}
            apiRef={canvas}
          />
        </Suspense>
      </div>
      <aside className="pie-graph-details" aria-label={title}>
        <span className="mono muted">{box ? layers[box.column] : "Flow"}</span>
        <h3>{box ? name(box.id) : "Everything shown"}</h3>
        <p className="muted">{plural(keys.length, noun)}</p>
        {box &&
          (["source", "target"] as const).map((end) => {
            const items = ends(end);
            const heading = end === "source" ? "Comes from" : "Goes to";
            return (
              items.length > 0 && (
                <section key={end}>
                  <h4>{heading}</h4>
                  <ul className="pie-graph-found" aria-label={heading}>
                    {items.map((t) => (
                      <li key={t.id}>{t.text}</li>
                    ))}
                  </ul>
                </section>
              )
            );
          })}
        {details?.(keys)}
        {box && (
          <div className="pie-graph-actions">
            <button
              type="button"
              className="btn"
              onClick={() => setSelected(null)}
            >
              Show everything
            </button>
          </div>
        )}
      </aside>
    </div>
  );
}
