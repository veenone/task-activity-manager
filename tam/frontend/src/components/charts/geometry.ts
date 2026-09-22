// The report charts' sizes, in one place so no component carries a pixel
// literal of its own. Widths are measured from the pane; everything else is
// fixed here.

// FALLBACK_WIDTH is drawn with before the box has been measured, and under
// a test runner that has no ResizeObserver to measure it with.
export const FALLBACK_WIDTH = 480;

// LINE_HEIGHT is the burndown's height and BAR_HEIGHT the velocity chart's.
// The outcome chart is as tall as its rows.
export const LINE_HEIGHT = 220;
export const BAR_HEIGHT = 200;
export const OUTCOME_ROW = 28;

// MARGIN leaves room for the y labels on the left, the value labels past the
// last point on the right, and the x labels under the axis.
export const MARGIN = { top: 14, right: 64, bottom: 26, left: 40 };
// OUTCOME_MARGIN leaves the row labels on the left and the amounts on the
// right of a horizontal bar.
export const OUTCOME_MARGIN = { top: 4, right: 80, bottom: 4, left: 92 };

// POINT_RADIUS is a drawn point; HIT_RADIUS the larger invisible target
// around it that takes the hover and the focus.
export const POINT_RADIUS = 3;
export const HIT_RADIUS = 10;

// TICKS is roughly how many y ticks a chart asks niceTicks for.
export const TICKS = 4;

// BAR_PADDING is the fraction of a band left empty between bar groups.
export const BAR_PADDING = 0.3;

// LABEL_OFFSET is the gap between a mark and the number written beside it,
// and VALUE_GAP the least two end labels may be apart vertically.
export const LABEL_OFFSET = 6;
export const VALUE_GAP = 13;
