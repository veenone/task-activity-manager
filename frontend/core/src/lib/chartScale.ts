// chartScale is the arithmetic under the report charts, and nothing else:
// no React, no DOM, no measuring. SVG text cannot be measured before layout,
// so everything here works from counts and character estimates, which is
// what keeps it pure enough to test with numbers alone.

// LABEL_WIDTH is the room one x label ("12 Sep") is assumed to take, in
// pixels, including the gap to the next one. An estimate, not a measure:
// see the file comment.
const LABEL_WIDTH = 40;

// linear maps a value in domain onto range, either way round. A domain of no
// width (a single-day sprint) maps everything to the middle of the range,
// so one point draws in the centre rather than at NaN.
export function linear(domain: [number, number], range: [number, number]): (v: number) => number {
  const [d0, d1] = domain;
  const [r0, r1] = range;
  if (d1 === d0) return () => (r0 + r1) / 2;
  return (v: number) => r0 + ((v - d0) / (d1 - d0)) * (r1 - r0);
}

// niceTicks is the y axis: zero to a round number at or past max, stepping
// by 1, 2 or 5 times a power of ten, with roughly count ticks. A max of
// zero is one tick at zero; the chart has nothing to scale.
export function niceTicks(max: number, count: number): number[] {
  if (max <= 0) return [0];
  const raw = max / Math.max(1, count);
  const power = 10 ** Math.floor(Math.log10(raw));
  const unit = raw / power;
  const step = (unit <= 1 ? 1 : unit <= 2 ? 2 : unit <= 5 ? 5 : 10) * power;
  const ticks: number[] = [];
  // Rounding each tick keeps 0.1 * 3 from printing as 0.30000000000000004.
  const places = Math.max(0, -Math.floor(Math.log10(step)));
  for (let v = 0; v < max + step; v += step) ticks.push(Number(v.toFixed(places)));
  return ticks;
}

export interface Band {
  // step is the distance from one band's left edge to the next's.
  step: number;
  // width is each band's own width, step less the inner padding.
  width: number;
  x: (i: number) => number;
}

// band places n equal bands across width, padding being the fraction of a
// step left empty between bands and, once more, at either edge. It is the
// x scale of the bar charts.
export function band(n: number, width: number, padding: number): Band {
  if (n <= 0) return { step: 0, width: 0, x: () => 0 };
  const step = width / (n - padding + 2 * padding);
  return { step, width: step * (1 - padding), x: (i) => padding * step + i * step };
}

// dayIndex numbers each day by its calendar distance from the first one, so
// a missing day leaves a gap on the axis rather than pulling the days after
// it closer. Dates are the backend's bare 2006-01-02 form, read as UTC so
// no local daylight change makes a day 23 hours long.
export function dayIndex(days: { date: string }[]): number[] {
  if (days.length === 0) return [];
  const at = (d: string) => Date.UTC(Number(d.slice(0, 4)), Number(d.slice(5, 7)) - 1, Number(d.slice(8, 10)));
  const first = at(days[0].date);
  const msPerDay = 24 * 60 * 60 * 1000;
  return days.map((d) => Math.round((at(d.date) - first) / msPerDay));
}

// labelEvery is how many days apart the x labels go so that n of them fit
// in width: 1 for a two-week sprint at a comfortable width, more for a long
// sprint or a narrow pane. The tooltip carries the exact date of every day
// whether or not the axis names it.
export function labelEvery(n: number, width: number): number {
  if (n <= 0 || width <= 0) return 1;
  return Math.max(1, Math.ceil((n * LABEL_WIDTH) / width));
}

// spread pushes value labels apart vertically so two lines that end near
// each other do not print one number over the other. The labels are
// walked from the topmost down and each is moved down to at least gap
// below the one above it; the result keeps the order it was given in.
export function spread(ys: number[], gap: number): number[] {
  const order = ys.map((y, i) => i).sort((a, b) => ys[a] - ys[b]);
  const out = [...ys];
  for (let k = 1; k < order.length; k += 1) {
    const above = out[order[k - 1]];
    if (out[order[k]] < above + gap) out[order[k]] = above + gap;
  }
  return out;
}
