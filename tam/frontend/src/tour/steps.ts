// TAM's onboarding tours, as data. The hook that runs them is shared with
// XTM and lives in @agile-suite/core; the words are TAM's own.
//
// Every step targets a `data-tour` attribute rather than a CSS class.
// Classes churn with styling work; a dedicated attribute is an explicit
// contract, so a reader can see at the element that something depends on
// it.
//
// CONSTRAINT: every target must be an element that is ALWAYS MOUNTED while
// its tour's view is showing. Conditionally rendered targets (the pending
// badge, an open detail panel, anything inside a modal) are the main way
// tours break, because the step lands on nothing. So the first-run tour
// explains the local cache and the Commit step from stable anchors instead
// of spotlighting the widgets themselves, and each view's tour anchors on
// that view's own tab, its body, and the Help menu, all three of which are
// always there.

import type { TourStep } from "@agile-suite/core";
import { VIEWS } from "../nav";

export const TOUR_VERSION = 1;

// The first-run tour: the loop TAM is built around. It runs on the Backlog,
// which is where a new profile lands.
const START_STEPS: TourStep[] = [
  {
    id: "profile",
    target: "profile",
    title: "Your connection",
    body: "Each profile points at one Jira project. Switch here to work on another. A profile on the demo URL runs on sample data rather than a real Jira.",
    side: "bottom",
  },
  {
    id: "sync",
    target: "sync",
    title: "Pull from Jira",
    body: "Sync copies the project's issues into a local cache on this machine. That cache is what makes the backlog instant. Jira stays the system of record; the cache is a copy of it.",
    side: "bottom",
  },
  {
    id: "views",
    target: "views",
    title: "The views",
    body: "Each area of TAM has its own tab here. Every one has a short tour of its own, in the Help menu while you are on it.",
    side: "bottom",
  },
  {
    id: "edits",
    target: "backlog-body",
    title: "Edits stay here until you commit",
    body: "This is the part that surprises people. Editing an issue, moving it between sprints or dragging a card writes to the local cache, not to Jira. The changes collect as pending work, and a count appears in the top bar once you have some.",
    side: "top",
  },
  {
    id: "commit",
    target: "sync",
    title: "Commit sends them",
    body: "Nothing reaches Jira until you commit. That is what makes it safe to plan a sprint here, change your mind twice and only then push the result.",
    side: "bottom",
  },
  {
    id: "restart",
    target: "help",
    title: "That is the loop",
    body: "Sync, plan, commit. Everything else builds on it. You can run this tour again any time from this menu.",
    side: "bottom",
  },
];

// what each view's tour says about itself: the sentence on its tab, and
// the sentence on its body.
const VIEW_WORDS: Record<string, { purpose: string; body: string }> = {
  backlog: {
    purpose: "Every issue the profile's project holds, as the cache has it. Filter by text, type or sprint, and open one to edit it.",
    body: "The grid is one page of the filter. Subtasks stay under their parent, and the Export button writes the whole filter to a workbook.",
  },
  assigned: {
    purpose: "The same grid, narrowed to the issues assigned to you in Jira.",
    body: "Your own work, in rank order. The rest of the project is a tab away.",
  },
  epics: {
    purpose: "The project's epics with their children underneath, for seeing how a body of work is broken down.",
    body: "Open an epic to see what hangs off it and how much of it has landed.",
  },
  boards: {
    purpose: "A board's columns with its cards in them, read from the cache and moved locally until you commit.",
    body: "Drag a card between columns here. The move is journalled, not sent, until Commit.",
  },
  sprints: {
    purpose: "A board's sprints: what is in each one, and the dialogs that start, complete and edit them.",
    body: "Each sprint lists the work it carries. A sprint you draft here is created in Jira on Commit.",
  },
  reports: {
    purpose: "A sprint reconstructed from its own history: the burndown, the outcome figures and the board's velocity.",
    body: "The charts and the figures are built from Jira's changelog, and can be published to Confluence or exported.",
  },
  dashboards: {
    purpose: "The numbers behind one of your saved Jira filters, counted and kept so they read with Jira unreachable.",
    body: "Each panel is one filter. Refresh re-runs it; the stamp says how old the figures are.",
  },
  rituals: {
    purpose: "The team's standing documents, edited here and synced to Confluence.",
    body: "The document you are editing is local until it is synced, the same way an issue edit is.",
  },
};

// viewTour walks the view's own tab, its body, and the Help menu the tour
// can be re-run from. The tab and the menu are in the chrome and so are
// always mounted; the body is the view's own root element, which is there
// for as long as the view is.
function viewTour(view: string, label: string): TourStep[] {
  const words = VIEW_WORDS[view];
  if (!words) return [];
  return [
    { id: `${view}-intro`, target: `tab-${view}`, title: label, body: words.purpose, side: "bottom" },
    { id: `${view}-body`, target: `${view}-body`, title: "The main area", body: words.body, side: "top" },
    {
      id: `${view}-rerun`,
      target: "help",
      title: "Run this again",
      body: "Every view's tour is in this menu, and it always runs the one you are looking at.",
      side: "bottom",
    },
  ];
}

// TOURS is the first-run tour plus one per view, keyed the way the views
// are, so the Help menu can start the current one by its own id.
export const TOURS: Record<string, TourStep[]> = {
  start: START_STEPS,
  ...Object.fromEntries(VIEWS.map((v) => [v.id, viewTour(v.id, v.label)])),
};

// shouldOfferTour is when the first-run tour starts by itself: after the
// first successful sync, and only for somebody who has not been through
// this version of it.
//
// Not at launch. A tour of an empty backlog explains nothing, and the one
// thing a new profile has to do first is sync. Not again afterwards
// either: the version is written when the tour ends, so a second sync in
// the same session finds it seen.
export function shouldOfferTour(seenVersion: number, hasSynced: boolean): boolean {
  return hasSynced && seenVersion < TOUR_VERSION;
}
