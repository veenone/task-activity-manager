import { useCallback, useMemo, useState } from "react";
import type { query } from "../wailsjs/go/models";
import { type Edge, flowPaths, projectOf } from "./linkData";
import { LayeredFlow } from "./LayeredFlow";
import { ProjectsChooser } from "./ProjectInput";

const LAYERS = ["From project", "Link type", "To project"];

// LinkFlow runs each link from the project it starts in, through its link
// type, to the project it points to (#473), its lines coloured by type.
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
  const starts = useMemo(() => [...new Set(edges.map((e) => projectOf(e.from, issues)))].sort(), [edges, issues]);
  const kept = useMemo(() => left.filter((p) => starts.includes(p)), [left, starts]);
  const paths = useMemo(() => {
    const all = flowPaths(edges, issues);
    return kept.length === 0 ? all : all.filter((p) => kept.includes(p.steps[0].slice(2)));
  }, [edges, issues, kept]);
  const name = useCallback((id: string) => (id.startsWith("t:") ? id.slice(2) : label(id.slice(2))), [label]);
  if (edges.length === 0) return <p className="muted">No links between projects to show.</p>;
  return (
    <LayeredFlow
      paths={paths}
      layers={LAYERS}
      name={name}
      noun="link"
      label="Links from each project, by type, to the project they point to"
      options={<ProjectsChooser label="Left side" projects={starts} projectLabel={label} chosen={kept} onChange={setLeft} />}
    />
  );
}
