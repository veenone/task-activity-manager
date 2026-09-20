// jsdom applies no stylesheet, so a render test cannot see a cascade or a
// theme bug. The tests that cover CSS in this workspace read the stylesheet
// text instead; these helpers hand them the declarations a selector carries
// and the token table each theme resolves them against.
import fs from "node:fs";
import path from "node:path";

const here = path.dirname(new URL(import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1"));

export const appCss = (): string => fs.readFileSync(path.join(here, "..", "App.css"), "utf8");
export const coreStyle = (name: string): string =>
  fs.readFileSync(path.join(here, "..", "..", "..", "..", "frontend", "core", "styles", name), "utf8");

/** Comments out, whitespace collapsed: the shape the matchers below expect. */
function flatten(css: string): string {
  return css.replace(/\/\*[\s\S]*?\*\//g, " ").replace(/\s+/g, " ");
}

/** Every rule in `css`, as a selector list and the declarations under it. */
function rules(css: string): { head: string; body: string }[] {
  const out: { head: string; body: string }[] = [];
  // Split on the closing brace rather than matching whole rules with one
  // regex: a regex consumes the brace that delimits the next rule, so every
  // rule that follows a match directly goes unseen. Taking the head from the
  // last opening brace in the chunk keeps rules nested in a media query.
  for (const chunk of flatten(css).split("}")) {
    const at = chunk.lastIndexOf("{");
    if (at < 0) continue;
    const head = chunk.slice(0, at);
    out.push({ head: head.slice(head.lastIndexOf("{") + 1).trim(), body: chunk.slice(at + 1).trim() });
  }
  return out;
}

/** Declarations of the rules that name `selector` as a whole part of their
 *  selector list, so `.ritual-page` does not collect `.ritual-page-head`. */
export function declarationsOf(css: string, selector: string): string {
  return rules(css)
    .filter((r) => r.head.split(",").some((part) => part.trim() === selector))
    .map((r) => r.body)
    .join(" ");
}

/** Declarations of the rules whose selector mentions `selector` anywhere, for
 *  the cases where the interesting rules are a family rather than one name. */
export function declarationsMentioning(css: string, selector: string): string {
  return rules(css)
    .filter((r) => r.head.includes(selector))
    .map((r) => r.body)
    .join(" ");
}

/** The last value declared for `prop`, or undefined when it is never set. */
export function valueOf(declarations: string, prop: string): string | undefined {
  const found = [...declarations.matchAll(new RegExp(`(?:^|;)\\s*${prop}\\s*:([^;]*)`, "g"))];
  return found.length ? found[found.length - 1][1].trim() : undefined;
}

/** The custom properties a token block declares, keyed by name. */
export function tokenTable(css: string, selector: string): Map<string, string> {
  const table = new Map<string, string>();
  for (const [, name, value] of declarationsOf(css, selector).matchAll(/(--[\w-]+)\s*:([^;]*)/g)) {
    table.set(name, value.trim());
  }
  return table;
}

/** Every custom property `css` reads, whether or not anything defines it. */
export function tokensRead(css: string): string[] {
  return [...new Set([...flatten(css).matchAll(/var\(\s*(--[\w-]+)/g)].map((m) => m[1]))].sort();
}

/** Follows a value through the token table until it is no longer a var(). */
export function resolve(value: string, table: Map<string, string>): string {
  let current = value.trim();
  for (let step = 0; step < 10; step += 1) {
    const named = /^var\(\s*(--[\w-]+)\s*(?:,([\s\S]*))?\)$/.exec(current);
    if (!named) return current;
    const defined = table.get(named[1]);
    if (defined !== undefined) current = defined.trim();
    else if (named[2] !== undefined) current = named[2].trim();
    else throw new Error(`${named[1]} is defined nowhere and the reference carries no fallback`);
  }
  throw new Error(`${value} does not settle on a value`);
}

function channels(colour: string): [number, number, number] {
  const hex = /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/.exec(colour.trim());
  if (!hex) throw new Error(`${colour} is not a plain hex colour`);
  const full = hex[1].length === 3 ? [...hex[1]].map((c) => c + c).join("") : hex[1];
  return [0, 2, 4].map((i) => parseInt(full.slice(i, i + 2), 16)) as [number, number, number];
}

function luminance(colour: string): number {
  const [r, g, b] = channels(colour).map((c) => {
    const s = c / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

/** WCAG contrast ratio, 1 for a colour on itself and 21 for black on white. */
export function contrast(a: string, b: string): number {
  const [light, dark] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (light + 0.05) / (dark + 0.05);
}
