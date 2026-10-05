import type { ReportImage } from "../api";
import { chartAltLine, chartUndrawableLine } from "./reportText";

// chartImage turns the charts on screen into the pictures the report's outputs
// carry. It exists because the charts are the only place those pictures could
// come from: every mark in them is coloured by a CSS class that resolves to
// one of the --chart-* custom properties, so the SVG carries no colour of its
// own and nothing outside a browser can put one there.
//
// Two things follow from that. The computed paint has to be written onto the
// elements before the SVG is serialized, or the picture comes out colourless.
// And it has to be read with the light palette in force whatever theme the app
// is in, or a user reading TAM in dark mode publishes a dark chart onto a
// white Confluence page and drops a dark PNG into a white spreadsheet.
//
// The split in this module is the canvas: drawnCharts does everything up to
// it and is ordinary DOM work, and chartImages adds the one step a test
// environment has no canvas for.

// ChartKey is the section of the document a picture belongs to. The three
// charts declare their own through data-chart, so a picture is placed by the
// chart that drew it rather than by matching a caption.
export type ChartKey = "outcome" | "burndown" | "burndownTime" | "velocity";

// ChartImages is every picture collected, by the section it belongs to. A
// section can own several: the velocity chart splits into one panel per unit
// when a board changed how it estimates.
export type ChartImages = Partial<Record<ChartKey, ReportImage[]>>;

// Drawn is one chart ready for the canvas: where its picture belongs, what it
// is called, what it says, the SVG with its colours written on, and the size
// it was drawn at.
export interface Drawn {
  key: ChartKey;
  name: string;
  alt: string;
  title: string;
  markup: string;
  width: number;
  height: number;
}

// PAINT is what the .chart-* rules set on a mark, and TYPE what they set on a
// label, plus the font family, which a label inherits from the page and a
// serialized SVG has no page to inherit from.
const PAINT = ["fill", "stroke", "stroke-width", "stroke-dasharray", "stroke-linecap", "stroke-linejoin"];
const TYPE = ["font-family", "font-size", "font-weight", "font-variant-numeric", "text-anchor", "dominant-baseline"];

// drawnCharts is every chart drawn inside root, with its colours resolved.
//
// A chart with nothing to draw renders its sentence and no SVG, which is how
// a report with no days carries no picture and no empty frame.
export function drawnCharts(root: HTMLElement | null): Drawn[] {
  if (!root) return [];
  const found: { key: ChartKey; title: string; svg: SVGSVGElement }[] = [];
  for (const figure of root.querySelectorAll<HTMLElement>("[data-chart]")) {
    const svg = figure.querySelector("svg");
    if (!svg) continue;
    found.push({
      key: figure.dataset.chart as ChartKey,
      title: figure.querySelector(".chart-title")?.textContent?.trim() ?? "",
      svg,
    });
  }
  // One read, one task: the attribute is put back before anything else can
  // run, and the finally means a throw in the middle of the walk cannot leave
  // the app stuck in the light palette.
  return inLightPalette(() =>
    found.map((c, i) => ({
      key: c.key,
      name: `chart-${i + 1}.png`,
      alt: chartAltLine(c.title),
      title: c.title,
      markup: painted(c.svg),
      width: Number(c.svg.getAttribute("width")),
      height: Number(c.svg.getAttribute("height")),
    })),
  );
}

// chartImages is drawnCharts through a canvas, which is where the PNG the
// outputs carry comes from.
export async function chartImages(root: HTMLElement | null): Promise<ChartImages> {
  const out: ChartImages = {};
  for (const drawn of drawnCharts(root)) {
    const image: ReportImage = { name: drawn.name, alt: drawn.alt, data: await rasterise(drawn) };
    out[drawn.key] = [...(out[drawn.key] ?? []), image];
  }
  return out;
}

// inLightPalette reads with the light tokens in force. The tokens are defined
// on :root and overridden under :root[data-theme="dark"], so the light values
// are only reachable by setting that attribute: a nested element with a theme
// of its own inherits the root's custom properties rather than redefining
// them, so there is no way to do this without touching the document element.
function inLightPalette<T>(read: () => T): T {
  const root = document.documentElement;
  const was = root.dataset.theme;
  root.dataset.theme = "light";
  try {
    return read();
  } finally {
    if (was === undefined) delete root.dataset.theme;
    else root.dataset.theme = was;
  }
}

// painted is the SVG with every computed paint value written onto its own
// elements, serialized. The stylesheet does not travel with it, so whatever is
// not written here is not in the picture.
function painted(svg: SVGSVGElement): string {
  const clone = svg.cloneNode(true) as SVGSVGElement;
  // The clone is walked against the original element for element, so the paint
  // is read from the one in the document and written onto its copy.
  const from = [svg, ...svg.querySelectorAll("*")];
  const onto = [clone, ...clone.querySelectorAll("*")];
  for (let i = 0; i < onto.length; i++) {
    const computed = getComputedStyle(from[i]);
    const props = onto[i].tagName === "text" ? [...PAINT, ...TYPE] : PAINT;
    const style = props
      .map((p) => [p, computed.getPropertyValue(p)] as const)
      .filter(([, v]) => v !== "")
      .map(([p, v]) => `${p}: ${v}`)
      .join("; ");
    if (style !== "") onto[i].setAttribute("style", style);
  }
  // The invisible hit targets take the hover and the focus, and a picture has
  // neither, so they go once the paint is on rather than being painted
  // transparent. They are removed after the walk so the two trees match.
  for (const hit of clone.querySelectorAll(".chart-hit")) hit.remove();
  return new XMLSerializer().serializeToString(clone);
}

// rasterise is the canvas step, and the one thing under a test runner with no
// canvas that cannot run. The ground is the surface the chart was drawn for,
// because a PNG with a transparent ground reads as black in a deck.
async function rasterise(drawn: Drawn): Promise<string> {
  const ground = inLightPalette(() =>
    getComputedStyle(document.documentElement).getPropertyValue("--surface").trim(),
  );
  const image = new Image();
  image.src = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(drawn.markup)}`;
  await new Promise<void>((resolve, reject) => {
    image.onload = () => resolve();
    image.onerror = () => reject(new Error(chartUndrawableLine(drawn.title)));
  });
  const canvas = document.createElement("canvas");
  canvas.width = drawn.width;
  canvas.height = drawn.height;
  const paper = canvas.getContext("2d");
  if (!paper) throw new Error(chartUndrawableLine(drawn.title));
  paper.fillStyle = ground || "white";
  paper.fillRect(0, 0, canvas.width, canvas.height);
  paper.drawImage(image, 0, 0, canvas.width, canvas.height);
  // The base64 the binding carries, without the data URL the canvas answers
  // with: Go decodes the bytes and reads the PNG's own header.
  return canvas.toDataURL("image/png").split(",")[1] ?? "";
}
