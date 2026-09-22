import type { ReportSeries } from "../api";

// reportFigures is the five review figures in the order they are read, with
// the key each surface colours or styles them by. The metric tiles and the
// outcome chart both draw this list, so the two cannot label the last figure
// differently: it is Carried over once a sprint has closed and Remaining
// while it runs.
//
// group is which of the three the figure belongs to: the baseline the sprint
// started from, the changes made to it, and what came of it.
export type FigureGroup = "baseline" | "change" | "outcome";

export interface ReportFigure {
  key: "committed" | "added" | "removed" | "completed" | "carried";
  label: string;
  value: number;
  group: FigureGroup;
}

export function reportFigures(s: ReportSeries, live = false): ReportFigure[] {
  return [
    { key: "committed", label: "Committed", value: s.committed, group: "baseline" },
    { key: "added", label: "Added", value: s.added, group: "change" },
    { key: "removed", label: "Removed", value: s.removed, group: "change" },
    { key: "completed", label: "Completed", value: s.completed, group: "outcome" },
    { key: "carried", label: live ? "Remaining" : "Carried over", value: s.carriedOver, group: "outcome" },
  ];
}

// FIGURE_GROUPS names the three groups in reading order, for the tiles.
export const FIGURE_GROUPS: { group: FigureGroup; label: string }[] = [
  { group: "baseline", label: "Baseline" },
  { group: "change", label: "Changes" },
  { group: "outcome", label: "Outcome" },
];
