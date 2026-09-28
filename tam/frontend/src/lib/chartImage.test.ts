import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { drawnCharts } from "./chartImage";
import { chartAltLine } from "./reportText";

// The real palette is 26 --chart-* tokens on :root, overridden under
// :root[data-theme="dark"], and the marks carry no colour of their own. These
// rules stand in for that cascade: a chart read in dark mode would come back
// carrying the dark value instead of the light one.
const PALETTE = `
:root .chart-line { stroke: rgb(9, 9, 9); stroke-width: 2px; stroke-dasharray: 5 4; }
:root[data-theme="dark"] .chart-line { stroke: rgb(1, 2, 3); }
:root .chart-y-label { fill: rgb(50, 60, 70); text-anchor: end; }
:root[data-theme="dark"] .chart-y-label { fill: rgb(200, 200, 200); }
text { font-family: Inter, sans-serif; font-size: 11px; }
`;

function chart(key: string, title: string): string {
  return `<figure class="chart-figure" data-chart="${key}">
    <figcaption class="chart-title">${title}</figcaption>
    <div class="chart-box">
      <svg class="chart-svg" width="480" height="220" viewBox="0 0 480 220">
        <polyline class="chart-line" points="0,0 10,10"/>
        <text class="chart-y-label" x="10" y="20">34</text>
        <rect class="chart-hit" x="0" y="0" width="10" height="10"/>
      </svg>
    </div>
  </figure>`;
}

// A chart with nothing to draw renders its sentence and no SVG, which is how
// a report with no days carries no picture.
function nothingToDraw(key: string, title: string): string {
  return `<figure class="chart-figure" data-chart="${key}">
    <figcaption class="chart-title">${title}</figcaption>
    <p class="muted chart-empty">No days to draw yet.</p>
  </figure>`;
}

function frame(...figures: string[]): HTMLElement {
  document.head.innerHTML = `<style>${PALETTE}</style>`;
  document.body.innerHTML = `<div class="report-frame">${figures.join("")}</div>`;
  return document.querySelector<HTMLElement>(".report-frame")!;
}

describe("drawnCharts", () => {
  beforeEach(() => {
    document.documentElement.dataset.theme = "dark";
  });
  afterEach(() => {
    delete document.documentElement.dataset.theme;
    document.head.innerHTML = "";
    document.body.innerHTML = "";
  });

  it("writes the light palette onto the marks whatever theme the app is in", () => {
    const [drawn] = drawnCharts(frame(chart("burndown", "Burndown")));
    expect(drawn.markup).toContain("rgb(9, 9, 9)");
    expect(drawn.markup).toContain("rgb(50, 60, 70)");
    expect(drawn.markup).not.toContain("rgb(1, 2, 3)");
    expect(drawn.markup).not.toContain("rgb(200, 200, 200)");
  });

  it("puts the app's own theme back", () => {
    drawnCharts(frame(chart("burndown", "Burndown")));
    expect(document.documentElement.dataset.theme).toBe("dark");
  });

  it("carries the stroke, the dashes and the type a mark is drawn with", () => {
    const [drawn] = drawnCharts(frame(chart("burndown", "Burndown")));
    for (const paint of ["stroke-width: 2px", "stroke-dasharray: 5 4", "font-family: Inter", "text-anchor: end"]) {
      expect(drawn.markup).toContain(paint);
    }
  });

  it("declares the SVG namespace, without which an image will not load it", () => {
    const [drawn] = drawnCharts(frame(chart("burndown", "Burndown")));
    expect(drawn.markup).toContain(`xmlns="http://www.w3.org/2000/svg"`);
  });

  it("leaves out the invisible targets that take the hover", () => {
    const [drawn] = drawnCharts(frame(chart("burndown", "Burndown")));
    expect(drawn.markup).not.toContain("chart-hit");
  });

  it("names each picture and describes it by the chart's own caption", () => {
    const drawn = drawnCharts(frame(chart("burndown", "Burndown"), chart("outcome", "Sprint outcome")));
    expect(drawn.map((d) => [d.key, d.name, d.alt])).toEqual([
      ["burndown", "chart-1.png", chartAltLine("Burndown")],
      ["outcome", "chart-2.png", chartAltLine("Sprint outcome")],
    ]);
  });

  it("measures each picture from the SVG it was drawn at", () => {
    const [drawn] = drawnCharts(frame(chart("burndown", "Burndown")));
    expect([drawn.width, drawn.height]).toEqual([480, 220]);
  });

  it("gives a section every panel it drew", () => {
    const drawn = drawnCharts(frame(chart("velocity", "Velocity in points"), chart("velocity", "Velocity in cards")));
    expect(drawn.map((d) => [d.key, d.alt])).toEqual([
      ["velocity", chartAltLine("Velocity in points")],
      ["velocity", chartAltLine("Velocity in cards")],
    ]);
  });

  it("has no picture for a chart with nothing to draw", () => {
    expect(drawnCharts(frame(nothingToDraw("burndown", "Burndown")))).toEqual([]);
  });

  it("has no picture at all before the report is on screen", () => {
    expect(drawnCharts(null)).toEqual([]);
  });
});
