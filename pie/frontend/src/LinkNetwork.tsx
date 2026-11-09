import { useRef, useState } from "react";
import type { query } from "../wailsjs/go/models";
import { ProjectInput } from "./ProjectInput";
import { busiest, type Edge, neighbours, projectOf, type Spoke } from "./linkData";

// DRAWN is how many neighbours the diagram places around the centre. The list
// beside it names every one, so a busy project stays readable.
const DRAWN = 24;
const SPREAD = 10;
// HUB is the centre circle's radius; spokes start just outside it.
const HUB = 24;
// A pointer that moves further than this, in screen pixels, is a drag and
// not a click, so letting go does not recentre.
const DRAG = 3;

type Point = { x: number; y: number };

const words = (s: Spoke) => [s.out && `${s.out} out`, s.in && `${s.in} in`].filter(Boolean).join(", ");

// LinkNetwork centres on one project and places the projects it links to
// around it, one spoke per link type, with arrows for direction. Choosing a
// project around the centre makes it the centre.
export function LinkNetwork({
  edges,
  issues,
  label,
}: {
  edges: Edge[];
  issues: Map<string, query.Issue>;
  label: (project: string) => string;
}) {
  const projects = [...new Set(edges.flatMap((e) => [projectOf(e.from, issues), projectOf(e.to, issues)]))].sort();
  const [picked, setPicked] = useState("");
  const [hovered, setHovered] = useState<string | null>(null);
  // Positions the analyst dragged projects to, for one centre. Another centre
  // starts from the even ring again.
  const [placed, setPlaced] = useState<{ centre: string; at: Record<string, Point> }>({ centre: "", at: {} });
  const svg = useRef<SVGSVGElement>(null);
  const drag = useRef<{ project: string; from: Point; origin: Point; far: boolean } | null>(null);
  const dragged = useRef(false);
  const centre = projects.includes(picked) ? picked : busiest(edges, issues);
  if (!centre) return <p className="muted">No links between projects to show.</p>;
  const around = neighbours(centre, edges, issues);
  const drawn = around.slice(0, DRAWN);
  const radius = Math.max(140, drawn.length * 16);
  const moved = placed.centre === centre ? placed.at : {};
  const at = (i: number): Point => {
    const project = drawn[i].project;
    if (moved[project]) return moved[project];
    const a = (2 * Math.PI * i) / drawn.length - Math.PI / 2;
    return { x: radius * Math.cos(a), y: radius * Math.sin(a) };
  };
  const box = radius + 90;
  // unit is screen pixels per chart unit, read from the drawn SVG; a test DOM
  // has no layout, so one to one.
  const unit = () => svg.current?.getScreenCTM?.()?.a || 1;
  const startDrag = (project: string, e: React.PointerEvent, origin: Point) => {
    // Only the main button drags; a new press also forgets a drag whose click
    // never came, so it cannot swallow this one.
    dragged.current = false;
    if (e.button !== 0) return;
    drag.current = { project, from: { x: e.clientX, y: e.clientY }, origin, far: false };
    (e.currentTarget as Element).setPointerCapture?.(e.pointerId);
  };
  const moveDrag = (e: React.PointerEvent) => {
    const d = drag.current;
    if (!d) return;
    const [dx, dy] = [e.clientX - d.from.x, e.clientY - d.from.y];
    d.far ||= Math.hypot(dx, dy) > DRAG;
    if (!d.far) return;
    const u = unit();
    setPlaced({ centre, at: { ...moved, [d.project]: { x: d.origin.x + dx / u, y: d.origin.y + dy / u } } });
  };
  const endDrag = () => {
    dragged.current = drag.current?.far ?? false;
    drag.current = null;
  };

  return (
    <div className="pie-network">
      <div className="pie-network-chart">
        <div className="pie-network-pick">
          <ProjectInput
            label="Centre project"
            allText="Busiest project"
            projects={projects}
            projectLabel={label}
            value={centre}
            onChange={setPicked}
          />
        </div>
        <svg
          ref={svg}
          className="pie-network-svg"
          viewBox={`${-box} ${-box} ${box * 2} ${box * 2}`}
          role="img"
          aria-label={`Projects linked to ${centre} by link type`}
        >
          <defs>
            <marker
              id="pie-arrow"
              viewBox="0 0 10 10"
              refX="10"
              refY="5"
              markerUnits="userSpaceOnUse"
              markerWidth="9"
              markerHeight="9"
              orient="auto-start-reverse"
            >
              <path className="pie-network-arrow" d="M0,0 L10,5 L0,10 z" />
            </marker>
          </defs>
          {drawn.map((n, i) => {
            const p = at(i);
            const len = Math.max(1, Math.hypot(p.x, p.y));
            const [nx, ny] = [-p.y / len, p.x / len];
            return n.types.map((s, j) => {
              const off = (j - (n.types.length - 1) / 2) * SPREAD;
              const [x0, y0, x1, y1] = [nx * off, ny * off, p.x * 0.88 + nx * off, p.y * 0.88 + ny * off];
              return (
                <g key={`${n.project}|${s.type}`}>
                  <line
                    className="pie-network-spoke"
                    x1={x0 + (p.x * (HUB + 4)) / len}
                    y1={y0 + (p.y * (HUB + 4)) / len}
                    x2={x1}
                    y2={y1}
                    strokeWidth={1 + Math.log2(1 + s.out + s.in)}
                    markerEnd={s.out ? "url(#pie-arrow)" : undefined}
                    markerStart={s.in ? "url(#pie-arrow)" : undefined}
                  >
                    <title>{`${label(centre)} and ${label(n.project)}: ${s.type}, ${words(s)}`}</title>
                  </line>
                  {drawn.length <= 12 && (
                    <text className="pie-network-type" x={(x0 + x1) / 2} y={(y0 + y1) / 2} textAnchor="middle">
                      {`${s.type} ${s.out + s.in}`}
                    </text>
                  )}
                </g>
              );
            });
          })}
          <circle className="pie-network-centre" r={HUB} />
          <text className="pie-network-hub" textAnchor="middle" dominantBaseline="central">
            {centre}
          </text>
          {drawn.map((n, i) => {
            const p = at(i);
            return (
              <g
                key={n.project}
                data-project={n.project}
                className="pie-network-node"
                onMouseEnter={() => setHovered(n.project)}
                onMouseLeave={() => setHovered(null)}
                onPointerDown={(e) => startDrag(n.project, e, p)}
                onPointerMove={moveDrag}
                onPointerUp={endDrag}
                onPointerCancel={endDrag}
                onClick={() => {
                  if (dragged.current) dragged.current = false;
                  else setPicked(n.project);
                }}
              >
                <circle cx={p.x} cy={p.y} r={7} />
                {hovered !== n.project && (
                  <text className="pie-network-label" x={p.x} y={p.y + (p.y < 0 ? -12 : 20)} textAnchor="middle">
                    {label(n.project)}
                  </text>
                )}
              </g>
            );
          })}
          {/* Drawn last, so it sits on top of every other label. */}
          {drawn.map((n, i) => {
            if (n.project !== hovered) return null;
            const p = at(i);
            return (
              <text
                key="hovered"
                className="pie-network-label pie-network-label-hover"
                x={p.x}
                y={p.y + (p.y < 0 ? -12 : 20)}
                textAnchor="middle"
              >
                {label(n.project)}
              </text>
            );
          })}
        </svg>
        {around.length > DRAWN && (
          <p className="muted">{`The ${DRAWN} projects with the most links are drawn. The list names all ${around.length}.`}</p>
        )}
      </div>
      <section className="pie-flow-side">
        <h3>{`Linked to ${label(centre)}`}</h3>
        <ul aria-label={`Projects linked to ${centre}`}>
          {around.map((n) => (
            <li key={n.project}>
              <button type="button" className="pie-map-key" onClick={() => setPicked(n.project)}>
                {`${n.project}: ${n.types.map((s) => `${s.type} ${words(s)}`).join("; ")}`}
              </button>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
