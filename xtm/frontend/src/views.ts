import type { View } from "./contexts/NavContext";

// The views XTM offers, in the order they are shown.
//
// One list, rendered twice: by the tab bar across the top and by the
// navigation rail down the side (#171). Writing the rail as a second set of
// hand-maintained buttons is how the two drift apart, which is the defect
// TAM's View menu and tabs hit and now has a gate against (#147). Here there
// is nothing to pair, because there is only one list.
//
// `shown` is how a view that depends on a backend capability stays out of
// both at once. A view with no `shown` is always offered.
export interface ViewDef {
  id: View;
  label: string;
  // The data-tour hook the onboarding tour points at. The tour names the tab,
  // so the rail does not carry it: two elements with the same hook would make
  // the tour's target ambiguous.
  tour: string;
  shown?: (c: ViewContext) => boolean;
}

// ViewContext is what decides whether an optional view is offered: the
// backend's capabilities, plus the Coverage module's own opt-in, which is a
// user preference rather than something the backend reports.
export interface ViewContext {
  supportsPreconditionObjects: boolean;
  supportsRequirementObjects: boolean;
  supportsComponentAdmin: boolean;
  showCoverage: boolean;
}

export const VIEWS: ViewDef[] = [
  { id: "browse", label: "Browse", tour: "tab-browse" },
  {
    id: "preconditions",
    label: "Preconditions",
    tour: "tab-preconditions",
    shown: (c) => c.supportsPreconditionObjects,
  },
  {
    id: "requirements",
    label: "Requirements",
    tour: "tab-requirements",
    shown: (c) => c.supportsRequirementObjects,
  },
  { id: "duplicates", label: "Duplicates", tour: "tab-duplicates" },
  { id: "gapanalysis", label: "Gap Analysis", tour: "tab-gapanalysis" },
  { id: "testcalls", label: "Test Calls", tour: "tab-testcalls" },
  { id: "dashboard", label: "Dashboard", tour: "tab-dashboard" },
  { id: "traceability", label: "Traceability", tour: "tab-traceability" },
  { id: "plans", label: "Containers", tour: "tab-plans" },
  {
    id: "components",
    label: "Components",
    tour: "tab-components",
    shown: (c) => c.supportsComponentAdmin,
  },
  {
    id: "coverage",
    label: "Coverage",
    tour: "tab-coverage",
    shown: (c) => c.showCoverage,
  },
  { id: "misspellings", label: "Spellcheck", tour: "tab-misspellings" },
];

// visibleViews is the list for this profile. Both the tab bar and the rail
// call it, so a view hidden from one is hidden from the other.
export function visibleViews(c: ViewContext): ViewDef[] {
  return VIEWS.filter((v) => !v.shown || v.shown(c));
}
