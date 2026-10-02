import { driver } from "driver.js";
import "driver.js/dist/driver.css";
import "./tour.css";

// The onboarding tour, shared by both apps. The steps themselves are not
// here: each app's tour is about its own views and its own words, so it
// writes its own steps file and hands the list in.
//
// CONSTRAINT, carried from the steps that use it: every target must be an
// element that is ALWAYS MOUNTED while its tour's view is showing.
// Conditionally rendered targets (a pending-changes badge, an open detail
// panel, anything inside a modal) are the main way tours break, because
// the step lands on nothing. A step whose target is not there is dropped
// rather than shown against the page, which is what keeps a view that
// omits one degrading quietly.

export interface TourStep {
  id: string;
  /** Value of the target element's data-tour attribute. */
  target: string;
  title: string;
  body: string;
  side?: "top" | "bottom" | "left" | "right";
}

export interface TourOptions {
  /** Each view id's steps, keyed the way the app keys its views. */
  tours: Record<string, TourStep[]>;
  /** What the app records as seen once the tour ends. */
  version: number;
  /**
   * Writes the seen version. Best-effort by contract: a failure means the
   * tour is offered again next launch, which is a far better failure than
   * trapping somebody inside it.
   */
  markSeen: (version: number) => Promise<unknown>;
  /** Called once the tour ends, whether completed or skipped. */
  onFinish: () => void;
}

// useTour builds the driver instance and returns a start function. It is
// not stateful React, just a factory, so it can be called from a menu
// handler or an effect without re-render concerns. `start` takes the view
// id whose tour to run, so the same handler serves every view's tour.
export function useTour({ tours, version, markSeen, onFinish }: TourOptions) {
  function seen() {
    markSeen(version).catch(() => {});
    onFinish();
  }

  // start runs the tour for `viewId`. onBeforeStart (optional) puts the app
  // in the state the steps assume, such as switching to the first-run view;
  // on-demand launches run against the current view and pass nothing.
  function start(viewId: string, onBeforeStart?: () => void) {
    onBeforeStart?.();
    // The DOM has to settle after any onBeforeStart view switch, or the
    // first selectors resolve against the outgoing view.
    requestAnimationFrame(() => {
      const steps = (tours[viewId] ?? [])
        .map((s) => ({
          element: `[data-tour="${s.target}"]`,
          popover: {
            title: s.title,
            description: s.body,
            side: s.side ?? "bottom",
          },
        }))
        .filter((s) => document.querySelector(s.element) !== null);

      if (steps.length === 0) return;

      driver({
        showProgress: true,
        allowClose: true,
        nextBtnText: "Next",
        prevBtnText: "Back",
        doneBtnText: "Done",
        onDestroyed: seen,
        steps,
      }).drive();
    });
  }

  return { start };
}
