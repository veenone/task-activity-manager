import type { query } from "../wailsjs/go/models";
import { type Edge, flowLinks } from "./linkData";
import { FLOW_NODE, sankey } from "./mapData";
import { Zoom } from "./Zoom";

const WIDTH = 860;
const HEIGHT = 360;
const LEFT = 200;
const RIGHT = 200;

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
  if (edges.length === 0) return <p className="muted">No links between projects to show.</p>;
  const { columns, links } = flowLinks(edges, issues);
  const name = (id: string) => (id.startsWith("t:") ? id.slice(2) : label(id.slice(2)));
  const { nodes, bands, height } = sankey(columns, links, { width: WIDTH - LEFT - RIGHT, height: HEIGHT, gap: 10 });
  return (
    <div className="pie-flow">
      <Zoom label="Link flow">
        <svg
          className="pie-flow-svg"
          viewBox={`${-LEFT} -10 ${WIDTH} ${height + 20}`}
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
