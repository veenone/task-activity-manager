import { compactUrl, isAllowedLink } from "@agile-suite/core";

// isAllowedLink now lives in frontend/core/src/lib/links.ts, one definition
// shared with the rich text renderer's link nodes. Re-exported so
// useRitualToolbar's import (and, before this move, this file's own test)
// keeps working unchanged.
export { isAllowedLink };

// The read-only fallback shows a Confluence page body, which any Confluence
// editor can write, so everything here is an allowlist (#536). An element not
// listed is unwrapped: its text stays, its tag and attributes go. The elements
// in DROPPED go with their content, because that content is code, a foreign
// namespace (SVG animation, MathML) or raw text the page never meant to show.
const ALLOWED_TAGS = new Set([
  "a", "abbr", "b", "blockquote", "br", "caption", "cite", "code", "col", "colgroup", "del",
  "div", "em", "h1", "h2", "h3", "h4", "h5", "h6", "hr", "i", "img", "ins", "kbd", "li", "mark",
  "ol", "p", "pre", "q", "s", "small", "span", "strike", "strong", "sub", "sup", "table",
  "tbody", "td", "tfoot", "th", "thead", "time", "tr", "u", "ul",
]);

const DROPPED = new Set([
  "script", "style", "template", "noscript", "iframe", "frame", "frameset", "object", "embed",
  "svg", "math", "title", "textarea", "select", "option", "head", "base", "link", "meta",
  "noembed", "noframes", "xmp", "plaintext",
]);

// No id or class: an id can clobber globals and a class picks up the app's own
// rules, a modal backdrop's fixed position among them.
const GLOBAL_ATTRIBUTES = new Set(["title", "style"]);
const TAG_ATTRIBUTES: Record<string, Set<string>> = {
  a: new Set(["href"]),
  img: new Set(["src", "alt", "width", "height"]),
  ol: new Set(["start", "type"]),
  td: new Set(["colspan", "rowspan"]),
  th: new Set(["colspan", "rowspan", "scope"]),
  col: new Set(["span"]),
  colgroup: new Set(["span"]),
  time: new Set(["datetime"]),
};
const URL_ATTRIBUTES = new Set(["href", "src"]);
const HTML_NAMESPACE = "http://www.w3.org/1999/xhtml";

// The properties a Confluence page sets for colour, emphasis and alignment.
// Position, size, transform and z-index are left out, so a page cannot draw
// over the app.
const STYLE_PROPERTIES = new Set([
  "color", "background-color", "text-align", "text-decoration", "font-weight", "font-style",
  "vertical-align",
]);
// Words, numbers, hex and one rgb()/rgba(): no url(), expression() or escape.
const STYLE_VALUE = /^[a-z0-9#%.,\s-]*(rgba?\([0-9.,%\s]*\))?[a-z0-9#%.,\s-]*$/i;

// safeUrl is isAllowedLink's one relaxation: a relative address, which is
// what a Confluence page's own links legitimately carry, passes too. No
// second copy of the allowed-scheme set here (the ruling: one definition of
// an allowed link) - a value with no scheme at all skips straight to true,
// and everything else is exactly isAllowedLink's own answer.
function safeUrl(value: string): boolean {
  const scheme = /^([a-z][a-z0-9+.-]*):/i.exec(compactUrl(value));
  return !scheme || isAllowedLink(value);
}

function cleanStyle(value: string): string {
  return value
    .split(";")
    .map((declaration) => {
      const colon = declaration.indexOf(":");
      if (colon < 0) return "";
      const property = declaration.slice(0, colon).trim().toLowerCase();
      const propertyValue = declaration.slice(colon + 1).trim();
      const keep = STYLE_PROPERTIES.has(property) && propertyValue !== "" && STYLE_VALUE.test(propertyValue);
      return keep ? `${property}: ${propertyValue}` : "";
    })
    .filter(Boolean)
    .join("; ");
}

function copyAttributes(from: Element, to: Element, tag: string): void {
  const allowed = TAG_ATTRIBUTES[tag];
  for (const attribute of [...from.attributes]) {
    const name = attribute.name.toLowerCase();
    if (!GLOBAL_ATTRIBUTES.has(name) && !allowed?.has(name)) continue;
    if (URL_ATTRIBUTES.has(name) && !safeUrl(attribute.value)) continue;
    const value = name === "style" ? cleanStyle(attribute.value) : attribute.value;
    if (name === "style" && !value) continue;
    to.setAttribute(name, value);
  }
}

// Builds a fresh tree from what passes rather than editing the parsed one in
// place, so nothing the parser made survives unless the allowlist rebuilt it.
function copyChildren(from: Node, to: Node, output: Document): void {
  for (const child of [...from.childNodes]) {
    if (child.nodeType === Node.TEXT_NODE) {
      to.appendChild(output.createTextNode(child.textContent ?? ""));
      continue;
    }
    if (child.nodeType !== Node.ELEMENT_NODE) continue;
    const element = child as Element;
    const tag = element.localName.toLowerCase();
    if (DROPPED.has(tag) || element.namespaceURI !== HTML_NAMESPACE) continue;
    if (!ALLOWED_TAGS.has(tag)) {
      copyChildren(element, to, output);
      continue;
    }
    const clean = output.createElement(tag);
    copyAttributes(element, clean, tag);
    copyChildren(element, clean, output);
    to.appendChild(clean);
  }
}

export function sanitizeHtml(value: string): string {
  if (typeof DOMParser === "undefined") return "";
  const parsed = new DOMParser().parseFromString(value, "text/html");
  const output = document.implementation.createHTMLDocument("");
  const root = output.createElement("div");
  copyChildren(parsed.body, root, output);
  return root.innerHTML;
}
