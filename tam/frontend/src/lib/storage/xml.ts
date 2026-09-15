// Confluence storage format is XML in all but two habits: it uses the ac:,
// ri: and at: prefixes without declaring them, and it writes HTML named
// entities XML does not define. parseXml undoes both so the browser's own XML
// parser can read a page, and outerXml undoes the declarations XMLSerializer
// adds back when a fragment is written out, so an element TAM does not model
// goes back to Confluence exactly as it came.

export const NAMESPACES: Record<string, string> = {
  ac: "http://atlassian.com/content",
  ri: "http://atlassian.com/resource/identifier",
  at: "http://atlassian.com/template",
};

const XML_ENTITIES = new Set(["amp", "lt", "gt", "quot", "apos"]);
const decoded = new Map<string, string>();

// isBlank is ASCII whitespace only. String.prototype.trim also strips a
// non-breaking space, which in a page is content somebody typed.
export function isBlank(s: string): boolean {
  return /^[ \t\r\n]*$/.test(s);
}

export function toNumericEntities(body: string): string {
  return body.replace(/&([a-zA-Z][a-zA-Z0-9]*);/g, (whole, name: string) => {
    if (XML_ENTITIES.has(name)) return whole;
    let text = decoded.get(name);
    if (text === undefined) {
      text = new DOMParser().parseFromString(`<!doctype html><body>&${name};`, "text/html").body.textContent ?? "";
      decoded.set(name, text);
    }
    // An entity the HTML parser does not know comes back as its own text.
    if (text === "" || text === whole) return whole;
    return Array.from(text).map((ch) => `&#${ch.codePointAt(0)};`).join("");
  });
}

export function parseXml(body: string): Element | null {
  const declarations = Object.entries(NAMESPACES).map(([prefix, uri]) => `xmlns:${prefix}="${uri}"`).join(" ");
  const doc = new DOMParser().parseFromString(`<tam-root ${declarations}>${toNumericEntities(body)}</tam-root>`, "application/xml");
  if (doc.getElementsByTagName("parsererror").length > 0) return null;
  return doc.documentElement;
}

// stripNamespaces removes the declarations only from start and self-closing
// tags. Running the same regex over the whole serialized string would also
// match text or CDATA content that merely looks like a declaration (a code
// macro documenting its own XML, a sentence about attribute syntax), and
// silently rewrite content that is meant to survive byte for byte.
export function stripNamespaces(xml: string): string {
  const CDATA = /<!\[CDATA\[[\s\S]*?\]\]>/g;
  const stripTags = (segment: string) => segment.replace(/<[^>]*>/g, (tag) => tag.replace(/\s+xmlns:(ac|ri|at)="[^"]*"/g, ""));
  let out = "";
  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = CDATA.exec(xml))) {
    out += stripTags(xml.slice(last, m.index)) + m[0];
    last = CDATA.lastIndex;
  }
  return out + stripTags(xml.slice(last));
}

export function outerXml(node: Node): string {
  return stripNamespaces(new XMLSerializer().serializeToString(node));
}

export function escapeText(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

export function escapeAttr(s: string): string {
  return escapeText(s).replace(/"/g, "&quot;");
}
