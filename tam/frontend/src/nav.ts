import { createViewContext } from "@agile-suite/core";

export type View = "backlog" | "epics" | "boards" | "reports" | "rituals";

export interface ViewInfo {
  id: View;
  label: string;
  // The phase of the foundation design that delivers the view.
  phase: string;
  blurb: string;
}

export const VIEWS: ViewInfo[] = [
  {
    id: "backlog",
    label: "Backlog",
    phase: "Phase 1",
    blurb: "Issue sync, the grid, and the detail panel are the first feature slice.",
  },
  {
    id: "epics",
    label: "Epics",
    phase: "Phase 2",
    blurb: "The epic to story to task tree.",
  },
  {
    id: "boards",
    label: "Boards",
    phase: "Phase 3",
    blurb: "Kanban and the active sprint, with live drag.",
  },
  {
    id: "reports",
    label: "Reports",
    phase: "Phase 4",
    blurb: "Burndown, velocity, and sprint analytics.",
  },
  {
    id: "rituals",
    label: "Rituals",
    phase: "Phase 5",
    blurb: "Confluence pages for planning, standups, reviews, and retros.",
  },
];

export const { ViewProvider, useView } = createViewContext<View>("backlog");
