import { createViewContext } from "@agile-suite/core";

export type View = "backlog" | "assigned" | "worklog" | "epics" | "boards" | "sprints" | "reports" | "dashboards" | "rituals";

export interface ViewInfo {
  id: View;
  label: string;
  // gated views offer actions a Jira permission can hold back, so they say
  // in text when the token lacks one (#533).
  gated?: boolean;
}

export const VIEWS: ViewInfo[] = [
  { id: "backlog", label: "Backlog", gated: true },
  { id: "assigned", label: "Assigned to me", gated: true },
  { id: "worklog", label: "Worklog", gated: true },
  { id: "epics", label: "Epics", gated: true },
  { id: "boards", label: "Boards", gated: true },
  { id: "sprints", label: "Sprints", gated: true },
  { id: "reports", label: "Reports" },
  { id: "dashboards", label: "Dashboards" },
  { id: "rituals", label: "Rituals" },
];

export const { ViewProvider, useView } = createViewContext<View>("backlog");
