// VIEWS is the tab list. It has to agree with menuViews in pie/menu.go, and
// the instruction gate checks that it does.
export type View = "results" | "trace" | "map" | "links" | "stats" | "cases" | "cycles";
export const VIEWS = [
  { id: "results", label: "Results" },
  { id: "trace", label: "Trace" },
  { id: "map", label: "Project map" },
  { id: "links", label: "Link map" },
  { id: "stats", label: "Statistics" },
  { id: "cases", label: "Cases" },
  { id: "cycles", label: "Cycles" },
] as const;
