import { createViewContext } from "@agile-suite/core";

export type View = "backlog" | "assigned" | "epics" | "boards" | "sprints" | "reports" | "dashboards" | "rituals";

export interface ViewInfo {
  id: View;
  label: string;
}

export const VIEWS: ViewInfo[] = [
  { id: "backlog", label: "Backlog" },
  { id: "assigned", label: "Assigned to me" },
  { id: "epics", label: "Epics" },
  { id: "boards", label: "Boards" },
  { id: "sprints", label: "Sprints" },
  { id: "reports", label: "Reports" },
  { id: "dashboards", label: "Dashboards" },
  { id: "rituals", label: "Rituals" },
];

export const { ViewProvider, useView } = createViewContext<View>("backlog");
