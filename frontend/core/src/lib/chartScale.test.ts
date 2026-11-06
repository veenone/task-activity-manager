import { describe, expect, it } from "vitest";
import { band, dayIndex, labelEvery, linear, niceTicks, spread } from "./chartScale";

describe("linear", () => {
  it("maps the domain's ends onto the range's ends and interpolates between", () => {
    const x = linear([0, 10], [0, 200]);
    expect(x(0)).toBe(0);
    expect(x(10)).toBe(200);
    expect(x(2.5)).toBe(50);
  });

  it("runs a range backwards, the way a y axis grows upwards on screen", () => {
    const y = linear([0, 40], [180, 20]);
    expect(y(0)).toBe(180);
    expect(y(40)).toBe(20);
    expect(y(20)).toBe(100);
  });

  it("puts a single-day domain in the middle of the range rather than dividing by zero", () => {
    const x = linear([0, 0], [0, 300]);
    expect(x(0)).toBe(150);
    expect(Number.isNaN(x(0))).toBe(false);
  });
});

describe("niceTicks", () => {
  it("steps by 1, 2 or 5 times a power of ten and reaches past the maximum", () => {
    expect(niceTicks(34, 5)).toEqual([0, 10, 20, 30, 40]);
    expect(niceTicks(7, 5)).toEqual([0, 2, 4, 6, 8]);
    expect(niceTicks(23, 5)).toEqual([0, 5, 10, 15, 20, 25]);
  });

  it("handles a maximum below one, since a board of halves can be small", () => {
    expect(niceTicks(2.5, 5)).toEqual([0, 0.5, 1, 1.5, 2, 2.5]);
  });

  it("gives a chart of nothing one tick at zero", () => {
    expect(niceTicks(0, 5)).toEqual([0]);
  });
});

describe("band", () => {
  it("lays n bands across the width with the padding between and outside them", () => {
    const b = band(4, 400, 0.2);
    // step = 400 / (4 - 0.2 + 0.4) = 400 / 4.2
    expect(b.step).toBeCloseTo(95.238, 2);
    expect(b.width).toBeCloseTo(76.19, 1);
    expect(b.x(0)).toBeCloseTo(19.05, 1);
    expect(b.x(3)).toBeCloseTo(19.05 + 3 * 95.238, 1);
    expect(b.x(3) + b.width).toBeLessThanOrEqual(400);
  });

  it("gives one band the whole width less its padding", () => {
    const b = band(1, 200, 0.25);
    expect(b.x(0) + b.width).toBeLessThanOrEqual(200);
    expect(b.width).toBeGreaterThan(100);
  });

  it("has nothing to place for zero bands and does not divide by zero", () => {
    const b = band(0, 200, 0.2);
    expect(Number.isFinite(b.step)).toBe(true);
    expect(b.width).toBe(0);
  });
});

describe("dayIndex", () => {
  it("numbers the days from the first one by calendar distance", () => {
    expect(dayIndex([{ date: "2026-09-01" }, { date: "2026-09-02" }, { date: "2026-09-03" }])).toEqual([0, 1, 2]);
  });

  it("keeps a gap where a day is missing, so the line does not compress time", () => {
    expect(dayIndex([{ date: "2026-09-01" }, { date: "2026-09-04" }])).toEqual([0, 3]);
  });

  it("crosses a month end and a daylight saving change by whole days", () => {
    expect(dayIndex([{ date: "2026-03-28" }, { date: "2026-03-30" }, { date: "2026-04-01" }])).toEqual([0, 2, 4]);
  });

  it("is a single zero for a single-day sprint and empty for no days", () => {
    expect(dayIndex([{ date: "2026-09-01" }])).toEqual([0]);
    expect(dayIndex([])).toEqual([]);
  });
});

describe("labelEvery", () => {
  it("labels every day of a two-week sprint at a comfortable width", () => {
    expect(labelEvery(14, 600)).toBe(1);
  });

  it("thins the labels of a sprint over thirty days so they cannot collide", () => {
    expect(labelEvery(31, 600)).toBeGreaterThan(1);
    expect(labelEvery(60, 600)).toBeGreaterThan(labelEvery(31, 600));
  });

  it("thins a two-week sprint too when the pane is narrow", () => {
    expect(labelEvery(14, 300)).toBeGreaterThan(1);
  });

  it("never asks for less than every label, whatever the width", () => {
    expect(labelEvery(0, 600)).toBe(1);
    expect(labelEvery(3, 0)).toBeGreaterThanOrEqual(1);
  });
});

describe("spread", () => {
  it("leaves labels alone when they already clear each other", () => {
    expect(spread([10, 40, 80], 12)).toEqual([10, 40, 80]);
  });

  it("pushes labels that would overlap apart by the gap, keeping their order", () => {
    const out = spread([50, 52, 100], 12);
    expect(out[1] - out[0]).toBeGreaterThanOrEqual(12);
    expect(out[2]).toBe(100);
    expect(out[0]).toBeLessThan(out[1]);
  });

  it("does not reorder labels given out of order", () => {
    const out = spread([80, 20, 82], 12);
    expect(out[1]).toBe(20);
    expect(out[2] - out[0]).toBeGreaterThanOrEqual(12);
  });
});
