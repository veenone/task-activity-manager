import { useState } from "react";
import type { query } from "../wailsjs/go/models";
import { type Edge, flowLinks, projectOf } from "./linkData";
import { ProjectsChooser } from "./ProjectInput";
import { FLOW_NODE, sankey } from "./mapData";
import { Zoom } from "./Zoom";

// INNER is the width the three columns and their bands take. The margins
// beside it grow with the longest name, so no name is cut off (#455).
const INNER = 460;
const HEIGHT = 360;
const MARGIN = 200;
// CHAR is about how wide one character of a label is, in chart units.
const CHAR = 7;

// LinkFlow draws each link from the project it starts in, through its link
// type, to the project it points to, one band per pair sized by link count.
export function LinkFlow({
  edges,
  issues,
  label,
}: {
  edges: Edge[];
  issues: Map<string, query.Issue>;
  label: (project: string) => string;
}) {
  // The projects kept on the left; none chosen keeps them all.
  const [left, setLeft] = useState<string[]>([]);
  if (edges.length === 0) return <p className="muted">No links between projects to show.</p>;
  const starts = [...new Set(edges.map((e) => projectOf(e.from, issues)))].sort();
  const kept = left.filter((p) => starts.includes(p));
  const shown = kept.length === 0 ? edges : edges.filter((e) => kept.includes(projectOf(e.from, issues)));
  const { columns, links } = flowLinks(shown, issues);
  const name = (id: string) => (id.startsWith("t:") ? id.slice(2) : label(id.slice(2)));
  const { nodes, bands, height } = sankey(columns, links, { width: INNER, height: HEIGHT, gap: 10 });
  const margin = (prefix: string) =>
    Math.max(MARGIN, ...nodes.filter((n) => n.id.startsWith(prefix)).map((n) => `${name(n.id)} ${n.value}`.length * CHAR + 20));
  const [lead, tail] = [margin("f:"), margin("o:")];
  const width = lead + INNER + tail;
  return (
    <div className="pie-flow">
      <div className="pie-flow-pick">
        <ProjectsChooser label="Left side" projects={starts} projectLabel={label} chosen={kept} onChange={setLeft} />
      </div>
      <Zoom label="Link flow">
        <svg
          className="pie-flow-svg"
          viewBox={`${-lead} -10 ${width} ${height + 20}`}
          // Wider than the box, it scrolls sideways instead of shrinking
          // its names.
          style={{ minWidth: `${width}px` }}
          role="img"
          aria-label="Links from each project, by type, to the project they point to. The grid view counts the same links."
        >
          {bands.map((b) => (
            <path key={`${b.from}|${b.to}`} className="pie-flow-band pie-flow-plain" d={b.path}>
              <title>{`${name(b.from)} to ${name(b.to)}: ${b.keys.length}`}</title>
            </path>
          ))}
          {nodes.map((n) => {
            const left = n.id.startsWith("f:");
            return (
              <g key={n.id}>
                <rect
                  className={`pie-flow-node ${n.id.startsWith("t:") ? "pie-flow-type" : "pie-flow-plain"}`}
                  x={n.x}
                  y={n.y}
                  width={FLOW_NODE}
                  height={Math.max(n.h, 2)}
                  rx={2}
                />
                <text
                  className="pie-flow-label"
                  x={left ? n.x - 6 : n.x + FLOW_NODE + 6}
                  y={n.y + n.h / 2}
                  textAnchor={left ? "end" : "start"}
                >
                  {`${name(n.id)} ${n.value}`}
                </text>
              </g>
            );
          })}
        </svg>
      </Zoom>
    </div>
  );
}
